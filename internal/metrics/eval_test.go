package metrics

import "testing"

type staticProvider []Reading

func (s staticProvider) Refresh() error                 { return nil }
func (s staticProvider) Get(src string) (Reading, bool) { return match(s, src) }
func (s staticProvider) All() []Reading                 { return s }

func TestEval(t *testing.T) {
	p := staticProvider{
		{Key: "a", Path: "CPU/Powers/Package", Value: 72, Unit: "W"},
		{Key: "b", Path: "GPU/Powers/GPU Package", Value: 51, Unit: "W"},
		{Key: "c", Path: "Board/Voltages/+12V", Value: 12.1, Unit: "V"},
	}
	cases := []struct {
		src  string
		want float64
		ok   bool
	}{
		{"Powers/Package + Powers/GPU Package", 123, true},
		{"Powers/GPU Package - Powers/Package", -21, true},
		{"Powers/Package + Powers/GPU Package - Powers/Package", 51, true},
		{"Voltages/+12V", 12.1, true},
		{"Powers/Package + Powers/Missing", 0, false},
	}
	for _, c := range cases {
		r, ok := Eval(p, c.src)
		if ok != c.ok || (ok && (r.Value != c.want || r.Unit != "W" && r.Unit != "V")) {
			t.Errorf("Eval(%q) = %+v, %v; want %v, %v", c.src, r, ok, c.want, c.ok)
		}
	}
}
