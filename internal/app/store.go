// Package app is the long-running core behind the GUI: it owns the panels,
// the per-panel configuration, designs and media files, and streams frames.
package app

import (
	"encoding/json"
	"fmt"
	"image"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/kewe87/casehud/internal/lcd"
	"github.com/kewe87/casehud/internal/media"
	"github.com/kewe87/casehud/internal/panel"
)

// PanelConfig is what one LCD shows.
type PanelConfig struct {
	Mode       string `json:"mode"`   // "design", "media" or "off"
	Design     string `json:"design"` // design id
	Media      string `json:"media"`  // file name in the media folder
	Brightness int    `json:"brightness"`
	Rotation   int    `json:"rotation"`
	Theme      string `json:"theme,omitempty"` // "dark" (default) or "light"
	Name       string `json:"name,omitempty"`  // user label, e.g. "Front oben"
	// Background is an animated layer behind a design: an effect name
	// (see lcd.BackgroundEffects) or "media:<file>"; "" = none.
	Background string `json:"background,omitempty"`
	BgStrength int    `json:"bg_strength,omitempty"` // 1-100 %, 0 = default (35)
	BgBloom    bool   `json:"bg_bloom,omitempty"`    // soft, washed-out glow on the background

	// Grid divides a bar display (1920x480, or 480x1920 mounted upright) into
	// fields (see lcd.GridPresets); "" = one design on the whole screen.
	// With a grid (and Mode "design") each field shows its own Tiles entry,
	// indexed like the preset's cells, and Design is not used.
	Grid  string `json:"grid,omitempty"`
	Tiles []Tile `json:"tiles,omitempty"`
	// Layout is the id of a stored or ready-made layout (see layouts.go). The
	// fields of the layout are copied into Grid and Tiles whenever the
	// configuration is set, so they always match; "" = an own composition.
	Layout string `json:"layout,omitempty"`
}

// Tile is one field of a grid: a virtual small display with its own design,
// theme and animated background.
type Tile struct {
	Mode       string `json:"mode,omitempty"` // "design" (default), "media" or "off"
	Design     string `json:"design,omitempty"`
	Media      string `json:"media,omitempty"`
	Theme      string `json:"theme,omitempty"` // "" = the panel's theme
	Background string `json:"background,omitempty"`
	BgStrength int    `json:"bg_strength,omitempty"`
	BgBloom    bool   `json:"bg_bloom,omitempty"`
}

// config is the tile as a panel configuration (for its source).
func (t Tile) config(panelTheme string) PanelConfig {
	c := PanelConfig{Mode: t.Mode, Design: t.Design, Media: t.Media, Theme: t.Theme,
		Background: t.Background, BgStrength: t.BgStrength, BgBloom: t.BgBloom}
	if c.Mode == "" {
		c.Mode = "design"
	}
	if c.Theme == "" {
		c.Theme = panelTheme
	}
	return c
}

// Clone returns a copy that shares no slice with c, so a stored config cannot
// change behind the back of whoever holds another copy.
func (c PanelConfig) Clone() PanelConfig {
	c.Tiles = slices.Clone(c.Tiles)
	return c
}

