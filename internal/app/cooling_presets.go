package app

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	coolingPresetsFile    = "cooling-presets.json"
	coolingMaxPresets     = 32
	coolingMaxPresetName  = 60 // characters
	coolingMaxPresetsSize = 16 << 20
	coolingPresetSlugLen  = 40
)

var coolingPresetID = regexp.MustCompile(`^[a-z0-9-]{1,64}$`)

// Reasons a preset request is refused; the API answers with them as codes.
const (
	PresetNotFound     = "not_found"
	PresetCoolingOff   = "cooling_off"
	PresetUnconfigured = "unconfigured"
	PresetBlocked      = "blocked"
	PresetBusy         = "busy"
	PresetInvalid      = "invalid_preset"
	PresetBadRequest   = "bad_request"
	PresetStorage      = "storage"
)

// CoolingPresetError is a refused preset request.
type CoolingPresetError struct{ Code, Msg string }

func (e *CoolingPresetError) Error() string { return e.Msg }

func presetErr(code, format string, a ...any) error {
	return &CoolingPresetError{Code: code, Msg: fmt.Sprintf(format, a...)}
}

// ErrCoolingRevision means the configuration changed since the page loaded it.
var ErrCoolingRevision = errors.New("the cooling settings changed in the meantime")

// coolingPreset is a named configuration without mode. Speed measurements are
// not part of it: they describe a fan, not a scheme, and stay with the output.
type coolingPreset struct {
	ID     string        `json:"id"`
	Name   string        `json:"name"`
	Config coolingConfig `json:"config"`
}

type coolingPresetFile struct {
	Presets []coolingPreset `json:"presets"`
}

// CoolingPresetInfo is a preset as listed.
type CoolingPresetInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// CoolingPresetsView is what the page needs to show the presets.
type CoolingPresetsView struct {
	Presets []CoolingPresetInfo `json:"presets"`
	// Active is the ID of the preset the running configuration equals, or "".
	Active string `json:"active"`
	// Error says why the file could not be read; nothing is changed then.
	Error   string `json:"error,omitempty"`
	Max     int    `json:"max"`
	NameMax int    `json:"nameMax"`
}

// coolingRevision changes with every stored configuration, and differs between
// runs of the program.
func (e *Engine) coolingRevision() int64 { return coolingRevBase + e.coolRev.Load() }

// CoolingRevision is the revision of the stored cooling configuration.
func (e *Engine) CoolingRevision() int64 { return e.coolingRevision() }

var coolingRevBase = time.Now().UnixMilli()

// loadCoolingPresets reads the presets file. A missing file is no presets.
func (s *Store) loadCoolingPresets() ([]coolingPreset, error) {
	f, err := os.Open(s.path(coolingPresetsFile))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", coolingPresetsFile, withoutPath(err))
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, coolingMaxPresetsSize+1))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", coolingPresetsFile, withoutPath(err))
	}
	if len(b) > coolingMaxPresetsSize {
		return nil, fmt.Errorf("%s: larger than %d bytes", coolingPresetsFile, coolingMaxPresetsSize)
	}
	var file coolingPresetFile
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&file); err != nil {
		return nil, fmt.Errorf("%s: %w", coolingPresetsFile, err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%s: unexpected data after the presets", coolingPresetsFile)
	}
	if len(file.Presets) > coolingMaxPresets {
		return nil, fmt.Errorf("%s: more than %d presets", coolingPresetsFile, coolingMaxPresets)
	}
	ids := map[string]bool{}
	for i := range file.Presets {
		p := &file.Presets[i]
		if !coolingPresetID.MatchString(p.ID) || ids[p.ID] {
			return nil, fmt.Errorf("%s: a preset ID is invalid or used twice", coolingPresetsFile)
		}
		ids[p.ID] = true
		if _, err := cleanPresetName(p.Name); err != nil {
			return nil, fmt.Errorf("%s: %q: %w", coolingPresetsFile, p.ID, err)
		}
		p.Config.Mode = "" // a preset never switches between test and live
	}
	return file.Presets, nil
}

