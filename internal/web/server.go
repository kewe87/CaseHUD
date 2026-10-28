// Package web serves the GUI (embedded HTML/JS) and its JSON API on
// localhost. Every API call must carry the session token, so websites open in
// a browser can't drive the local API.
package web

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kewe87/casehud/internal/app"
	"github.com/kewe87/casehud/internal/autostart"
	"github.com/kewe87/casehud/internal/awtrix"
	"github.com/kewe87/casehud/internal/community"
	"github.com/kewe87/casehud/internal/lcd"
	"github.com/kewe87/casehud/internal/media"
	"github.com/kewe87/casehud/internal/metrics"
	"github.com/kewe87/casehud/internal/sysperm"
	"github.com/kewe87/casehud/internal/update"
)

//go:embed static
var static embed.FS

// maxUpload bounds media uploads.
const maxUpload = 200 << 20

// Server is the GUI backend.
type Server struct {
	Engine  *app.Engine
	Version string
	// OnShow is called when a second app instance asks this one to show
	// its window.
	OnShow func() error
	// OnQuit shuts the whole app down; POST /api/v1/quit calls it.
	OnQuit func()
	// token is the GUI's session token, clientToken the one handed to other
	// programs through api.json. Only the GUI token opens the internal /api/.
	token       string
	clientToken string
	// apiMu guards clientToken, which is empty while the API is off.
	// APIFileDir is where api.json lives ("" = no discovery file).
	apiMu      sync.RWMutex
	clientGen  chan struct{} // closed when the client token is revoked
	syncMu     sync.Mutex    // serialises SyncAPI, so api.json follows the last change
	APIFileDir string

	// Shared status snapshot for the event streams, and their count.
	statusMu     sync.Mutex
	statusBuf    []byte
	statusAt     time.Time
	eventClients atomic.Int32
	liveClients  atomic.Int32 // open /api/live streams

	updMu   sync.Mutex
	update  *update.Release // newer release, if any
	selfUpd selfUpdate
	// NoTray: the app runs without a tray icon (--no-tray); an update asked
	// for through the API then brings it back the same way.
	NoTray bool
	ln     net.Listener

	galleryOnce sync.Once
	gallery     *community.Gallery
}

// Listen binds to 127.0.0.1 (port 0 = random) and returns the GUI URL,
// including the session token.
func (s *Server) Listen(port int) (string, error) {
	var err error
	if s.token, err = randomToken(); err != nil {
		return "", err
	}
	if s.clientToken, err = randomToken(); err != nil {
		return "", err
	}
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return "", err
	}
	s.ln = ln
	return fmt.Sprintf("http://%s/?t=%s", ln.Addr(), s.token), nil
}

func randomToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Serve blocks serving HTTP.
// No global WriteTimeout: SSE streams and large uploads must stay open.
func (s *Server) Serve() error {
	srv := &http.Server{Handler: s.Handler(), ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 2 * time.Minute}
	return srv.Serve(s.ln)
}

