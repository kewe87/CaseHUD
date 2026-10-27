package app

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kewe87/casehud/internal/awtrix"
	"github.com/kewe87/casehud/internal/metrics"
)

func TestAwtrixAppsFit(t *testing.T) {
	m := metrics.NewMock()
	for _, info := range AwtrixApps {
		a, ok := awtrixApp(info.ID, m, false, false)
		if !ok {
			t.Fatalf("app %s not built", info.ID)
		}
		n := 0
		for _, f := range a.Text {
			n += len([]rune(f.T))
		}
		// About five characters fit next to the 8×8 icon on a TC001.
		if n == 0 || n > 6 || a.Icon != info.ID || a.Label == "" {
			t.Errorf("app %s text %v has %d characters", info.ID, a.Text, n)
		}
	}
	if _, ok := awtrixApp("nope", m, false, false); ok {
		t.Error("unknown app should not build")
	}
}

func TestAwtrixPushAndRemove(t *testing.T) {
	var mu sync.Mutex
	apps := map[string]map[string]any{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		defer mu.Unlock()
		switch r.URL.Path {
		case "/api/stats":
			w.Write([]byte(`{"version":"0.98"}`))
			return
		case "/api/custom":
		default:
			return // icon uploads
		}
		name := r.URL.Query().Get("name")
		if len(b) == 0 {
			delete(apps, name)
			return
		}
		var a map[string]any
		json.Unmarshal(b, &a)
		apps[name] = a
	}))
	defer srv.Close()

	e := &Engine{Store: &Store{Dir: t.TempDir()}, Demo: true}
	ctx, cancel := context.WithCancel(context.Background())
	if err := e.Start(ctx); err != nil {
		t.Fatal(err)
	}
	host := strings.TrimPrefix(srv.URL, "http://")
	if err := e.SetAwtrixDevices([]AwtrixDevice{{ID: "desk", Name: "Desk", Host: host, Apps: []string{"cpu", "fps", "bogus"}}}); err != nil {
		t.Fatal(err)
	}
	waitFor := func(cond func() bool) {
		t.Helper()
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			mu.Lock()
			ok := cond()
			mu.Unlock()
			if ok {
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
		t.Fatalf("timed out, apps on clock: %v", apps)
	}
	waitFor(func() bool { return len(apps) == 2 && apps["casehud_cpu"]["icon"] == "casehud_cpu" })
	ds, st, infos := e.AwtrixDevices()
	if len(ds[0].Apps) != 2 || st["desk"] != "" || infos["desk"].Firmware != awtrix.Awtrix3 {
		t.Fatalf("devices %+v status %v", ds, st)
	}
	// Deselecting an app removes it from the clock.
	ds[0].Apps = []string{"fps"}
	if err := e.SetAwtrixDevices(ds); err != nil {
		t.Fatal(err)
	}
	waitFor(func() bool { _, cpu := apps["casehud_cpu"]; return !cpu && len(apps) == 1 })
	// Stopping removes everything.
	cancel()
	waitFor(func() bool { return len(apps) == 0 })
	e.Stop()

	if err := e.SetAwtrixDevices([]AwtrixDevice{{ID: "x", Host: "ftp://nope"}}); err == nil {
		t.Error("invalid address accepted")
	}
}

func TestDecimalSep(t *testing.T) {
	if got := decimalSep("27.6°", true); got != "27,6°" {
		t.Errorf("got %q", got)
	}
	if got := decimalSep("27.6°", false); got != "27.6°" {
		t.Errorf("got %q", got)
	}
}