// Config is persisted as config.json.
type Config struct {
	LHMURL   string `json:"lhm_url,omitempty"`
	Language string `json:"language,omitempty"` // "de" (default) or "en"
	// NoUpdateCheck disables the daily GitHub release check.
	NoUpdateCheck bool `json:"no_update_check,omitempty"`
	// FWUpdateCheck asks the manufacturer's download page once per start for
	// the newest firmware (off by default; nothing is downloaded).
	FWUpdateCheck bool `json:"fw_update_check,omitempty"`
	// SensorSource is "auto" (default), "service", "lhm" or "custom".
	SensorSource string `json:"sensor_source,omitempty"`
	SensorURL    string `json:"sensor_url,omitempty"` // for "custom"
	NoPresentMon bool   `json:"no_presentmon,omitempty"`
	NoRTSS       bool   `json:"no_rtss,omitempty"`
	// IntervalMS overrides how often designs refresh (0 = per design).
	IntervalMS int `json:"interval_ms,omitempty"`
	// CommunityURL overrides the community gallery index (for testing forks).
	CommunityURL string `json:"community_url,omitempty"`
	// MCP allows AI assistants to use CaseHUD via "casehud-cli mcp" (off by default).
	MCP bool `json:"mcp,omitempty"`
	// API turns the local v1 API for other programs on (off by default); while
	// it is off there is no api.json and the client token is not accepted.
	API bool `json:"api,omitempty"`
	// Fahrenheit shows temperatures in °F, Clock12h clocks with AM/PM.
	Fahrenheit bool `json:"fahrenheit,omitempty"`
	Clock12h   bool `json:"clock_12h,omitempty"`
	// ScreensaverOff blanks the LCDs (brightness 0) while the screen saver runs.
	ScreensaverOff bool `json:"screensaver_off,omitempty"`
	// Rotation changes the displays by themselves (scenes or conveyor).
	Rotation RotationConfig `json:"rotation,omitempty"`
	// Automation switches scenes by itself when a game starts or ends.
	Automation AutomationConfig `json:"automation,omitempty"`
	// Lighting drives addressable RGB hardware (off by default).
	Lighting LightingConfig `json:"lighting,omitzero"`
	// ExperimentalDevices lets CaseHUD talk to devices that are not yet
	// confirmed on real hardware; ExperimentalOff lists driver IDs switched
	// off individually while the global switch is on.
	ExperimentalDevices bool     `json:"experimental_devices,omitempty"`
	ExperimentalOff     []string `json:"experimental_off,omitempty"`
	// Disclaimer is the accepted version of the disclaimer; until it is the
	// current one, CaseHUD sends nothing to any device. AckExperimental and
	// AckLighting record the one-time confirmations of those switches.
	Disclaimer      DisclaimerAck `json:"disclaimer,omitzero"`
	AckExperimental bool          `json:"ack_experimental,omitempty"`
	AckLighting     bool          `json:"ack_lighting,omitempty"`
	// Awtrix lists AWTRIX 3 pixel clocks that show CaseHUD apps.
	Awtrix []AwtrixDevice `json:"awtrix,omitempty"`
	// NoAnimation turns off gliding values (on by default).
	NoAnimation bool `json:"no_animation,omitempty"`
	// GamingMigrated is set once the old "pause while gaming" option has been
	// turned into an automation rule; a rule the user deletes later is not
	// brought back when an old client posts the option again.
	// FanNumbers are the logical numbers of the LCD fans by panel ID (the order
	// of the screens as the user wants it); fans without an entry are numbered
	// in chain order.
	FanNumbers     map[string]int         `json:"fan_numbers,omitempty"`
	GamingMigrated bool                   `json:"gaming_migrated,omitempty"`
	Panels         map[string]PanelConfig `json:"panels"`
	// OfflineClock lists the bar displays (by panel ID) that show their own
	// clock when CaseHUD is not running (a flag the device keeps). NoDeviceVideo
	// switches off playing videos on the displays themselves (see devvideo.go),
	// NoLayers the mode that sends the picture once and the values as an overlay.
	OfflineClock map[string]bool `json:"offline_clock,omitempty"`
	// DeviceFlip lists the bar displays (by panel ID) that turn the picture by
	// 180 degrees themselves (experimental, see panel.DeviceFlipper).
	DeviceFlip    map[string]bool `json:"device_flip,omitempty"`
	NoDeviceVideo bool            `json:"no_device_video,omitempty"`
	NoLayers      bool            `json:"no_layers,omitempty"`
	// DeviceWritten is the brightness and rotation last written to each device
	// that stores them in flash (see panel.SettingsCache): a restart does not
	// write values the device already holds.
	DeviceWritten map[string]panel.Written `json:"device_written,omitempty"`
	// TLStartImageOff switches off writing the CaseHUD logo as the start image
	// of the TL LCD fans; TLStartImages is the hash of the image last written
	// to each fan (see tlpersist.go).
	TLStartImageOff bool              `json:"tl_start_image_off,omitempty"`
	TLStartImages   map[string]string `json:"tl_start_images,omitempty"`
	// TLRestImages is the same record for the stored picture the fans show
	// when CaseHUD is not running.
	TLRestImages map[string]string `json:"tl_rest_images,omitempty"`
	// TLSerialOn assigns a serial number to TL LCD fans that have none (off
	// by default). TLSerialOff is the old opt-out flag; TLSerialMigrated
	// records that it has been turned into TLSerialOn (see migrateTLSerial).
	TLSerialOn       bool `json:"tl_serial_on,omitempty"`
	TLSerialOff      bool `json:"tl_serial_off,omitempty"`
	TLSerialMigrated bool `json:"tl_serial_migrated,omitempty"`
	// ConflictsAskOff switches off the banner about other programs that use
	// the same devices; ConflictsIgnore lists the image names of listed
	// programs the user does not want to be asked about (see conflicts.go).
	ConflictsAskOff bool     `json:"conflicts_ask_off,omitempty"`
	ConflictsIgnore []string `json:"conflicts_ignore,omitempty"`
}

