package update

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"
)

// fakeReleases serves the real signed 1.0.1 checksums and a setup that is
// not the real one, so every successful path must stop at the checksum.
func fakeReleases(t *testing.T, sig []byte, setupHits *atomic.Int32) *httptest.Server {
	sums, _ := os.ReadFile("testdata/SHA256SUMS.txt")
	if sig == nil {
		sig, _ = os.ReadFile("testdata/SHA256SUMS.txt.sig")
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1.0.1/SHA256SUMS.txt", func(w http.ResponseWriter, r *http.Request) { w.Write(sums) })
	mux.HandleFunc("/v1.0.1/SHA256SUMS.txt.sig", func(w http.ResponseWriter, r *http.Request) { w.Write(sig) })
	mux.HandleFunc("/v1.0.1/CaseHUD-Setup-1.0.1.exe", func(w http.ResponseWriter, r *http.Request) {
		setupHits.Add(1)
		w.Write([]byte("MZ not the real setup"))
	})
	mux.HandleFunc("/v1.0.1/elsewhere", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://evil.example/x", http.StatusFound)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	oldBase, oldAllowed := downloadBase, hostAllowed
	t.Cleanup(func() { downloadBase, hostAllowed = oldBase, oldAllowed })
	downloadBase = srv.URL + "/"
	host := strings.TrimPrefix(srv.URL, "http://")
	hostAllowed = func(u *url.URL) bool { return u.Host == host }
	return srv
}

func TestDownloadRejectsWrongSetup(t *testing.T) {
	var hits atomic.Int32
	fakeReleases(t, nil, &hits)
	dir := t.TempDir()
	if _, _, err := Download(context.Background(), "1.0.1", dir, nil); err == nil || !strings.Contains(err.Error(), "signed checksum") {
		t.Fatalf("err = %v", err)
	}
	if hits.Load() != 1 {
		t.Fatalf("setup requested %d times", hits.Load())
	}
	if left, _ := os.ReadDir(dir); len(left) != 0 {
		t.Fatalf("left behind: %v", left)
	}
}

func TestDownloadNeedsSignatureFirst(t *testing.T) {
	var hits atomic.Int32
	fakeReleases(t, []byte("MEUCIQ=="), &hits)
	if _, _, err := Download(context.Background(), "1.0.1", t.TempDir(), nil); err == nil {
		t.Fatal("bad signature accepted")
	}
	if hits.Load() != 0 {
		t.Fatal("setup downloaded before the signature verified")
	}
}

func TestDownloadRefusesForeignHosts(t *testing.T) {
	var hits atomic.Int32
	srv := fakeReleases(t, nil, &hits)
	if _, err := fetchSmall(context.Background(), srv.URL+"/v1.0.1/elsewhere", maxSums); err == nil {
		t.Fatal("redirect to a foreign host followed")
	}
	if _, err := fetchSmall(context.Background(), "http://evil.example/x", maxSums); err == nil {
		t.Fatal("foreign host fetched")
	}
	if _, _, err := Download(context.Background(), "1.0.1/../../x", t.TempDir(), nil); err == nil {
		t.Fatal("bad version accepted")
	}
}

func TestReleaseHosts(t *testing.T) {
	for u, want := range map[string]bool{
		"https://github.com/kewe87/CaseHUD/releases/download/v1.0.2/x": true,
		"https://objects.githubusercontent.com/x":                      true,
		"https://release-assets.githubusercontent.com/x":               true,
		"http://github.com/x":                                          false,
		"https://github.com:8443/x":                                    false,
		"https://user@github.com/x":                                    false,
		"https://github.com.evil.example/x":                            false,
		"https://raw.githubusercontent.com/kewe87/CaseHUD/main/x":      false,
	} {
		p, _ := url.Parse(u)
		if got := hostAllowed(p); got != want {
			t.Errorf("%s: %v", u, got)
		}
	}
}
