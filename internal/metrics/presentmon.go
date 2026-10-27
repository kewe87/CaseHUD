package metrics

import (
	"encoding/csv"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"
)

// PresentMon (Intel, open source) measures frame presentation for every
// running game via ETW. CaseHUD runs its console app in the background with
// CSV on stdout and derives, for the foreground game:
//
//	PresentMon/FPS/Framerate          displayed FPS (incl. generated frames)
//	PresentMon/FPS/Native framerate   rendered FPS without frame generation
//	PresentMon/FPS/Frame generation   factor, e.g. 2 for DLSS FG 2x
//	PresentMon/FPS/Frametime          ms between displayed frames
//	PresentMon/FPS/Latency            PC latency (ms) when available
//
// Column names differ between PresentMon versions, so the CSV header is
// matched against known alternatives.

var (
	pmColPID       = []string{"ProcessID"}
	pmColApp       = []string{"Application"}
	pmColTimeMS    = []string{"CPUStartTime", "CPUStartQPCTime"}
	pmColTimeS     = []string{"TimeInSeconds"}
	pmColPresent   = []string{"MsBetweenPresents", "FrameTime", "MsBetweenAppStart"}
	pmColDisplay   = []string{"MsBetweenDisplayChange", "DisplayedTime"}
	pmColLatency   = []string{"MsPCLatency", "PCLatency", "MsClickToPhotonLatency", "ClickToPhotonLatency", "MsAllInputToPhotonLatency", "DisplayLatency", "MsDisplayLatency", "MsUntilDisplayed"}
	pmColFrameType = []string{"FrameType"}
	pmColMode      = []string{"PresentMode"}
)

// pmIgnore are processes that present frames but are desktop software, never
// games: shell, browsers, terminals, editors, office, chat, launcher UIs,
// capture and video tools. Names are lower case. Users add more in the
// automation settings.
var pmIgnore = map[string]bool{
	"dwm.exe": true, "casehud.exe": true, "explorer.exe": true, "searchhost.exe": true,
	"startmenuexperiencehost.exe": true, "shellexperiencehost.exe": true, "applicationframehost.exe": true,
	"textinputhost.exe": true, "taskmgr.exe": true, "lockapp.exe": true, "logonui.exe": true,
	// terminals
	"windowsterminal.exe": true, "wt.exe": true, "cmd.exe": true, "powershell.exe": true, "pwsh.exe": true,
	"conhost.exe": true, "openconsole.exe": true, "alacritty.exe": true, "wezterm-gui.exe": true, "mintty.exe": true,
	// browsers
	"chrome.exe": true, "msedge.exe": true, "firefox.exe": true, "brave.exe": true, "opera.exe": true,
	"vivaldi.exe": true, "iexplore.exe": true, "arc.exe": true, "zen.exe": true,
	// editors, IDEs, dev tools
	"code.exe": true, "devenv.exe": true, "idea64.exe": true, "pycharm64.exe": true, "goland64.exe": true,
	"webstorm64.exe": true, "rider64.exe": true, "clion64.exe": true, "studio64.exe": true, "notepad.exe": true,
	"notepad++.exe": true, "sublime_text.exe": true, "cursor.exe": true, "githubdesktop.exe": true,
	// Office and documents
	"winword.exe": true, "excel.exe": true, "powerpnt.exe": true, "outlook.exe": true, "onenote.exe": true,
	"olk.exe": true, "teams.exe": true, "ms-teams.exe": true, "acrobat.exe": true, "acrord32.exe": true,
	"soffice.bin": true, "soffice.exe": true, "thunderbird.exe": true, "obsidian.exe": true, "notion.exe": true,
	// chat and communication
	"discord.exe": true, "slack.exe": true, "telegram.exe": true, "whatsapp.exe": true, "signal.exe": true,
	"zoom.exe": true, "skype.exe": true,
	// launchers and stores (the UI, not the games)
	"steam.exe": true, "steamwebhelper.exe": true, "epicgameslauncher.exe": true, "epicwebhelper.exe": true,
	"battle.net.exe": true, "gog galaxy.exe": true, "galaxyclient.exe": true, "origin.exe": true,
	"eadesktop.exe": true, "upc.exe": true, "ubisoftconnect.exe": true, "xboxpcapp.exe": true,
	"gamingservicesui.exe": true, "playnite.desktopapp.exe": true, "playnite.fullscreenapp.exe": true,
	// capture, video, music, creative tools
	"obs64.exe": true, "obs32.exe": true, "streamlabs obs.exe": true, "vlc.exe": true, "mpc-hc64.exe": true,
	"mpc-be64.exe": true, "mpv.exe": true, "potplayermini64.exe": true, "wmplayer.exe": true, "spotify.exe": true,
	"video.ui.exe": true, "microsoft.photos.exe": true, "gimp-2.10.exe": true, "photoshop.exe": true,
	"blender.exe": true, "davinciresolve.exe": true,
	// monitoring and tuning tools
	"msiafterburner.exe": true, "rtss.exe": true, "hwinfo64.exe": true, "hwmonitor.exe": true, "aida64.exe": true,
	"nvidia app.exe": true, "nvcontainer.exe": true, "radeonsoftware.exe": true,
}