// DefaultPanelConfig picks a sensible design for a new panel.
func DefaultPanelConfig(round bool) PanelConfig {
	d := "water-cooling"
	if round {
		d = "cpu-gpu"
	}
	return PanelConfig{Mode: "design", Design: d, Brightness: 80}
}

// IsWide reports whether a panel is a landscape bar (about 2:1 or wider), as
// opposed to the square and round screens.
func IsWide(w, h int) bool { return h > 0 && w >= h*3/2 }

// IsBar reports whether a panel is a bar in either orientation.
func IsBar(w, h int) bool { return IsWide(w, h) || IsWide(h, w) }

// defaultConfigFor is DefaultPanelConfig for a panel of the given size.
func defaultConfigFor(sz image.Point, round bool) PanelConfig {
	c := DefaultPanelConfig(round)
	if IsBar(sz.X, sz.Y) {
		// Four squares side by side, like the Dashboard on the pump head.
		c.Design = "water-wide"
		if t, ok := GridTemplateByID("overview", "clean"); ok {
			c.Grid, c.Tiles = t.Grid, t.Tiles
		}
	}
	return c
}

// Store manages the data directory:
//
//	config.json   panel assignments
//	designs/      user designs (*.json, lcd.Layout)
//	media/        uploaded images, GIFs and videos
type Store struct {
	Dir string
}

// DefaultDir is %APPDATA%\casehud on Windows, ~/.config/casehud on Linux.
func DefaultDir() string {
	d, err := os.UserConfigDir()
	if err != nil {
		return "casehud"
	}
	return filepath.Join(d, "casehud")
}

func (s *Store) path(parts ...string) string {
	return filepath.Join(append([]string{s.Dir}, parts...)...)
}

// LoadConfig reads config.json (missing file = empty config).
func (s *Store) LoadConfig() (Config, error) {
	c := Config{Panels: map[string]PanelConfig{}}
	b, err := os.ReadFile(s.path("config.json"))
	if os.IsNotExist(err) {
		migrateTLSerial(&c)
		return c, nil
	}
	if err != nil {
		return c, err
	}
	if err := json.Unmarshal(b, &c); err != nil {
		// Unreadable (empty, cut off): keep one copy for the user and start
		// with defaults instead of not starting at all.
		bad := s.path("config.bad.json")
		os.Remove(bad)
		if rerr := os.Rename(s.path("config.json"), bad); rerr != nil {
			return Config{Panels: map[string]PanelConfig{}}, fmt.Errorf("config.json: %w", err)
		}
		log.Printf("config.json unreadable (%v): moved to config.bad.json, starting with defaults", err)
		c = Config{Panels: map[string]PanelConfig{}}
		migrateTLSerial(&c)
		return c, nil
	}
	if c.Panels == nil {
		c.Panels = map[string]PanelConfig{}
	}
	migrateTLSerial(&c)
	return c, nil
}

// migrateTLSerial keeps the serial number assignment on for an installation
// that already accepted the disclaimer (and did not switch it off), while a
// new configuration starts with it off. It runs once per configuration.
func migrateTLSerial(c *Config) {
	if c.TLSerialMigrated {
		return
	}
	c.TLSerialMigrated = true
	if c.Disclaimer.Version > 0 && !c.TLSerialOff {
		c.TLSerialOn = true
	}
}

// SaveConfig writes config.json.
func (s *Store) SaveConfig(c Config) error {
	return writeJSON(s.path("config.json"), c)
}

// saveConfigJSON writes already marshalled config JSON.
func (s *Store) saveConfigJSON(b []byte) error {
	return writeFileAtomic(s.path("config.json"), append(b, '\n'))
}

func writeJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(path, append(b, '\n'))
}