// presetsCache keeps the presets and their comparison keys while the file stays
// as it is. The page asks every half second, a file can be large, and the key of
// a preset takes a marshalling.
type presetsCache struct {
	mu   sync.Mutex
	have bool
	mod  time.Time
	size int64
	list []coolingPreset
	keys []string
}

func (c *presetsCache) reset() {
	c.mu.Lock()
	c.have, c.list, c.keys = false, nil, nil
	c.mu.Unlock()
}

// readCoolingPresets is loadCoolingPresets for a caller that only reads: it hands
// out the list and the preset keys from memory as long as the file has not
// changed (size and time of change), and neither may be modified. A caller that
// changes the list reads it with loadCoolingPresets.
func (s *Store) readCoolingPresets() ([]coolingPreset, []string, error) {
	c := &s.presets
	st, serr := os.Stat(s.path(coolingPresetsFile))
	if serr != nil || !st.Mode().IsRegular() {
		list, err := s.loadCoolingPresets()
		return list, nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.have && c.size == st.Size() && c.mod.Equal(st.ModTime()) {
		return c.list, c.keys, nil
	}
	list, err := s.loadCoolingPresets()
	if err != nil {
		c.have = false
		return nil, nil, err
	}
	keys := make([]string, len(list))
	for i, p := range list {
		keys[i] = presetKey(p.Config)
	}
	// The file may have changed while it was read: then the next call reads it again.
	if st2, err := os.Stat(s.path(coolingPresetsFile)); err == nil && st2.Size() == st.Size() && st2.ModTime().Equal(st.ModTime()) {
		c.have, c.size, c.mod, c.list, c.keys = true, st.Size(), st.ModTime(), list, keys
	}
	return list, keys, nil
}

// saveCoolingPresets replaces the file as a whole. The caller read it before, so
// an unreadable file is never overwritten.
func (s *Store) saveCoolingPresets(list []coolingPreset) error {
	s.presets.reset()
	defer s.presets.reset()
	b, err := json.MarshalIndent(coolingPresetFile{Presets: list}, "", "  ")
	if err != nil {
		return err
	}
	if len(b) > coolingMaxPresetsSize {
		return errors.New("the presets are too large")
	}
	return withoutPath(writeFileAtomic(s.path(coolingPresetsFile), b))
}

func cleanPresetName(name string) (string, error) {
	name = strings.TrimSpace(name)
	n := utf8.RuneCountInString(name)
	if n == 0 || n > coolingMaxPresetName || !utf8.ValidString(name) {
		return "", fmt.Errorf("the name needs 1 to %d characters", coolingMaxPresetName)
	}
	if strings.IndexFunc(name, unicode.IsControl) >= 0 {
		return "", errors.New("the name has control characters")
	}
	return name, nil
}

// clipName cuts a name to what the API promises.
func clipName(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}

var presetSlugger = strings.NewReplacer("ä", "ae", "ö", "oe", "ü", "ue", "ß", "ss")

// newPresetID derives an ID from the name and makes it unique with a short
// random ending, so that an ID is not handed out twice after a delete.
func newPresetID(name string, taken func(string) bool) string {
	var b strings.Builder
	dash := true
	for _, r := range presetSlugger.Replace(strings.ToLower(name)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			dash = false
		case !dash:
			b.WriteByte('-')
			dash = true
		}
	}
	slug := strings.Trim(b.String(), "-")
	if len(slug) > coolingPresetSlugLen {
		slug = strings.TrimRight(slug[:coolingPresetSlugLen], "-")
	}
	if slug == "" {
		slug = "preset"
	}
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	for {
		var r [4]byte
		_, _ = rand.Read(r[:])
		id := slug + "-" + string([]byte{alphabet[int(r[0])%36], alphabet[int(r[1])%36], alphabet[int(r[2])%36], alphabet[int(r[3])%36]})
		if !taken(id) {
			return id
		}
	}
}

