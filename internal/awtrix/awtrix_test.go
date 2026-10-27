package awtrix

import (
	"bytes"
	"context"
	"encoding/json"
	"image/gif"
	"io"
	"mime"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeClock answers like an AWTRIX 3 (ng=false) or AWTRIX NG TC002 clock and
// records what it was sent.
type fakeClock struct {
	mu    sync.Mutex
	ng    bool
	apps  map[string]map[string]any
	files map[string][]byte
}

func (f *fakeClock) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	b, _ := io.ReadAll(r.Body)
	switch {
	case f.ng && r.URL.Path == "/api/v1/device":
		w.Write([]byte(`{"version":"1.2.2","uid":"a1b2c3","boardType":"tc002","hostname":"awtrixng-a1b2c3"}`))
	case f.ng && strings.HasPrefix(r.URL.Path, "/api/v1/apps/pushed/") && r.Method == http.MethodPut:
		var a map[string]any
		if json.Unmarshal(b, &a) != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		f.apps[strings.TrimPrefix(r.URL.Path, "/api/v1/apps/pushed/")] = a
	case f.ng && strings.HasPrefix(r.URL.Path, "/api/v1/apps/") && r.Method == http.MethodDelete:
		delete(f.apps, strings.TrimPrefix(r.URL.Path, "/api/v1/apps/"))
	case !f.ng && r.URL.Path == "/api/stats":
		w.Write([]byte(`{"version":"0.98","app":"Time","uid":"awtrix_123456"}`))
	case !f.ng && r.URL.Path == "/api/custom":
		name := r.URL.Query().Get("name")
		if len(b) == 0 {
			delete(f.apps, name)
			return
		}
		var a map[string]any
		json.Unmarshal(b, &a)
		f.apps[name] = a
	case !f.ng && r.URL.Path == "/edit" && r.Method == http.MethodPost:
		r.Body = io.NopCloser(bytes.NewReader(b))
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		for _, fh := range r.MultipartForm.File["image"] {
			fl, _ := fh.Open()
			data, _ := io.ReadAll(fl)
			// Go strips the folder from Filename; the clock needs it.
			_, params, _ := mime.ParseMediaType(fh.Header.Get("Content-Disposition"))
			f.files[params["filename"]] = data
		}
	default:
		http.NotFound(w, r)
	}
}

func newFake(ng bool) (*fakeClock, *httptest.Server) {
	f := &fakeClock{ng: ng, apps: map[string]map[string]any{}, files: map[string][]byte{}}
	return f, httptest.NewServer(f)
}

var card = Card{Icon: "cpu", Label: "CPU", Text: []Fragment{{"54°", "22D3EE"}}, Progress: 40, ProgressC: "22D3EE", ProgressBC: "202020", Lifetime: 10 * time.Second}

func TestAwtrix3(t *testing.T) {
	f, srv := newFake(false)
	defer srv.Close()
	c := New(strings.TrimPrefix(srv.URL, "http://"))
	ctx := context.Background()
	info, err := c.Probe(ctx)
	if err != nil || info.Firmware != Awtrix3 || info.Model != "tc001" || info.W != 32 || info.H != 8 {
		t.Fatalf("Probe = %+v, %v", info, err)
	}
	if err := c.UploadIcons(ctx); err != nil {
		t.Fatal(err)
	}
	for _, id := range IconIDs {
		g, err := gif.Decode(bytes.NewReader(f.files["ICONS/casehud_"+id+".gif"]))
		if err != nil || g.Bounds().Dx() != 8 || g.Bounds().Dy() != 8 {
			t.Fatalf("icon %s: %v", id, err)
		}
	}
	if err := c.Push(ctx, "casehud_cpu", card, info, true); err != nil {
		t.Fatal(err)
	}
	a := f.apps["casehud_cpu"]
	if a["icon"] != "casehud_cpu" || a["lifetime"] != 10.0 || a["progressC"] != "22D3EE" || len(a["text"].([]any)) != 1 {
		t.Fatalf("pushed %v", a)
	}
	// Without icons the label goes in front of the reading.
	c.Push(ctx, "casehud_cpu", card, info, false)
	if a := f.apps["casehud_cpu"]; a["icon"] != nil || len(a["text"].([]any)) != 2 {
		t.Fatalf("pushed without icons %v", a)
	}
	if err := c.Remove(ctx, "casehud_cpu", Awtrix3); err != nil || len(f.apps) != 0 {
		t.Fatalf("Remove: %v, apps %v", err, f.apps)
	}
	if _, err := New("example.invalid:1").Probe(ctx); err == nil {
		t.Fatal("unreachable clock should fail")
	}
}

