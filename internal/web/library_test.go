package web

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"testing"
)

func guiReq(t *testing.T, ts interface{ Client() *http.Client }, method, url, token string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(method, url, nil)
	if token != "" {
		req.Header.Set("X-Token", token)
	}
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestLibraryAndDiagnosticsAreGUIOnly(t *testing.T) {
	_, ts := testServer(t)
	for _, c := range [][2]string{
		{"GET", "/api/library"}, {"POST", "/api/library/refresh"}, {"POST", "/api/library/presets/x/apply"},
		{"POST", "/api/diagnostics/export"}, {"POST", "/api/diagnostics/trace"}, {"GET", "/api/diagnostics/trace"},
	} {
		for _, tok := range []string{"", "client"} {
			resp := guiReq(t, ts, c[0], ts.URL+c[1], tok)
			resp.Body.Close()
			if resp.StatusCode != http.StatusForbidden {
				t.Errorf("%s %s with token %q: %d, want 403", c[0], c[1], tok, resp.StatusCode)
			}
		}
	}
}

func TestLibraryOfflineKeepsWorking(t *testing.T) {
	_, ts := testServerWith(t, nil)
	resp := guiReq(t, ts, "GET", ts.URL+"/api/library", "secret")
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("library status %d", resp.StatusCode)
	}
	b, _ := io.ReadAll(resp.Body)
	if !bytes.Contains(b, []byte(`"presets":[]`)) {
		t.Fatalf("empty cache must give an empty list: %s", b)
	}
	r2 := guiReq(t, ts, "POST", ts.URL+"/api/library/presets/nope/apply", "secret")
	r2.Body.Close()
	if r2.StatusCode != http.StatusBadRequest {
		t.Fatalf("unknown preset: %d", r2.StatusCode)
	}
}

func TestDiagnosticsExportDownload(t *testing.T) {
	_, ts := testServer(t)
	resp := guiReq(t, ts, "POST", ts.URL+"/api/diagnostics/export", "secret")
	defer resp.Body.Close()
	if resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "application/zip" {
		t.Fatalf("export: %d %s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	b, _ := io.ReadAll(resp.Body)
	zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil || len(zr.File) < 5 {
		t.Fatalf("zip: %v, %d files", err, len(zr.File))
	}
	tr := guiReq(t, ts, "POST", ts.URL+"/api/diagnostics/trace", "secret")
	tr.Body.Close()
	if tr.StatusCode != http.StatusConflict {
		t.Fatalf("trace with lighting off: %d, want 409", tr.StatusCode)
	}
}
