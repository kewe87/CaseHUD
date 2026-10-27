package update

import "testing"

func TestNewer(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"1.1.0", "1.0.9", true},
		{"v1.0.0", "1.0.0", false},
		{"1.0.0", "1.0.1", false},
		{"2.0", "1.9.9", true},
		{"1.0.1-beta", "1.0.0", true},
	}
	for _, c := range cases {
		if got := Newer(c.a, c.b); got != c.want {
			t.Errorf("Newer(%q,%q)=%v", c.a, c.b, got)
		}
	}
	if !IsDev("0.0.0-dev.12") || !IsDev("dev") || IsDev("1.0.0") {
		t.Error("IsDev")
	}
}