func TestAwtrixNG(t *testing.T) {
	f, srv := newFake(true)
	defer srv.Close()
	c := New(strings.TrimPrefix(srv.URL, "http://"))
	ctx := context.Background()
	info, err := c.Probe(ctx)
	if err != nil || info.Firmware != NG || info.Model != "tc002" || info.W != 52 || info.Name != "awtrixng-a1b2c3" {
		t.Fatalf("Probe = %+v, %v", info, err)
	}
	if err := c.Push(ctx, "casehud_cpu", card, info, false); err != nil {
		t.Fatal(err)
	}
	a := f.apps["casehud_cpu"]
	icon, _ := a["icon"].(string)
	if !strings.HasPrefix(icon, "data:image/gif;base64,") || a["font"] != "matrix-chunky8" || a["lifetimeMs"] != 10000.0 || a["progressColor"] != "#22D3EE" {
		t.Fatalf("pushed %v", a)
	}
	frag := a["text"].([]any)[0].(map[string]any)
	if frag["text"] != "54°" || frag["color"] != "#22D3EE" {
		t.Fatalf("fragment %v", frag)
	}
	// Only keys AWTRIX NG documents: it rejects anything else.
	allowed := map[string]bool{"text": true, "textCase": true, "progress": true, "lifetimeMs": true, "icon": true, "font": true, "progressColor": true, "progressTrackColor": true}
	for k := range a {
		if !allowed[k] {
			t.Errorf("unexpected key %q", k)
		}
	}
	if err := c.Remove(ctx, "casehud_cpu", NG); err != nil || len(f.apps) != 0 {
		t.Fatalf("Remove: %v, apps %v", err, f.apps)
	}
}

func TestIconsAndPreview(t *testing.T) {
	for _, id := range IconIDs {
		for _, size := range []int{8, 16} {
			img := Icon(id, size)
			lit := 0
			for y := 0; y < size; y++ {
				for x := 0; x < size; x++ {
					if r, g, b, _ := img.At(x, y).RGBA(); r+g+b > 0 {
						lit++
					}
				}
			}
			if lit < size*size/8 {
				t.Errorf("icon %s/%d nearly empty (%d lit)", id, size, lit)
			}
		}
	}
	if b := Preview(card, 52, 16, 5); len(b) < 100 {
		t.Error("preview empty")
	}
}

func TestHosts(t *testing.T) {
	_, n, _ := net.ParseCIDR("192.168.1.77/24")
	n.IP = net.ParseIP("192.168.1.77").To4()
	h := hosts(n)
	if len(h) != 254 || h[0] != "192.168.1.1" || h[253] != "192.168.1.254" {
		t.Fatalf("hosts /24: %d %v…", len(h), h[:2])
	}
	_, n, _ = net.ParseCIDR("10.0.0.0/16")
	n.IP = net.ParseIP("10.0.9.5").To4()
	h = hosts(n)
	if len(h) != maxSweep || h[0] != "10.0.8.0" {
		t.Fatalf("hosts /16 around 10.0.9.5: %d starting %s", len(h), h[0])
	}
	if got := broadcastAddr(&net.IPNet{IP: net.IPv4(192, 168, 1, 77).To4(), Mask: net.CIDRMask(24, 32)}); got.String() != "192.168.1.255" {
		t.Fatalf("broadcast %s", got)
	}
}