// presetKey is a configuration in the form presets are compared in: without
// mode and speed measurements, and with no difference between none and empty.
func presetKey(c coolingConfig) string {
	c.Mode = ""
	c.Outputs = slices.Clone(c.Outputs)
	if c.Outputs == nil {
		c.Outputs = []coolingOutputConfig{}
	}
	for i := range c.Outputs {
		c.Outputs[i].Speeds, c.Outputs[i].Calibration = nil, nil
	}
	c.Curves = slices.Clone(c.Curves)
	for i := range c.Curves {
		if c.Curves[i].Sources == nil {
			c.Curves[i].Sources = []string{}
		}
		if c.Curves[i].Points == nil {
			c.Curves[i].Points = [][]float64{}
		}
	}
	if len(c.Curves) == 0 {
		c.Curves = nil
	}
	if len(c.SensorNames) == 0 {
		c.SensorNames = nil
	}
	b, _ := json.Marshal(c)
	return string(b)
}

// presetConfigOf is the configuration as a preset keeps it.
func presetConfigOf(c coolingConfig) coolingConfig {
	c.Mode = ""
	c.Outputs = slices.Clone(c.Outputs)
	for i := range c.Outputs {
		c.Outputs[i].Speeds, c.Outputs[i].Calibration = nil, nil
	}
	if c.Outputs == nil {
		c.Outputs = []coolingOutputConfig{}
	}
	return c
}

// usable says whether c is a configuration that runs, or could.
func (c coolingState) usable() bool {
	return c.errMsg == "" && c.configured && len(c.cfg.regulated()) > 0
}

// activeCoolingPreset is the preset the running configuration equals.
func (e *Engine) activeCoolingPreset(c coolingState) string {
	list, keys, err := e.Store.readCoolingPresets()
	if err != nil {
		return ""
	}
	return e.activeOf(c, list, keys)
}

// activeOf is the preset of list that c equals. keys are the keys of the presets
// if known (nil: they are worked out).
func (e *Engine) activeOf(c coolingState, list []coolingPreset, keys []string) string {
	if len(list) == 0 || !e.coolingOn() || !c.usable() {
		return ""
	}
	key := presetKey(c.cfg)
	for i, p := range list {
		pk := ""
		if i < len(keys) {
			pk = keys[i]
		} else {
			pk = presetKey(p.Config)
		}
		if pk == key {
			return p.ID
		}
	}
	return ""
}

func (e *Engine) coolingPresetsOf(c coolingState) CoolingPresetsView {
	v := CoolingPresetsView{Presets: []CoolingPresetInfo{}, Max: coolingMaxPresets, NameMax: coolingMaxPresetName}
	list, keys, err := e.Store.readCoolingPresets()
	if err != nil {
		v.Error = shortCoolingError(err)
		return v
	}
	for _, p := range list {
		v.Presets = append(v.Presets, CoolingPresetInfo{ID: p.ID, Name: p.Name})
	}
	v.Active = e.activeOf(c, list, keys)
	return v
}

// CoolingPresets lists the presets and the active one.
func (e *Engine) CoolingPresets() CoolingPresetsView {
	return e.coolingPresetsOf(e.coolingSnapshot())
}

// presetSource is the running configuration, or why it cannot be kept.
func (e *Engine) presetSource() (coolingConfig, error) {
	if !e.coolingOn() {
		return coolingConfig{}, presetErr(PresetCoolingOff, "cooling is switched off in the settings")
	}
	c := e.coolingSnapshot()
	if !c.usable() {
		return coolingConfig{}, presetErr(PresetUnconfigured, "there is no cooling set up to keep")
	}
	return presetConfigOf(c.cfg), nil
}

