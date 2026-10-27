package fwcheck

import "testing"

const page = `<a href="https://example.invalid/x/QZ000_M48401SQ_TL12028-Controller_V10062.bin">a</a>
<a href="https://example.invalid/old/QZ000_M48401SQ_TL12028-Controller_V10058.bin">b</a>
<a href="https://example.invalid/FW_0618/lianli88_0001_0023">c</a>
<a href="https://example.invalid/FW_0618/lianli88_0001_0021">d</a>
<a href="https://example.invalid/FW_0618/JZ5081_20231227_1.bin">e</a>
<a href="https://example.invalid/FW_0618/JZ5081_20230105_1.bin">f</a>
<a href="https://example.invalid/other_V99999.bin">g</a>`

func TestParse(t *testing.T) {
	got := Parse([]byte(page))
	want := []Release{
		{Product: "0416:7372", Version: "0.62", File: "QZ000_M48401SQ_TL12028-Controller_V10062.bin"},
		{Product: "1cbe:a088", Version: "1.5", File: "lianli88_0001_0023"},
		{Product: "04fc:7393", Date: "2023-12-27", File: "JZ5081_20231227_1.bin"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for i, w := range want {
		g := got[i]
		if g.Product != w.Product || g.Version != w.Version || g.Date != w.Date || g.File != w.File {
			t.Errorf("%d: got %+v, want %+v", i, g, w)
		}
	}
}

func TestNewer(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want bool
	}{
		{"0.62", "0.58", true}, {"0.62", "0.62", false}, {"0.58", "0.62", false},
		{"1.10", "1.9", true}, {"1.5", "", true}, {"", "1.5", false}, {"x", "1.0", false},
	} {
		if got := Newer(c.a, c.b); got != c.want {
			t.Errorf("Newer(%q, %q) = %v", c.a, c.b, got)
		}
	}
}