// Heuristic limits for a window that is not fullscreen.
const (
	// pmGameGrace is how long a game stays detected after losing the focus.
	pmGameGrace = 15 * time.Second

	pmWindowedMinFPS    = 20.0 // sustained displayed FPS of a windowed game
	pmWindowedMinFrames = 15   // frames within the window
)

// pmFg describes the foreground window.
type pmFg struct {
	PID uint32
	// Fullscreen is true for exclusive fullscreen and for a borderless
	// window that covers its whole monitor.
	Fullscreen bool
}

// pmRect is a window or monitor rectangle in screen coordinates.
type pmRect struct{ Left, Top, Right, Bottom int32 }

const pmWsCaption = 0x00C00000 // WS_BORDER | WS_DLGFRAME

// coversMonitor reports whether a window with the given style counts as
// fullscreen: it covers its whole monitor and is not an ordinary captioned
// window (a maximized window with a title bar also reaches the edges).
func coversMonitor(w, m pmRect, style uint32, zoomed bool) bool {
	if style&pmWsCaption == pmWsCaption {
		return false
	}
	if zoomed && style&0x00040000 != 0 { // maximized with a sizing frame: not a game surface
		return false
	}
	return w.Left <= m.Left && w.Top <= m.Top && w.Right >= m.Right && w.Bottom >= m.Bottom
}

// IsBuiltinIgnored reports whether an executable is on the built-in list of
// desktop programs that are never treated as games.
func IsBuiltinIgnored(exe string) bool { return pmIgnore[strings.ToLower(strings.TrimSpace(exe))] }

type pmFrame struct {
	at        time.Duration // timeline position
	present   float64       // ms, 0 = unknown
	display   float64       // ms, 0 = not displayed / unknown
	latency   float64       // ms, 0 = unknown
	generated bool
	blit      bool // composed copy / GDI path, not a flip path
}

// pmStats keeps a short frame history per process.
type pmStats struct {
	mu        sync.Mutex
	window    time.Duration
	frames    map[uint32][]pmFrame
	names     map[uint32]string
	game      string // process name behind the last readings
	lastPID   uint32 // last process detected as the foreground game
	lastFS    bool
	lastAt    time.Time
	nowFn     func() time.Time // test hook
	hasFType  bool
	ignore    map[string]bool // user list, lower case
	latest    time.Duration
	clockBase time.Time
}

func (s *pmStats) now() time.Time {
	if s.nowFn != nil {
		return s.nowFn()
	}
	return time.Now()
}

func newPMStats() *pmStats {
	return &pmStats{window: time.Second, frames: map[uint32][]pmFrame{}, names: map[uint32]string{}, clockBase: time.Now()}
}