func presetStorageErr(err error) error {
	return presetErr(PresetStorage, "%s", shortCoolingError(err))
}

// SaveCoolingPreset keeps the running configuration under a new name.
func (e *Engine) SaveCoolingPreset(name string) (CoolingPresetInfo, error) {
	name, err := cleanPresetName(name)
	if err != nil {
		return CoolingPresetInfo{}, presetErr(PresetBadRequest, "%v", err)
	}
	e.coolOps.Lock()
	defer e.coolOps.Unlock()
	cfg, err := e.presetSource()
	if err != nil {
		return CoolingPresetInfo{}, err
	}
	list, err := e.Store.loadCoolingPresets()
	if err != nil {
		return CoolingPresetInfo{}, presetStorageErr(err)
	}
	if len(list) >= coolingMaxPresets {
		return CoolingPresetInfo{}, presetErr(PresetBadRequest, "at most %d presets", coolingMaxPresets)
	}
	if presetNameTaken(list, name, "") {
		return CoolingPresetInfo{}, presetErr(PresetBadRequest, "there is a preset with this name")
	}
	id := newPresetID(name, func(id string) bool {
		return slices.ContainsFunc(list, func(p coolingPreset) bool { return p.ID == id })
	})
	list = append(list, coolingPreset{ID: id, Name: name, Config: cfg})
	if err := e.Store.saveCoolingPresets(list); err != nil {
		return CoolingPresetInfo{}, presetStorageErr(err)
	}
	return CoolingPresetInfo{ID: id, Name: name}, nil
}

func presetNameTaken(list []coolingPreset, name, except string) bool {
	return slices.ContainsFunc(list, func(p coolingPreset) bool { return p.ID != except && strings.EqualFold(p.Name, name) })
}

// changePreset reads the file, lets f change the list and writes it back.
func (e *Engine) changePreset(id string, f func(list []coolingPreset, i int) ([]coolingPreset, error)) error {
	e.coolOps.Lock()
	defer e.coolOps.Unlock()
	list, err := e.Store.loadCoolingPresets()
	if err != nil {
		return presetStorageErr(err)
	}
	i := slices.IndexFunc(list, func(p coolingPreset) bool { return p.ID == id })
	if i < 0 {
		return presetErr(PresetNotFound, "no preset %q", id)
	}
	list, err = f(list, i)
	if err != nil {
		return err
	}
	if err := e.Store.saveCoolingPresets(list); err != nil {
		return presetStorageErr(err)
	}
	return nil
}

// RenameCoolingPreset changes the name; the ID stays.
func (e *Engine) RenameCoolingPreset(id, name string) error {
	name, err := cleanPresetName(name)
	if err != nil {
		return presetErr(PresetBadRequest, "%v", err)
	}
	return e.changePreset(id, func(list []coolingPreset, i int) ([]coolingPreset, error) {
		if presetNameTaken(list, name, id) {
			return nil, presetErr(PresetBadRequest, "there is a preset with this name")
		}
		list[i].Name = name
		return list, nil
	})
}

// OverwriteCoolingPreset replaces the configuration of a preset with the running one.
func (e *Engine) OverwriteCoolingPreset(id string) error {
	return e.changePreset(id, func(list []coolingPreset, i int) ([]coolingPreset, error) {
		cfg, err := e.presetSource()
		if err != nil {
			return nil, err
		}
		list[i].Config = cfg
		return list, nil
	})
}

// DeleteCoolingPreset removes a preset.
func (e *Engine) DeleteCoolingPreset(id string) error {
	return e.changePreset(id, func(list []coolingPreset, i int) ([]coolingPreset, error) {
		return slices.Delete(list, i, i+1), nil
	})
}

// blockedNow is why the live control leaves the fans alone, or "".
func (c coolingState) blockedNow() (reason, program string) {
	if c.fans == nil {
		return "", ""
	}
	b := c.fans.blocked()
	if b.reason == "" {
		b = c.pumps.blockedBy()
	}
	return b.reason, b.program
}

