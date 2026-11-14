package metrics

import "testing"

func TestMockOffersTemperaturesAndSpeeds(t *testing.T) {
	units := map[string]int{}
	for _, r := range NewMock().All() {
		if r.Key == "" {
			t.Fatalf("%+v", r)
		}
		units[r.Unit]++
	}
	if units["°C"] == 0 || units["RPM"] == 0 {
		t.Fatalf("%v", units)
	}
}
