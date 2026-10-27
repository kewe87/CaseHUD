package app

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"time"

	leds "github.com/kewe87/casehud/internal/rgb"
)

// Diagnostics export: a ZIP the user saves and sends in by hand. It is built
// in memory from what CaseHUD already knows; nothing is sent anywhere. Every
// text goes through a scrubber that removes the user name, the computer name,
// home paths, network addresses, device paths and anything that looks like a
// token.

const (
	logTailBytes = 256 << 10
	traceFor     = 30 * time.Second
)

// ErrTrace marks a trace request that cannot run (bad state, not bad input).
var ErrTrace = errors.New("trace")

// StartLightTrace records the packets exchanged with the lighting devices for
// 30 seconds. It needs the lighting engine to run, because only then there is
// traffic: CaseHUD never starts talking to a device for diagnostics.
func (e *Engine) StartLightTrace() (time.Time, error) {
	l := &e.light
	l.mu.Lock()
	running := l.eng != nil
	l.mu.Unlock()
	if !running {
		return time.Time{}, fmt.Errorf("%w: lighting is not active, there is no device traffic to record", ErrTrace)
	}
	l.traceMu.Lock()
	defer l.traceMu.Unlock()
	now := time.Now()
	if l.traceUntil.After(now) {
		return time.Time{}, fmt.Errorf("%w: a recording is already running", ErrTrace)
	}
	leds.SetTracing(true)
	l.traceUntil = now.Add(traceFor)
	until := l.traceUntil
	l.traceTimer = time.AfterFunc(traceFor, func() {
		l.traceMu.Lock()
		defer l.traceMu.Unlock()
		leds.SetTracing(false)
		l.traceUntil = time.Time{}
	})
	return until, nil
}

// LightTraceStatus reports a running recording and when it ends.
func (e *Engine) LightTraceStatus() (bool, time.Time) {
	l := &e.light
	l.traceMu.Lock()
	defer l.traceMu.Unlock()
	if l.traceUntil.After(time.Now()) {
		return true, l.traceUntil
	}
	return false, time.Time{}
}

// stopTrace ends a recording early (engine shutdown).
func (l *lightingState) stopTrace() {
	l.traceMu.Lock()
	defer l.traceMu.Unlock()
	if l.traceTimer != nil {
		l.traceTimer.Stop()
	}
	leds.SetTracing(false)
	l.traceUntil = time.Time{}
}

// lightingReport returns the report of the running engine. While lighting is
// off it only lists what the system has (HID inventory, other lighting
// programs): no device is addressed.
func (e *Engine) lightingReport(ctx context.Context) leds.Report {
	l := &e.light
	l.mu.Lock()
	eng := l.eng
	l.mu.Unlock()
	if eng != nil {
		return eng.Diagnostics()
	}
	return leds.Inspect(ctx, nil)
}

// DiagnosticsOptions selects optional parts of the export.
type DiagnosticsOptions struct {
	Packets bool // include the recorded device traffic
}

