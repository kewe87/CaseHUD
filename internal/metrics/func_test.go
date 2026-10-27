package metrics

import "testing"

func TestFuncProviderInMulti(t *testing.T) {
	f := &Func{Fn: func() []Reading {
		return []Reading{{Key: "edgehub/ambient", Path: "EdgeHub/Ambient", Value: 27.5, Unit: "°C"}}
	}}
	m := New(Options{SensorURLs: []string{"http://127.0.0.1:1/none"}, Extra: []Provider{f}})
	m.Refresh() // the primary source is unreachable; the extra one still delivers
	r, ok := m.Get("EdgeHub/Ambient")
	if !ok || r.Value != 27.5 || r.Unit != "°C" {
		t.Fatalf("%+v %v", r, ok)
	}
	if _, ok := m.Get("Ambient"); !ok {
		t.Fatal("suffix match failed")
	}
	if (&Func{}).Refresh() != nil {
		t.Fatal("empty Func failed")
	}
}
