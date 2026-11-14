package metrics

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

const trustTree = `{"Text":"Sensor","Value":"","Children":[{"Text":"PC","Value":"","Children":[
 {"Text":"CPU","Value":"","Children":[{"Text":"Temperatures","Value":"","Children":[
  {"Text":"Core","Value":"50 °C","SensorId":"/cpu/0/temperature/0","Children":[]}]}]}]}]}`

func trustServer(alive *atomic.Bool) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if alive != nil && !alive.Load() {
			http.Error(w, "down", http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte(trustTree))
	}))
}

func TestOnlyTheVerifiedServiceMayDriveTheCooling(t *testing.T) {
	var svcUp atomic.Bool
	svcUp.Store(true)
	svc, fb := trustServer(&svcUp), trustServer(nil)
	defer svc.Close()
	defer fb.Close()
	verified := true
	l := NewLHM("")
	l.URLs = []string{svc.URL, fb.URL}
	l.serviceURL = svc.URL
	l.verifyService = func() bool { return verified }

	display := func() bool {
		if err := l.Refresh(); err != nil || len(l.All()) != 1 {
			t.Fatalf("refresh: %v, %d readings", err, len(l.All()))
		}
		return l.All()[0].DisplayOnly
	}
	if display() {
		t.Fatal("the verified service is marked display only")
	}
	verified = false
	if !display() {
		t.Fatal("a service that cannot be verified must be display only")
	}
	verified = true
	svcUp.Store(false)
	if !display() || l.Source() != fb.URL {
		t.Fatal("the fallback must be display only")
	}
	// The service comes back: it is used again, the fallback does not stick.
	svcUp.Store(true)
	if display() || l.Source() != svc.URL {
		t.Fatalf("the service must be tried first on every refresh, source %s", l.Source())
	}
}

func TestACustomEndpointIsDisplayOnly(t *testing.T) {
	s := trustServer(nil)
	defer s.Close()
	l := NewLHM(s.URL)
	if err := l.Refresh(); err != nil {
		t.Fatal(err)
	}
	for _, r := range l.All() {
		if !r.DisplayOnly {
			t.Fatalf("%+v", r)
		}
	}
}