var (
	reIPv4 = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
	// Serial numbers sit inside panel ids and names; they are hashed.
	reTLSerial   = regexp.MustCompile(`\btllcd-s[A-Za-z0-9-]+`)
	reUSSerial   = regexp.MustCompile(`\b(slv3|flex)-([A-Za-z0-9-]{3,})`)
	reNameSerial = regexp.MustCompile(`(\bLCD) \(([A-Za-z0-9-]{3,24})\)`)
	reURLHost    = regexp.MustCompile(`(?i)\b((?:https?|wss?|ftp)://)([^/\s:@"'?#]+)`)
	reVersionCtx = regexp.MustCompile(`(?i)(?:version|ver|fw|firmware|build|v)[\s:=]*$`)
	reMAC        = regexp.MustCompile(`\b(?:[0-9A-Fa-f]{2}[:-]){5}[0-9A-Fa-f]{2}\b`)
	reDevice     = regexp.MustCompile(`\\\\[?.]\\[^\s"']+`)
	reSecret     = regexp.MustCompile(`(?i)(\w*(?:token|secret|password|passwd|api[_-]?key)|authorization)(["']?\s*[:=]\s*["']?)(?:(?:bearer|basic)\s+)?[^\s"'&,}]+`)
	reUNC        = regexp.MustCompile(`\\\\[A-Za-z0-9_.-]+\\[^\s"']*`)
	reSID        = regexp.MustCompile(`S-1-5-21(?:-\d+)+`)
	reIPv6       = regexp.MustCompile(`\b(?:[0-9A-Fa-f]{1,4}:){4,7}[0-9A-Fa-f]{1,4}\b`)
	reLocal      = regexp.MustCompile(`(?i)\b[a-z0-9_-]+\.(?:local|lan|home|fritz\.box)\b`)
	reQueryT     = regexp.MustCompile(`([?&](?:t|token)=)[^&\s"']+`)
	reURLCre     = regexp.MustCompile(`(?i)(https?://)[^/\s:@]+:[^/\s@]+@`)
	reName       = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,47}$`)
)

// scrubFunc returns a function that removes personal data from a text: known
// folders become %VARIABLES%, user and computer name <user> and <computer>,
// network and device addresses and secrets are masked.
func (e *Engine) scrubFunc() func(string) string {
	type rep struct {
		re *regexp.Regexp
		to string
	}
	var reps []rep
	path := func(p, to string) {
		if len(p) < 4 {
			return
		}
		q := regexp.QuoteMeta(filepath.Clean(p))
		q = strings.ReplaceAll(q, `\\`, `[\\/]+`)
		reps = append(reps, rep{regexp.MustCompile(`(?i)` + q), to})
	}
	// Deepest folders first: the data directory sits inside the home folder.
	if e.Store != nil {
		path(e.Store.Dir, "%DATADIR%")
	}
	for _, v := range []string{"LOCALAPPDATA", "APPDATA", "TEMP", "TMP"} {
		path(os.Getenv(v), "%"+v+"%")
	}
	if h, err := os.UserHomeDir(); err == nil {
		path(h, "%USERPROFILE%")
	}
	word := func(w, to string) {
		if len(w) < 3 {
			return
		}
		reps = append(reps, rep{regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(w) + `\b`), to})
	}
	if u, err := user.Current(); err == nil {
		name := u.Username
		if i := strings.LastIndex(name, `\`); i >= 0 {
			word(name[:i], "<computer>")
			name = name[i+1:]
		}
		word(name, "<user>")
		word(u.Name, "<user>")
	}
	word(os.Getenv("USERNAME"), "<user>")
	word(os.Getenv("COMPUTERNAME"), "<computer>")
	if h, err := os.Hostname(); err == nil {
		word(h, "<computer>")
	}
	return func(s string) string {
		s = reTLSerial.ReplaceAllStringFunc(s, func(m string) string { return "tllcd-s#" + hashTag(strings.TrimPrefix(m, "tllcd-s")) })
		s = reUSSerial.ReplaceAllStringFunc(s, func(m string) string {
			sm := reUSSerial.FindStringSubmatch(m)
			if sm[2] == "u" || strings.HasPrefix(sm[2], "u-") {
				return m // no serial number: nothing to hide
			}
			return sm[1] + "-s#" + hashTag(sm[2])
		})
		s = reNameSerial.ReplaceAllStringFunc(s, func(m string) string {
			sm := reNameSerial.FindStringSubmatch(m)
			return sm[1] + " (#" + hashTag(sm[2]) + ")"
		})
		s = reDevice.ReplaceAllString(s, `<device path>`)
		for _, r := range reps {
			s = r.re.ReplaceAllLiteralString(s, r.to)
		}
		s = reUNC.ReplaceAllString(s, `<network path>`)
		s = reSID.ReplaceAllString(s, `<sid>`)
		s = reIPv6.ReplaceAllString(s, `x:x:x:x`)
		s = reLocal.ReplaceAllString(s, `<host>`)
		s = reURLCre.ReplaceAllString(s, `${1}<credentials>@`)
		s = reURLHost.ReplaceAllStringFunc(s, func(m string) string {
			sm := reURLHost.FindStringSubmatch(m)
			if isLoopbackHost(sm[2]) || strings.HasPrefix(sm[2], "<") {
				return m
			}
			return sm[1] + "<host>"
		})
		s = reSecret.ReplaceAllString(s, `${1}${2}<redacted>`)
		s = reQueryT.ReplaceAllString(s, `${1}<redacted>`)
		s = reMAC.ReplaceAllString(s, `xx:xx:xx:xx:xx:xx`)
		return maskIPv4(s)
	}
}

// hashTag is the first 8 hex digits of the SHA-256 of s: enough to tell two
// devices apart in a report, not enough to read the serial number back.
func hashTag(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:4])
}

func isLoopbackHost(h string) bool {
	h = strings.ToLower(h)
	return h == "localhost" || h == "127.0.0.1" || h == "0.0.0.0" || h == "[::1]"
}

// maskIPv4 masks every dotted quad whose parts are valid octets, public ones
// too. Loopback and 0.0.0.0 stay, and so do version numbers ("version
// 1.1.0.5").
func maskIPv4(s string) string {
	var b strings.Builder
	last := 0
	for _, loc := range reIPv4.FindAllStringIndex(s, -1) {
		m := s[loc[0]:loc[1]]
		ok := true
		for _, part := range strings.Split(m, ".") {
			if n, _ := strconv.Atoi(part); n > 255 {
				ok = false
			}
		}
		if !ok || strings.HasPrefix(m, "127.") || m == "0.0.0.0" {
			continue
		}
		pre := s[max(0, loc[0]-14):loc[0]]
		if reVersionCtx.MatchString(pre) {
			continue
		}
		b.WriteString(s[last:loc[0]])
		b.WriteString("x.x.x.x")
		last = loc[1]
	}
	b.WriteString(s[last:])
	return b.String()
}

// scrubJSON marshals v, cleans every string value (the hex of packets stays
// as it is) and returns indented JSON.
func scrubJSON(v any, scrub func(string) string) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var generic any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return nil, err
	}
	generic = mapKeys(generic, scrub)
	out, err := json.MarshalIndent(mapStrings(generic, "", func(key, s string) string {
		if key == "hex" {
			return s
		}
		return scrub(s)
	}), "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}

// mapKeys applies f to every object key of a decoded JSON value (keys can
// hold panel ids, device paths or names). Keys that become equal get a
// numeric suffix instead of overwriting each other.
func mapKeys(x any, f func(string) string) any {
	switch t := x.(type) {
	case []any:
		for i := range t {
			t[i] = mapKeys(t[i], f)
		}
	case map[string]any:
		out := make(map[string]any, len(t))
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			nk := f(k)
			for n := 2; ; n++ {
				if _, taken := out[nk]; !taken {
					break
				}
				nk = fmt.Sprintf("%s~%d", f(k), n)
			}
			out[nk] = mapKeys(t[k], f)
		}
		return out
	}
	return x
}

// mapStrings applies f to every string of a decoded JSON value, with the
// name of the key it sits under.
func mapStrings(x any, key string, f func(key, s string) string) any {
	switch t := x.(type) {
	case string:
		return f(key, t)
	case []any:
		for i := range t {
			t[i] = mapStrings(t[i], key, f)
		}
	case map[string]any:
		for k, v := range t {
			t[k] = mapStrings(v, k, f)
		}
	}
	return x
}

// sanitizedConfig is config.json without addresses and file names.
func (e *Engine) sanitizedConfig() any {
	e.mu.Lock()
	raw, _ := json.Marshal(e.cfg)
	e.mu.Unlock()
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	if m == nil {
		m = map[string]any{}
	}
	for _, k := range []string{"lhm_url", "sensor_url", "community_url"} {
		if _, ok := m[k]; ok {
			m[k] = "<set>"
		}
	}
	if as, ok := m["awtrix"].([]any); ok {
		m["awtrix"] = map[string]any{"clocks": len(as)}
	}
	if lc, ok := m["lighting"].(map[string]any); ok {
		delete(lc, "fans") // controller read-out
		delete(lc, "fansAt")
		if lb, ok := lc["labels"]; ok { // names the user gave to zones
			delete(lc, "labels")
			if m, ok := lb.(map[string]any); ok {
				lc["zoneLabels"] = len(m)
			}
		}
	}
	// Designs, themes and effects are ids; media files and free-text labels
	// can carry names, so only id-shaped values stay.
	return mapStrings(m, "", func(key, s string) string {
		switch key {
		case "media", "label", "background", "name", "ignore_games", "game", "scene", "gaming_scene", "title", "text":
			if s != "" && !reName.MatchString(s) {
				return "<name>"
			}
		case "when": // "game" or "game:<exe>": the executable is a free text
			if s != "game" && s != "" {
				return "game:<name>"
			}
		case "host", "addr", "address":
			if s != "" {
				return "<host>"
			}
		}
		return s
	})
}

type panelInfo struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	Round        bool   `json:"round"`
	Experimental bool   `json:"experimental,omitempty"`
	Error        string `json:"error,omitempty"`
	ErrorCode    string `json:"errorCode,omitempty"`
}

// WriteDiagnostics writes the diagnostics ZIP to w.
func (e *Engine) WriteDiagnostics(ctx context.Context, w io.Writer, version string, opt DiagnosticsOptions) error {
	scrub := e.scrubFunc()
	zw := zip.NewWriter(w)
	add := func(name string, b []byte) error {
		f, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate, Modified: time.Now()})
		if err != nil {
			return err
		}
		_, err = f.Write(b)
		return err
	}
	addJSON := func(name string, v any) error {
		b, err := scrubJSON(v, scrub)
		if err != nil {
			return err
		}
		return add(name, b)
	}

	if err := add("README.txt", []byte(diagReadme(opt.Packets))); err != nil {
		return err
	}
	info := map[string]any{
		"generated": time.Now().UTC().Format(time.RFC3339),
		"version":   version, "go": runtime.Version(), "os": runtime.GOOS, "arch": runtime.GOARCH,
		"demo": e.Demo,
	}
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, s := range bi.Settings {
			if s.Key == "vcs.revision" || s.Key == "vcs.time" || s.Key == "vcs.modified" {
				info[s.Key] = s.Value
			}
		}
	}
	if err := addJSON("version.json", info); err != nil {
		return err
	}
	if err := addJSON("config.json", e.sanitizedConfig()); err != nil {
		return err
	}
	ps := []panelInfo{}
	for _, p := range e.Panels() {
		ps = append(ps, panelInfo{ID: p.ID, Name: p.Name, Width: p.Width, Height: p.Height, Round: p.Round,
			Experimental: p.Experimental, Error: p.Error, ErrorCode: p.ErrorCode})
	}
	if err := addJSON("panels.json", map[string]any{"panels": ps}); err != nil {
		return err
	}
	rep := e.lightingReport(ctx)
	if !opt.Packets {
		rep.Packets = nil
	}
	if err := addJSON("lighting.json", rep); err != nil {
		return err
	}
	if err := add("casehud.log", []byte(scrub(e.logTail()))); err != nil {
		return err
	}
	return zw.Close()
}

// logTail returns the end of casehud.log (whole lines).
func (e *Engine) logTail() string {
	f, err := os.Open(e.Store.path("casehud.log"))
	if err != nil {
		return "(no log file)\n"
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return "(log not readable)\n"
	}
	off := int64(0)
	if st.Size() > logTailBytes {
		off = st.Size() - logTailBytes
	}
	buf := make([]byte, st.Size()-off)
	n, _ := f.ReadAt(buf, off)
	s := string(buf[:n])
	if off > 0 {
		if i := strings.IndexByte(s, '\n'); i >= 0 {
			s = s[i+1:]
		}
		s = "(older lines left out)\n" + s
	}
	return s
}

func diagReadme(packets bool) string {
	var b strings.Builder
	b.WriteString("CaseHUD diagnostics\r\n\r\n")
	b.WriteString("This file was created on your PC and has not been sent anywhere. Look through it before you share it.\r\n\r\n")
	b.WriteString("README.txt     this file\r\n")
	b.WriteString("version.json   program version, build revision, Go version, operating system and architecture. No personal data.\r\n")
	b.WriteString("config.json    your settings. Addresses and host names are replaced, names of scenes, rules, games, media files and\r\n")
	b.WriteString("               labels are replaced with <name> unless they are plain ids; panel ids are included with serial numbers hashed.\r\n")
	b.WriteString("panels.json    connected displays: id, name, size and the last error. Serial numbers inside ids and names are hashed.\r\n")
	b.WriteString("lighting.json  lighting devices, their log and the HID inventory (no serial numbers, device paths as hashes).\r\n")
	b.WriteString("casehud.log    the end of the log of this session.\r\n\r\n")
	b.WriteString("Serial numbers are never included: a serial inside a panel id or name is replaced by a short hash (for example tllcd-s#1a2b3c4d),\r\n")
	b.WriteString("which tells two devices apart but does not show the serial number itself.\r\n")
	b.WriteString("Removed from all texts and from the names of JSON entries: user name, computer name, home folder, all IPv4 addresses\r\n")
	b.WriteString("(except 127.x.x.x), IPv6 and MAC addresses, host names in URLs, network paths, Windows SIDs, device paths, tokens and passwords.\r\n")
	if packets {
		b.WriteString("\r\nlighting.json also holds a 30 second recording of the raw data exchanged with the lighting devices.\r\n")
		b.WriteString("It can contain hardware identifiers of the controller. Check it before you share it.\r\n")
	}
	return b.String()
}