// writeFileAtomic writes via a unique temporary file and a rename, so a
// crash or a concurrent writer never leaves a half-written file.
func writeFileAtomic(path string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	if _, err := f.Write(b); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// DesignInfo describes a design for listings.
type DesignInfo struct {
	ID      string `json:"id"`
	Set     string `json:"set,omitempty"`
	Name    string `json:"name"`
	Round   bool   `json:"round"`
	Builtin bool   `json:"builtin"`
	W       int    `json:"w"` // design size, so the GUI can keep its aspect ratio
	H       int    `json:"h"`
}

var idRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

// ValidID reports whether id is usable as a design/file id.
func ValidID(id string) bool { return idRe.MatchString(id) }

// Designs lists built-in and user designs; user designs override built-ins
// with the same id.
func (s *Store) Designs() []DesignInfo {
	all := map[string]DesignInfo{}
	for id, l := range builtinDesigns() {
		if !slices.Contains(BuiltinOrder, id) {
			continue // hidden alias
		}
		all[id] = DesignInfo{ID: id, Set: l.Set, Name: l.Name, Round: l.Round, Builtin: true, W: l.Width, H: l.Height}
	}
	files, _ := filepath.Glob(s.path("designs", "*.json"))
	for _, f := range files {
		id := strings.TrimSuffix(filepath.Base(f), ".json")
		if l, err := lcd.LoadLayout(f); err == nil && ValidID(id) {
			name := l.Name
			if name == "" {
				name = id
			}
			all[id] = DesignInfo{ID: id, Name: name, Round: l.Round, W: l.Width, H: l.Height}
		}
	}
	out := make([]DesignInfo, 0, len(all))
	for _, d := range all {
		out = append(out, d)
	}
	rank := func(d DesignInfo) int {
		for i, id := range BuiltinOrder {
			if d.Builtin && id == d.ID {
				return i
			}
		}
		return len(BuiltinOrder)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Builtin != out[j].Builtin {
			return out[i].Builtin
		}
		if ri, rj := rank(out[i]), rank(out[j]); ri != rj {
			return ri < rj
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// Design loads a design by id.
func (s *Store) Design(id string) (lcd.Layout, error) {
	if ValidID(id) {
		if l, err := lcd.LoadLayout(s.path("designs", id+".json")); err == nil {
			return l, nil
		}
	}
	if l, ok := builtinDesigns()[id]; ok {
		return l, nil
	}
	return lcd.Layout{}, fmt.Errorf("design %q not found", id)
}

// SaveDesign stores a user design.
func (s *Store) SaveDesign(id string, l lcd.Layout) error {
	if !ValidID(id) {
		return fmt.Errorf("invalid design id %q (use a-z, 0-9 and -)", id)
	}
	return lcd.SaveLayout(s.path("designs", id+".json"), l)
}

// DeleteDesign removes a user design (built-ins come back if overridden).
func (s *Store) DeleteDesign(id string) error {
	if !ValidID(id) {
		return fmt.Errorf("invalid design id")
	}
	return os.Remove(s.path("designs", id+".json"))
}

// MediaInfo describes an uploaded file.
type MediaInfo struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
	Size int64  `json:"size"`
}

// Media lists uploaded media files.
func (s *Store) Media() []MediaInfo {
	entries, _ := os.ReadDir(s.path("media"))
	out := []MediaInfo{}
	for _, e := range entries {
		if k := media.Kind(e.Name()); k != "" && !e.IsDir() {
			fi, _ := e.Info()
			out = append(out, MediaInfo{Name: e.Name(), Kind: k, Size: fi.Size()})
		}
	}
	return out
}

// ReservedMediaName reports whether name (with or without an extension) is a
// Windows device name such as CON or COM1: opening it never reaches a file.
// Uploads rename such files.
func ReservedMediaName(name string) bool {
	stem, _, _ := strings.Cut(name, ".")
	stem = strings.ToUpper(strings.TrimRight(stem, " "))
	switch stem {
	case "CON", "PRN", "AUX", "NUL", "CONIN$", "CONOUT$":
		return true
	}
	if len(stem) == 4 && (strings.HasPrefix(stem, "COM") || strings.HasPrefix(stem, "LPT")) {
		return stem[3] >= '1' && stem[3] <= '9'
	}
	// COM and LPT followed by a superscript digit are device names too
	for _, p := range []string{"COM", "LPT"} {
		if rest, ok := strings.CutPrefix(stem, p); ok && slices.Contains([]string{"¹", "²", "³"}, rest) {
			return true
		}
	}
	return false
}

// MediaPath resolves a media file name, rejecting path tricks, Windows
// device names, alternate data streams and wildcard characters. Other
// characters (umlauts, brackets, spaces) are fine so files a user dropped
// into the folder keep working; uploads are reduced to a plain character set
// by the web layer.
func (s *Store) MediaPath(name string) (string, error) {
	bad := name == "" || len(name) > 200 || name != filepath.Base(name) || strings.HasPrefix(name, ".") ||
		strings.HasSuffix(name, " ") || strings.HasSuffix(name, ".") ||
		strings.ContainsAny(name, `:<>"|?*/\`) ||
		strings.IndexFunc(name, func(r rune) bool { return r < 0x20 || r == 0x7f }) >= 0 ||
		ReservedMediaName(name) || media.Kind(name) == ""
	if bad {
		return "", fmt.Errorf("invalid media name %q", name)
	}
	return s.path("media", name), nil
}