// consume reads PresentMon CSV until r ends.
func (s *pmStats) consume(r io.Reader) error {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	cr.ReuseRecord = true
	header, err := cr.Read()
	if err != nil {
		return err
	}
	idx := map[string]int{}
	for i, h := range header {
		idx[strings.TrimSpace(h)] = i
	}
	col := func(names []string) int {
		for _, n := range names {
			if i, ok := idx[n]; ok {
				return i
			}
		}
		return -1
	}
	cPID, cApp := col(pmColPID), col(pmColApp)
	cTimeMS, cTimeS := col(pmColTimeMS), col(pmColTimeS)
	cPresent, cDisplay, cLat, cType := col(pmColPresent), col(pmColDisplay), col(pmColLatency), col(pmColFrameType)
	cMode := col(pmColMode)
	s.mu.Lock()
	s.hasFType = cType >= 0
	s.mu.Unlock()

	num := func(rec []string, i int) float64 {
		if i < 0 || i >= len(rec) {
			return 0
		}
		v, err := strconv.ParseFloat(strings.TrimSpace(rec[i]), 64)
		if err != nil || v < 0 {
			return 0
		}
		return v
	}
	for {
		rec, err := cr.Read()
		if err != nil {
			return err
		}
		if cPID < 0 || cPID >= len(rec) {
			continue
		}
		pid64, err := strconv.ParseUint(strings.TrimSpace(rec[cPID]), 10, 32)
		if err != nil {
			continue
		}
		f := pmFrame{present: num(rec, cPresent), display: num(rec, cDisplay), latency: num(rec, cLat)}
		switch {
		case cTimeMS >= 0 && num(rec, cTimeMS) > 0:
			f.at = time.Duration(num(rec, cTimeMS) * float64(time.Millisecond))
		case cTimeS >= 0 && num(rec, cTimeS) > 0:
			f.at = time.Duration(num(rec, cTimeS) * float64(time.Second))
		default:
			f.at = time.Since(s.clockBase)
		}
		if cType >= 0 && cType < len(rec) {
			t := strings.ToLower(strings.TrimSpace(rec[cType]))
			f.generated = t != "" && t != "na" && t != "application"
		}
		if cMode >= 0 && cMode < len(rec) {
			m := strings.ToLower(rec[cMode])
			f.blit = strings.Contains(m, "copy") || strings.Contains(m, "gdi") || strings.Contains(m, "blit")
		}
		app := ""
		if cApp >= 0 && cApp < len(rec) {
			app = strings.TrimSpace(rec[cApp])
		}
		s.add(uint32(pid64), app, f)
	}
}

func (s *pmStats) add(pid uint32, app string, f pmFrame) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if pmIgnore[strings.ToLower(app)] || s.ignore[strings.ToLower(app)] {
		return
	}
	s.names[pid] = app
	if f.at > s.latest {
		s.latest = f.at
	}
	fr := append(s.frames[pid], f)
	// Drop frames older than the window.
	cut := 0
	for cut < len(fr) && f.at-fr[cut].at > s.window {
		cut++
	}
	s.frames[pid] = fr[cut:]
}

// setIgnore replaces the user's list of programs that are not games and
// forgets what was collected for them.
func (s *pmStats) setIgnore(names []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ignore = map[string]bool{}
	for _, n := range names {
		if n = strings.ToLower(strings.TrimSpace(n)); n != "" {
			s.ignore[n] = true
		}
	}
	for pid, n := range s.names {
		if s.ignore[strings.ToLower(n)] {
			delete(s.frames, pid)
			delete(s.names, pid)
		}
	}
}

// looksLikeGame decides whether the foreground process's frames come from a
// game rather than a desktop program that merely presents frames: it must
// not be on an ignore list and either cover its monitor (fullscreen or
// borderless) or, as a window, present on a flip path at a sustained rate.
func looksLikeGame(name string, fg pmFg, frames []pmFrame, ignore map[string]bool) bool {
	n := strings.ToLower(name)
	if n == "" || pmIgnore[n] || ignore[n] {
		return false
	}
	if fg.Fullscreen {
		return true
	}
	if len(frames) < pmWindowedMinFrames {
		return false
	}
	var sum float64
	var cnt, blit int
	for _, f := range frames {
		if f.blit {
			blit++
		}
		if d := f.display; d > 0 {
			sum += d
			cnt++
		} else if f.present > 0 {
			sum += f.present
			cnt++
		}
	}
	if blit*2 > len(frames) || cnt == 0 {
		return false
	}
	return 1000/(sum/float64(cnt)) >= pmWindowedMinFPS
}