// Handler returns the HTTP handler (exported for tests).
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	sub, _ := fs.Sub(static, "static")
	mux.Handle("/", securityHeaders(http.FileServer(http.FS(sub))))

	api := http.NewServeMux()
	api.HandleFunc("GET /api/state", s.state)
	api.HandleFunc("POST /api/rescan", s.rescan)
	api.HandleFunc("PUT /api/panels/{id}", s.putPanel)
	api.HandleFunc("GET /api/panels/{id}/preview", s.panelPreview)
	api.HandleFunc("GET /api/live", s.live)
	s.lightingRoutes(api)
	s.libraryRoutes(api)
	s.devicesRoutes(api)
	s.monitorBackRoutes(api)
	s.tlStartRoutes(api)
	s.conflictRoutes(api)
	api.HandleFunc("POST /api/panels/{id}/identify", func(w http.ResponseWriter, r *http.Request) {
		if err := s.Engine.Identify(r.PathValue("id")); err != nil {
			fail(w, http.StatusBadRequest, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	api.HandleFunc("POST /api/panels/{id}/offline-clock", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			On bool `json:"on"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<10)).Decode(&body); err != nil {
			fail(w, http.StatusBadRequest, err)
			return
		}
		if err := s.Engine.SetOfflineClock(r.PathValue("id"), body.On); err != nil {
			fail(w, http.StatusBadRequest, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	api.HandleFunc("POST /api/panels/{id}/device-flip", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			On bool `json:"on"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<10)).Decode(&body); err != nil {
			fail(w, http.StatusBadRequest, err)
			return
		}
		if err := s.Engine.SetDeviceFlip(r.PathValue("id"), body.On); err != nil {
			fail(w, http.StatusBadRequest, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	api.HandleFunc("POST /api/identify", func(w http.ResponseWriter, r *http.Request) {
		if q := r.URL.Query().Get("port"); q != "" {
			port, err := strconv.Atoi(q)
			if err == nil {
				err = s.Engine.IdentifyPort(port)
			}
			if err != nil {
				fail(w, http.StatusBadRequest, err)
				return
			}
		} else {
			s.Engine.Identify("")
		}
		w.WriteHeader(http.StatusNoContent)
	})
	api.HandleFunc("POST /api/fans/{id}/number", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Number int `json:"number"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<10)).Decode(&body); err != nil {
			fail(w, http.StatusBadRequest, err)
			return
		}
		if err := s.Engine.SetFanNumber(r.PathValue("id"), body.Number); err != nil {
			fail(w, http.StatusBadRequest, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	api.HandleFunc("POST /api/fans/reset", func(w http.ResponseWriter, r *http.Request) {
		if err := s.Engine.ResetFanNumbers(); err != nil {
			fail(w, http.StatusInternalServerError, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	api.HandleFunc("GET /api/designs/{id}", s.getDesign)
	api.HandleFunc("PUT /api/designs/{id}", s.putDesign)
	api.HandleFunc("DELETE /api/designs/{id}", s.deleteDesign)
	api.HandleFunc("GET /api/designs/{id}/preview", s.designPreview)
	api.HandleFunc("PUT /api/layouts/{id}", s.putLayout)
	api.HandleFunc("DELETE /api/layouts/{id}", s.deleteLayout)
	api.HandleFunc("GET /api/layouts/{id}/preview", s.layoutPreview)
	api.HandleFunc("POST /api/layouts/preview", s.layoutPreviewDraft)
	api.HandleFunc("POST /api/render", s.render)
	api.HandleFunc("GET /api/sensors", s.sensors)
	api.HandleFunc("POST /api/sensors/test", s.testSensors)
	api.HandleFunc("POST /api/open-data", s.openData)
	api.HandleFunc("PUT /api/settings/advanced", func(w http.ResponseWriter, r *http.Request) {
		var st app.Settings
		if err := json.NewDecoder(r.Body).Decode(&st); err != nil {
			fail(w, http.StatusBadRequest, err)
			return
		}
		if err := s.Engine.ApplySettings(st); err != nil {
			fail(w, http.StatusBadRequest, err)
			return
		}
		s.SyncAPI() // api.json and the client token follow the "API" switch
		if st.UpdateCheck {
			go s.checkUpdate()
		}
		w.WriteHeader(http.StatusNoContent)
	})
	api.HandleFunc("POST /api/open-log", func(w http.ResponseWriter, r *http.Request) {
		if err := openPath(filepath.Join(s.Engine.Store.Dir, "casehud.log")); err != nil {
			fail(w, http.StatusInternalServerError, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	api.HandleFunc("POST /api/presentmon/setup", func(w http.ResponseWriter, r *http.Request) {
		if err := sysperm.AddToPerformanceLogUsers(); err != nil {
			fail(w, http.StatusBadRequest, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	api.HandleFunc("POST /api/media", s.uploadMedia)
	api.HandleFunc("GET /api/media/{name}", s.getMedia)
	api.HandleFunc("DELETE /api/media/{name}", s.deleteMedia)
	api.HandleFunc("PUT /api/settings", s.putSettings)
	api.HandleFunc("POST /api/update/install", func(w http.ResponseWriter, r *http.Request) {
		if err := s.startSelfUpdate(RelaunchWindow); err != nil {
			fail(w, http.StatusConflict, err)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	})
	api.HandleFunc("GET /api/community", s.communityList)
	api.HandleFunc("GET /api/community/{id}/preview", s.communityPreview)
	api.HandleFunc("POST /api/community/{id}/import", s.communityImport)
	api.HandleFunc("GET /api/rotation", s.getRotation)
	api.HandleFunc("PUT /api/rotation", s.putRotation)
	api.HandleFunc("POST /api/rotation/scene", s.saveScene)
	api.HandleFunc("POST /api/scenes/apply", s.applyScene)
	api.HandleFunc("POST /api/scenes/rename", s.renameScene)
	api.HandleFunc("POST /api/scenes/gaming", func(w http.ResponseWriter, r *http.Request) {
		if err := s.Engine.CreateGamingScene(""); err != nil {
			fail(w, http.StatusBadRequest, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	api.HandleFunc("GET /api/automation", s.getAutomation)
	api.HandleFunc("PUT /api/automation", s.putAutomation)
	api.HandleFunc("POST /api/rotation/next", func(w http.ResponseWriter, r *http.Request) {
		s.Engine.NextRotation()
		w.WriteHeader(http.StatusNoContent)
	})
	api.HandleFunc("GET /api/awtrix", s.getAwtrix)
	api.HandleFunc("PUT /api/awtrix", s.putAwtrix)
	api.HandleFunc("POST /api/awtrix/test", s.testAwtrix)
	api.HandleFunc("POST /api/awtrix/discover", s.discoverAwtrix)
	api.HandleFunc("GET /api/awtrix/preview.png", s.awtrixPreview)
	api.HandleFunc("POST /api/show", func(w http.ResponseWriter, r *http.Request) {
		if s.OnShow != nil {
			// Asynchronous: opening can take seconds and the second instance
			// that pings here only waits one.
			go func() {
				if err := s.OnShow(); err != nil {
					log.Println("show window:", err)
				}
			}()
		}
		w.WriteHeader(http.StatusNoContent)
	})
	// The first-start dialog offers "Quit" before anything runs.
	api.HandleFunc("POST /api/quit", func(w http.ResponseWriter, r *http.Request) {
		if s.OnQuit == nil {
			fail(w, http.StatusServiceUnavailable, errors.New("quit is not available"))
			return
		}
		w.WriteHeader(http.StatusNoContent)
		go func() {
			time.Sleep(200 * time.Millisecond) // let the answer leave first
			s.OnQuit()
		}()
	})
	mux.Handle("/api/", s.auth(limitBody(api)))
	mux.Handle("/api/v1/", s.authV1(limitBody(s.apiV1())))
	return mux
}

// limitBody caps request bodies at 1 MiB (the media upload sets its own limit).
func limitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil && !(r.Method == http.MethodPost && r.URL.Path == "/api/media") {
			r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
		}
		next.ServeHTTP(w, r)
	})
}

// queryTokenOK: the token may travel in the URL only for read requests, which
// browsers issue without custom headers (<img>, EventSource). Everything that
// changes state needs the X-Token header, so a token leaked through a URL
// (history, logs, Referer) cannot be used to write.
func queryTokenOK(r *http.Request) bool {
	return r.Method == http.MethodGet || r.Method == http.MethodHead
}

func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.hostOK(r.Host) { // DNS rebinding: only the loopback names reach the API
			http.Error(w, "forbidden: unexpected Host header", http.StatusForbidden)
			return
		}
		tok := r.Header.Get("X-Token")
		if tok == "" && queryTokenOK(r) {
			tok = r.URL.Query().Get("t") // for <img src> previews and EventSource
		}
		if subtle.ConstantTimeCompare([]byte(tok), []byte(s.token)) != 1 {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, code int, err error) {
	http.Error(w, err.Error(), code)
}

func (s *Server) state(w http.ResponseWriter, r *http.Request) {
	e := s.Engine
	writeJSON(w, map[string]any{
		"version":        s.Version,
		"demo":           e.Demo,
		"panels":         e.Panels(),
		"arrangement":    e.Arrangement(),
		"designs":        e.Store.Designs(),
		"media":          e.Store.Media(),
		"sensorsError":   e.Provider().Err(),
		"scanError":      e.ScanError(),
		"rescanGaveUp":   e.RescanGaveUp(),
		"ffmpeg":         media.FFmpegPath() != "",
		"autostart":      map[string]bool{"supported": autostart.Supported(), "enabled": autostart.Enabled()},
		"dataDir":        e.Store.Dir,
		"language":       e.Language(),
		"fanSets":        app.FanSets,
		"presentmon":     e.Provider().PresentMonStatus(),
		"sensorSource":   sensorSourceName(e.Provider().SensorSource()),
		"updateCheck":    e.UpdateCheck(),
		"settings":       e.Settings(),
		"devices":        e.Devices(),
		"update":         s.availableUpdate(),
		"selfUpdate":     s.updateStatus(),
		"releasesUrl":    update.ReleasesURL,
		"mcpCommand":     mcpCommand(),
		"backgrounds":    lcd.BackgroundEffects,
		"setBackgrounds": lcd.SetBackgrounds,
		"transitions":    app.Transitions,
		"gridTemplates":  gridTemplates(),
		"layouts":        e.Layouts(),
		"lighting":       e.LightingBrief(),
		"monitorMode":    e.MonitorModeDevices(),
		"acks":           e.Acks(),
		"barGrids":       lcd.GridPresets(1920, 480),
	})
}

func (s *Server) rescan(w http.ResponseWriter, r *http.Request) {
	s.Engine.ManualRescan()
	writeJSON(w, s.Engine.Panels())
}

func (s *Server) putPanel(w http.ResponseWriter, r *http.Request) {
	var c app.PanelConfig
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	if err := s.Engine.SetPanelConfig(r.PathValue("id"), c); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) panelPreview(w http.ResponseWriter, r *http.Request) {
	b := s.Engine.Preview(r.PathValue("id"))
	if b == nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Write(b)
}

// live streams the frames the panels show as server-sent events, so the
// GUI preview moves as smoothly as the LCDs. One connection carries all
// panels (browsers allow only a few connections per host).
func (s *Server) live(w http.ResponseWriter, r *http.Request) {
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	if s.liveClients.Add(1) > maxLiveClients {
		s.liveClients.Add(-1)
		http.Error(w, "too many live streams are open", http.StatusServiceUnavailable)
		return
	}
	defer s.liveClients.Add(-1)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("X-Accel-Buffering", "no")
	seen := map[string]uint64{}
	t := time.NewTicker(time.Second / liveFPS)
	defer t.Stop()
	for {
		for _, f := range s.Engine.Frames(seen) {
			seen[f.ID] = f.Seq
			b, _ := json.Marshal(map[string]string{"id": f.ID, "jpg": base64.StdEncoding.EncodeToString(f.JPG)})
			if _, err := fmt.Fprintf(w, "data: %s\n\n", b); err != nil {
				return
			}
		}
		fl.Flush()
		select {
		case <-r.Context().Done():
			return
		case <-t.C:
		}
	}
}

// liveFPS caps the live view; the LCDs animate at about 20 frames/s.
const liveFPS = 25

// maxLiveClients caps the open /api/live streams (each one renders frames).
const maxLiveClients = 8

func (s *Server) getDesign(w http.ResponseWriter, r *http.Request) {
	l, err := s.Engine.Store.Design(r.PathValue("id"))
	if err != nil {
		fail(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, l)
}

func readLayout(r *http.Request) (lcd.Layout, error) {
	l := lcd.DefaultLayout()
	l.Widgets = nil
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&l); err != nil {
		return l, err
	}
	if l.Width <= 0 || l.Height <= 0 || l.Width > 4096 || l.Height > 4096 {
		return l, fmt.Errorf("invalid size %dx%d", l.Width, l.Height)
	}
	if l.IntervalMS < 100 {
		l.IntervalMS = 1000
	}
	if l.HistorySize < 2 {
		l.HistorySize = 90
	}
	lcd.Sanitize(&l)
	return l, nil
}

func (s *Server) putDesign(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	l, err := readLayout(r)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	if err := s.Engine.Store.SaveDesign(id, l); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	s.Engine.DesignChanged(id)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) deleteDesign(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.Engine.Store.DeleteDesign(id); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	s.Engine.DesignChanged(id)
	w.WriteHeader(http.StatusNoContent)
}

func size(r *http.Request, def int) (int, int) {
	wd, _ := strconv.Atoi(r.URL.Query().Get("w"))
	ht, _ := strconv.Atoi(r.URL.Query().Get("h"))
	if wd <= 0 || wd > 2048 {
		wd = def
	}
	if ht <= 0 || ht > 2048 {
		ht = wd
	}
	return wd, ht
}

func (s *Server) designPreview(w http.ResponseWriter, r *http.Request) {
	l, err := s.Engine.Store.Design(r.PathValue("id"))
	if err != nil {
		fail(w, http.StatusNotFound, err)
		return
	}
	s.writePNG(w, l, r)
}

func (s *Server) render(w http.ResponseWriter, r *http.Request) {
	l, err := readLayout(r)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	s.writePNG(w, l, r)
}

func (s *Server) writePNG(w http.ResponseWriter, l lcd.Layout, r *http.Request) {
	l = lcd.ApplyTheme(l, r.URL.Query().Get("theme"))
	wd, ht := size(r, l.Width)
	if r.URL.Query().Get("h") == "" {
		ht = wd * l.Height / l.Width
	}
	ht = max(1, min(ht, 2048)) // extreme aspect ratios must not allocate gigabytes
	bgs, _ := strconv.Atoi(r.URL.Query().Get("bgs"))
	img, err := s.Engine.RenderDesignWithBackground(l, wd, ht, r.URL.Query().Get("bg"), bgs, r.URL.Query().Get("bloom") == "1")
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	png.Encode(w, img)
}

func (s *Server) sensors(w http.ResponseWriter, r *http.Request) {
	rs := append([]metrics.Reading{}, s.Engine.Provider().All()...)
	sort.Slice(rs, func(i, j int) bool { return rs[i].Path < rs[j].Path })
	writeJSON(w, rs)
}

// sensorSourceName turns the active endpoint into a short label.
func sensorSourceName(url string) string {
	switch url {
	case metrics.ServiceURL:
		return "service"
	case "":
		return ""
	default:
		return "lhm"
	}
}

// RunUpdateChecks checks for a newer release now and then daily, while the
// check is enabled. Development builds are never nagged.
func (s *Server) RunUpdateChecks(ctx context.Context) {
	CleanUpdates()
	if update.IsDev(s.Version) {
		return
	}
	t := time.NewTicker(24 * time.Hour)
	defer t.Stop()
	for {
		s.checkUpdate()
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func (s *Server) checkUpdate() {
	if !s.Engine.UpdateCheck() || update.IsDev(s.Version) {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	rel, err := update.Latest(ctx)
	if err != nil {
		return // offline or no releases yet: stay quiet
	}
	s.updMu.Lock()
	defer s.updMu.Unlock()
	if update.Newer(rel.Version, s.Version) {
		s.update = &rel
	} else {
		s.update = nil
	}
}

func (s *Server) availableUpdate() *update.Release {
	if !s.Engine.UpdateCheck() {
		return nil
	}
	s.updMu.Lock()
	defer s.updMu.Unlock()
	return s.update
}

// openData opens the data folder in the file manager.
func (s *Server) openData(w http.ResponseWriter, r *http.Request) {
	if err := openPath(s.Engine.Store.Dir); err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// openPath shows a folder or opens a file with the system's default app.
func openPath(p string) error {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("explorer", p).Start()
	case "darwin":
		return exec.Command("open", p).Start()
	}
	return exec.Command("xdg-open", p).Start()
}

// testSensors refreshes the sensor backend now and reports the result.
func (s *Server) testSensors(w http.ResponseWriter, r *http.Request) {
	err := s.Engine.Provider().Refresh()
	res := map[string]any{"ok": err == nil, "count": len(s.Engine.Provider().All())}
	if err != nil {
		res["error"] = err.Error()
	}
	writeJSON(w, res)
}

func (s *Server) uploadMedia(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUpload)
	f, hdr, err := r.FormFile("file")
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	defer f.Close()
	name := sanitizeName(hdr.Filename)
	path, err := s.Engine.Store.MediaPath(name)
	if err != nil {
		fail(w, http.StatusBadRequest, fmt.Errorf("unsupported file: use PNG, JPG, GIF or a video (MP4, MOV, MKV, WEBM)"))
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	if dirSize(filepath.Dir(path)) > maxMediaFolder {
		fail(w, http.StatusInsufficientStorage, fmt.Errorf("media folder is full: delete files first"))
		return
	}
	out, err := os.Create(path + ".part")
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	if _, err := io.Copy(out, f); err != nil {
		out.Close()
		os.Remove(path + ".part")
		fail(w, http.StatusBadRequest, err)
		return
	}
	out.Close()
	if err := os.Rename(path+".part", path); err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	s.Engine.MediaChanged(name)
	writeJSON(w, map[string]string{"name": name})
}

// maxMediaFolder caps what uploads may add up to (checked before each upload).
const maxMediaFolder = 2 << 30

func dirSize(dir string) int64 {
	var n int64
	ents, _ := os.ReadDir(dir)
	for _, e := range ents {
		if fi, err := e.Info(); err == nil && fi.Mode().IsRegular() {
			n += fi.Size()
		}
	}
	return n
}

// sanitizeName keeps uploaded file names safe and readable.
func sanitizeName(n string) string {
	n = filepath.Base(strings.ReplaceAll(n, "\\", "/"))
	var b strings.Builder
	for _, r := range n {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '-', r == '_':
			b.WriteRune(r)
		case r == ' ':
			b.WriteRune('_')
		}
	}
	out := strings.TrimLeft(b.String(), ".")
	if app.ReservedMediaName(out) {
		out = "_" + out // CON.png and friends are Windows device names
	}
	return out
}

func (s *Server) getMedia(w http.ResponseWriter, r *http.Request) {
	path, err := s.Engine.Store.MediaPath(r.PathValue("name"))
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	http.ServeFile(w, r, path)
}

func (s *Server) deleteMedia(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	path, err := s.Engine.Store.MediaPath(name)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	if err := os.Remove(path); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	s.Engine.MediaChanged(name)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) putSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Autostart   *bool   `json:"autostart"`
		Language    *string `json:"language"`
		UpdateCheck *bool   `json:"updateCheck"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	if req.Autostart != nil {
		if err := autostart.Set(*req.Autostart); err != nil {
			fail(w, http.StatusBadRequest, err)
			return
		}
	}
	if req.UpdateCheck != nil {
		if err := s.Engine.SetUpdateCheck(*req.UpdateCheck); err != nil {
			fail(w, http.StatusInternalServerError, err)
			return
		}
		if *req.UpdateCheck {
			go s.checkUpdate()
		}
	}
	if req.Language != nil {
		if err := s.Engine.SetLanguage(*req.Language); err != nil {
			fail(w, http.StatusBadRequest, err)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) getAwtrix(w http.ResponseWriter, r *http.Request) {
	ds, st, infos := s.Engine.AwtrixDevices()
	if ds == nil {
		ds = []app.AwtrixDevice{}
	}
	writeJSON(w, map[string]any{"devices": ds, "status": st, "info": infos, "apps": app.AwtrixApps})
}

func (s *Server) putAwtrix(w http.ResponseWriter, r *http.Request) {
	var ds []app.AwtrixDevice
	if err := json.NewDecoder(r.Body).Decode(&ds); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	if err := s.Engine.SetAwtrixDevices(ds); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) testAwtrix(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Host string `json:"host"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	st, err := s.Engine.TestAwtrix(r.Context(), req.Host)
	if err != nil {
		fail(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, st)
}

func (s *Server) discoverAwtrix(w http.ResponseWriter, r *http.Request) {
	found := s.Engine.DiscoverAwtrix(r.Context())
	if found == nil {
		found = []awtrix.Found{}
	}
	writeJSON(w, found)
}

func (s *Server) awtrixPreview(w http.ResponseWriter, r *http.Request) {
	b, ok := s.Engine.AwtrixPreview(r.URL.Query().Get("app"), r.URL.Query().Get("model"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store")
	w.Write(b)
}

// mcpCommand is the path of casehud-cli next to the running app, for the
// MCP setup snippet shown in the settings.
func mcpCommand() string {
	exe, err := os.Executable()
	if err != nil {
		return "casehud-cli"
	}
	name := "casehud-cli"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(filepath.Dir(exe), name)
}

// community returns the gallery client; it only goes online when the user
// opens the community gallery.
func (s *Server) community() *community.Gallery {
	s.galleryOnce.Do(func() { s.gallery = community.New(s.Engine.CommunityURL()) })
	return s.gallery
}

func (s *Server) communityList(w http.ResponseWriter, r *http.Request) {
	es, err := s.community().Entries(r.Context())
	if err != nil {
		fail(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, es)
}

func (s *Server) communityPreview(w http.ResponseWriter, r *http.Request) {
	l, _, err := s.community().Design(r.Context(), r.PathValue("id"))
	if err != nil {
		fail(w, http.StatusBadGateway, err)
		return
	}
	s.writePNG(w, l, r)
}

func (s *Server) communityImport(w http.ResponseWriter, r *http.Request) {
	l, e, err := s.community().Design(r.Context(), r.PathValue("id"))
	if err != nil {
		fail(w, http.StatusBadGateway, err)
		return
	}
	id := "community-" + e.ID
	if err := s.Engine.Store.SaveDesign(id, l); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	s.Engine.DesignChanged(id)
	writeJSON(w, map[string]string{"id": id})
}

func (s *Server) getRotation(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"config": s.Engine.Rotation(), "status": s.Engine.RotationStatus()})
}

func (s *Server) putRotation(w http.ResponseWriter, r *http.Request) {
	var rc app.RotationConfig
	if err := json.NewDecoder(r.Body).Decode(&rc); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	if err := s.Engine.SetRotation(rc); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) getAutomation(w http.ResponseWriter, r *http.Request) {
	g := s.Engine.Gaming(s.Engine.Clock())
	writeJSON(w, map[string]any{"config": s.Engine.Automation(), "status": s.Engine.AutomationStatus(),
		"gaming": map[string]any{"active": g.Active, "source": g.Source, "game": g.Game}})
}

func (s *Server) putAutomation(w http.ResponseWriter, r *http.Request) {
	var ac app.AutomationConfig
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&ac); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	if err := s.Engine.SetAutomation(ac); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) applyScene(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	if err := s.Engine.ApplyScene(req.Name); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) renameScene(w http.ResponseWriter, r *http.Request) {
	var req struct {
		From string `json:"from"`
		To   string `json:"to"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	if err := s.Engine.RenameScene(req.From, req.To); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) saveScene(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name       string `json:"name"`
		NoRotation *bool  `json:"no_rotation"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	if err := s.Engine.SaveSceneWith(strings.TrimSpace(req.Name), req.NoRotation); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// securityHeaders locks the GUI page down: only its own scripts, styles and
// images, no framing, no referrer (the URL carries the session token).
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data: blob:; style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self'; frame-ancestors 'none'")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}

// gridTemplates are the ready-made grid compositions per design set.
func gridTemplates() map[string][]app.GridTemplate {
	out := map[string][]app.GridTemplate{}
	for _, set := range app.SetOrder {
		out[set] = app.GridTemplates(set)
	}
	return out
}

func (s *Server) putLayout(w http.ResponseWriter, r *http.Request) {
	var l app.Layout
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&l); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	if err := s.Engine.SaveLayout(r.PathValue("id"), l); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) deleteLayout(w http.ResponseWriter, r *http.Request) {
	if err := s.Engine.DeleteLayout(r.PathValue("id")); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) writeLayoutPNG(w http.ResponseWriter, r *http.Request, grid string, tiles []app.Tile, theme string) {
	wd, _ := strconv.Atoi(r.URL.Query().Get("w"))
	if wd <= 0 {
		wd = 960
	}
	img, err := s.Engine.RenderLayout(grid, tiles, theme, wd)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	png.Encode(w, img)
}

func (s *Server) layoutPreview(w http.ResponseWriter, r *http.Request) {
	l, ok := s.Engine.Store.Layout(r.PathValue("id"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	s.writeLayoutPNG(w, r, l.Grid, l.Tiles, r.URL.Query().Get("theme"))
}

// layoutPreviewDraft draws a composition that is still being edited.
func (s *Server) layoutPreviewDraft(w http.ResponseWriter, r *http.Request) {
	var l app.Layout
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&l); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	s.writeLayoutPNG(w, r, l.Grid, l.Tiles, r.URL.Query().Get("theme"))
}