// ActivateCoolingPreset makes a preset the running configuration. The mode
// stays as it is and the preset goes through the checks of a saved
// configuration. It either applies completely or changes nothing; a preset that
// is active already is left alone.
func (e *Engine) ActivateCoolingPreset(id string) error {
	if !e.coolingOn() {
		return presetErr(PresetCoolingOff, "cooling is switched off in the settings")
	}
	e.coolOps.Lock()
	defer e.coolOps.Unlock()
	list, err := e.Store.loadCoolingPresets()
	if err != nil {
		return presetStorageErr(err)
	}
	i := slices.IndexFunc(list, func(p coolingPreset) bool { return p.ID == id })
	if i < 0 {
		return presetErr(PresetNotFound, "no preset %q", id)
	}
	c := e.coolingSnapshot()
	if !c.usable() {
		return presetErr(PresetUnconfigured, "there is no cooling set up")
	}
	if e.activeOf(c, list, nil) == id {
		return nil
	}
	if reason, _ := c.blockedNow(); reason != "" {
		return presetErr(PresetBlocked, "another program is in charge of the fans")
	}
	if e.coolingMeasuring() {
		return presetErr(PresetBusy, "a speed measurement is running")
	}
	cfg := list[i].Config
	cfg.Mode = c.cfg.Mode
	cfg.Outputs = slices.Clone(cfg.Outputs)
	if cfg.Outputs == nil {
		cfg.Outputs = []coolingOutputConfig{}
	}
	for k := range cfg.Outputs { // measurements belong to the fan
		cfg.Outputs[k].Speeds, cfg.Outputs[k].Calibration = nil, nil
		for _, now := range c.cfg.Outputs {
			if now.ID == cfg.Outputs[k].ID {
				cfg.Outputs[k].Speeds, cfg.Outputs[k].Calibration = now.Speeds, now.Calibration
			}
		}
	}
	if err := cfg.check(); err != nil {
		return presetErr(PresetInvalid, "%s", shortCoolingError(err))
	}
	if err := e.coolingFit(cfg); err != nil {
		return presetErr(PresetInvalid, "%s", shortCoolingError(err))
	}
	if len(cfg.regulated()) > 0 {
		if _, err := newCoolingPreview(cfg, e.prov); err != nil {
			return presetErr(PresetInvalid, "%s", shortCoolingError(err))
		}
	}
	if err := e.storeCooling(cfg); err != nil {
		return presetStorageErr(err)
	}
	return nil
}

// coolingFit says whether the hardware has what cfg needs: its fans, and the
// sensors its curves and pumps read. What cannot be told, because the fan list
// or all sensors are unknown, does not count against it.
func (e *Engine) coolingFit(cfg coolingConfig) error {
	if fans, held := e.coolingFans(); !held && len(fans) > 0 {
		for _, o := range cfg.regulated() {
			if tlOutputID.MatchString(o.ID) && !slices.Contains(fans, o.ID) {
				return fmt.Errorf("%q is not on the fan controller", o.ID)
			}
		}
	}
	if e.prov == nil {
		return nil
	}
	have := map[string]bool{}
	for _, r := range e.prov.All() {
		have[r.Key] = true
	}
	if len(have) == 0 {
		return nil
	}
	seen := map[string]bool{}
	for _, o := range cfg.regulated() {
		for _, k := range cfg.sensors(o.Curve, seen) {
			if !have[k] {
				return fmt.Errorf("the sensor %q is missing", k)
			}
		}
		if o.Pump && o.RPMSensor != "" && !have[o.RPMSensor] {
			return fmt.Errorf("the sensor %q is missing", o.RPMSensor)
		}
		if o.Pump && o.ControlKey != "" && e.channelMissing(o.ControlKey) {
			return fmt.Errorf("the control channel %q is missing", o.ControlKey)
		}
	}
	return nil
}
