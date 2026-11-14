package metrics

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

const agedLHM = `{"id":0,"Text":"Sensor","Value":"","Children":[
 {"id":1,"Text":"PC","Value":"","Children":[
  {"id":2,"Text":"CPU","Value":"","AgeMs":300,"Children":[
   {"id":3,"Text":"Temperatures","Value":"","Children":[
    {"id":4,"Text":"Core","Value":"61.4 °C","SensorId":"/cpu/0/temperature/0","Children":[]}]}]},
  {"id":5,"Text":"GPU","Value":"","AgeMs":40000,"Children":[
   {"id":6,"Text":"Temperatures","Value":"","Children":[
    {"id":7,"Text":"Core","Value":"54 °C","SensorId":"/gpu/0/temperature/0","Children":[]}]}]},
  {"id":8,"Text":"Board","Value":"","Children":[
   {"id":9,"Text":"Fans","Value":"","Children":[
    {"id":10,"Text":"Pump","Value":"2100 RPM","SensorId":"/lpc/x/0/fan/1","Children":[]}]}]}]}]}`

func TestAgeComesFromTheSnapshotAndTheHardware(t *testing.T) {
	var root lhmNode
	if err := json.Unmarshal([]byte(agedLHM), &root); err != nil {
		t.Fatal(err)
	}
	snap := 2 * time.Second
	got := map[string]Reading{}
	for _, r := range flattenLHMAged(root, &snap) {
		got[r.Key] = r
	}
	for key, want := range map[string]time.Duration{
		"/cpu/0/temperature/0": 2300 * time.Millisecond,
		"/gpu/0/temperature/0": 42 * time.Second,
		"/lpc/x/0/fan/1":       2 * time.Second, // no age of its own: the snapshot's
	} {
		if r := got[key]; !r.AgeKnown || r.Age != want {
			t.Errorf("%s: %+v, want %v", key, r, want)
		}
	}
}

func TestNoAgeWithoutTheHeader(t *testing.T) {
	var root lhmNode
	if err := json.Unmarshal([]byte(agedLHM), &root); err != nil {
		t.Fatal(err)
	}
	for _, r := range flattenLHM(root) {
		if r.AgeKnown || r.Age != 0 {
			t.Errorf("%+v", r)
		}
	}
}

func TestSnapshotAgeHeader(t *testing.T) {
	for v, want := range map[string]struct {
		d  time.Duration
		ok bool
	}{
		"1500": {1500 * time.Millisecond, true}, " 0 ": {0, true},
		"": {}, "-3": {}, "abc": {}, "1.5": {},
	} {
		h := http.Header{}
		if v != "" {
			h.Set(SnapshotAgeHeader, v)
		}
		if d, ok := snapshotAge(h); d != want.d || ok != want.ok {
			t.Errorf("%q: %v %v", v, d, ok)
		}
	}
}

func TestFetchTakesTheAgeFromTheAnswer(t *testing.T) {
	for _, withHeader := range []bool{true, false} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if withHeader {
				w.Header().Set(SnapshotAgeHeader, "7000")
			}
			w.Write([]byte(agedLHM))
		}))
		l := NewLHM(srv.URL)
		if err := l.Refresh(); err != nil {
			t.Fatal(err)
		}
		srv.Close()
		r, ok := l.Get("/cpu/0/temperature/0")
		if !ok || r.AgeKnown != withHeader || (withHeader && r.Age != 7300*time.Millisecond) {
			t.Errorf("header %v: %+v", withHeader, r)
		}
	}
}