// readings computes the sensor values for the foreground process, if it is
// a game. Background processes never count: a terminal or browser repainting
// behind the game must not look like one.
//
// A game that was in front stays the game for pmGameGrace after another
// window took the focus (a game on a second monitor while the user clicks
// elsewhere), as long as it keeps presenting.
func (s *pmStats) readings(fg pmFg) []Reading {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	if rs := s.readingsFor(fg); rs != nil {
		s.lastPID, s.lastFS, s.lastAt = fg.PID, fg.Fullscreen, now
		return rs
	}
	if s.lastPID != 0 && s.lastPID != fg.PID && now.Sub(s.lastAt) < pmGameGrace {
		if rs := s.readingsFor(pmFg{PID: s.lastPID, Fullscreen: s.lastFS}); rs != nil {
			return rs
		}
	}
	s.game = ""
	return nil
}

// readingsFor computes the readings for one process (s.mu held).
func (s *pmStats) readingsFor(fg pmFg) []Reading {
	frames := s.frames[fg.PID]
	if len(frames) < 2 || s.latest-frames[len(frames)-1].at > 2*time.Second ||
		!looksLikeGame(s.names[fg.PID], fg, frames, s.ignore) {
		return nil
	}
	s.game = s.names[fg.PID]

	var dispSum, presSum, latSum float64
	var dispN, presN, latN, native int
	for _, f := range frames {
		if f.display > 0 {
			dispSum += f.display
			dispN++
		}
		if f.present > 0 {
			presSum += f.present
			presN++
		}
		if f.latency > 0 {
			latSum += f.latency
			latN++
		}
		if !f.generated {
			native++
		}
	}
	frametime := 0.0
	switch {
	case dispN > 0:
		frametime = dispSum / float64(dispN)
	case presN > 0:
		frametime = presSum / float64(presN)
	}
	if frametime <= 0 {
		return nil
	}
	fps := 1000 / frametime
	out := []Reading{
		{Key: "/presentmon/fps", Path: "PresentMon/FPS/Framerate", Value: fps, Unit: "FPS"},
		{Key: "/presentmon/frametime", Path: "PresentMon/FPS/Frametime", Value: frametime, Unit: "ms"},
	}
	if s.hasFType && native > 0 {
		factor := float64(len(frames)) / float64(native)
		out = append(out,
			Reading{Key: "/presentmon/native-fps", Path: "PresentMon/FPS/Native framerate", Value: fps / factor, Unit: "FPS"},
			Reading{Key: "/presentmon/fg", Path: "PresentMon/FPS/Frame generation", Value: factor, Unit: "x"})
	}
	if latN > 0 {
		out = append(out, Reading{Key: "/presentmon/latency", Path: "PresentMon/FPS/Latency", Value: latSum / float64(latN), Unit: "ms"})
	}
	return out
}

// PresentMon is the provider; the process handling is OS specific.
type PresentMon struct {
	stats    *pmStats
	runner   pmRunner
	readings []Reading
}

// pmRunner starts/monitors the PresentMon process.
type pmRunner interface {
	ensure(stats *pmStats) // start if needed (non-blocking)
	foreground() pmFg
	status() string
}

// NewPresentMon returns a provider; exe is the PresentMon console binary
// ("" = look next to CaseHUD).
func NewPresentMon(exe string) *PresentMon {
	return &PresentMon{stats: newPMStats(), runner: newPMRunner(exe)}
}

// Refresh implements Provider. Missing PresentMon is not an error.
func (p *PresentMon) Refresh() error {
	p.runner.ensure(p.stats)
	p.readings = p.stats.readings(p.runner.foreground())
	return nil
}

// SetIgnore sets the user's list of executables that are not games.
func (p *PresentMon) SetIgnore(names []string) { p.stats.setIgnore(names) }

// GameName is the executable name of the process the FPS readings come from
// ("" when none or unknown).
func (p *PresentMon) GameName() string {
	p.stats.mu.Lock()
	defer p.stats.mu.Unlock()
	return p.stats.game
}

// Status describes the PresentMon state for the GUI: "missing",
// "no-permission", "running" or "unsupported".
func (p *PresentMon) Status() string { return p.runner.status() }

// Get implements Provider.
func (p *PresentMon) Get(source string) (Reading, bool) { return match(p.readings, source) }

// All implements Provider.
func (p *PresentMon) All() []Reading { return p.readings }
