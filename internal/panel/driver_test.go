package panel

import (
	"strings"
	"testing"
)

func TestBuiltInDriversRegistered(t *testing.T) {
	want := map[string]bool{ // id -> experimental
		"tllcd": false, "us88": false,
		"aio": true, "us92": true, "us207": true, "slv3": true, "flex": true, "edgehub": true,
	}
	got := map[string]Driver{}
	for _, d := range Drivers() {
		got[d.ID] = d
	}
	for id, exp := range want {
		d, ok := got[id]
		if !ok {
			t.Errorf("driver %s missing", id)
			continue
		}
		if d.Experimental != exp {
			t.Errorf("driver %s: experimental = %v, want %v", id, d.Experimental, exp)
		}
		if len(d.Models) == 0 || d.Name == "" || d.Enumerate == nil || d.Transport == "" {
			t.Errorf("driver %s: incomplete metadata", id)
		}
		if strings.Contains(strings.ToLower(d.Vendor+d.Name), "lian") {
			t.Errorf("driver %s: brand name in metadata", id)
		}
	}
	// the known USB IDs are claimed by exactly one driver each
	owner := map[[2]uint16]string{}
	for _, d := range Drivers() {
		for _, m := range d.Models {
			k := [2]uint16{m.VendorID, m.ProductID}
			if o, dup := owner[k]; dup {
				t.Errorf("%04x:%04x claimed by %s and %s", k[0], k[1], o, d.ID)
			}
			owner[k] = d.ID
		}
	}
	for k, id := range map[[2]uint16]string{
		{0x04FC, 0x7393}: "tllcd", {0x1CBE, 0xA088}: "us88", {0x1CBE, 0xA092}: "us92", {0x1CBE, 0xA065}: "us207",
		{0x1CBE, 0x0005}: "slv3", {0x1CBE, 0x0006}: "slv3", {0x1CBE, 0xA018}: "flex", {0x1CBE, 0xA019}: "flex",
		{0x0416, 0x7391}: "aio", {0x0416, 0x7395}: "aio", {0x0416, 0x7398}: "aio", {0x0416, 0x7399}: "aio", {0x0416, 0x739A}: "aio",
		{0x39B0, 0x0101}: "edgehub",
	} {
		if owner[k] != id {
			t.Errorf("%04x:%04x owned by %q, want %q", k[0], k[1], owner[k], id)
		}
	}
	// existing panel IDs and the discovery order are unchanged: TL LCD, pump heads, bar
	order := ""
	for _, d := range Drivers() {
		if d.ID == "tllcd" || d.ID == "aio" || d.ID == "us88" {
			order += d.ID + " "
		}
	}
	if order != "tllcd aio us88 " {
		t.Errorf("discovery order %q", order)
	}
}

// With a test driver we can see exactly when discovery runs a driver.
func TestExperimentalGate(t *testing.T) {
	calls := map[string]int{}
	mk := func(id string, exp bool) Driver {
		return Driver{
			ID: id, Name: id, Kind: KindDisplay, Transport: TransportHID, Experimental: exp, Order: 900,
			Models:    []Model{{VendorID: 1, ProductID: 1}},
			Enumerate: func() ([]string, error) { calls[id+"/enum"]++; return []string{"path-" + id}, nil },
			Discover:  func() ([]Panel, error) { calls[id+"/disc"]++; return nil, nil },
		}
	}
	Register(mk("t-stable", false))
	Register(mk("t-exp", true))
	t.Cleanup(func() { SetOptions(Options{}) })

	fp := func() string { s, _ := Fingerprint(); return s }

	SetOptions(Options{}) // default: experimental off
	Discover()
	if calls["t-stable/disc"] != 1 || calls["t-exp/disc"] != 0 {
		t.Fatalf("default: %v", calls)
	}
	if f := fp(); !strings.Contains(f, "path-t-stable") || strings.Contains(f, "path-t-exp") {
		t.Fatalf("default fingerprint %q", f)
	}
	if calls["t-exp/enum"] != 0 {
		t.Fatal("a disabled driver was even enumerated")
	}

	// per-driver switches alone do nothing without the global one
	SetOptions(Options{Off: map[string]bool{"t-stable": false}})
	Discover()
	if calls["t-exp/disc"] != 0 {
		t.Fatal("experimental driver ran with the global switch off")
	}

	SetOptions(Options{Experimental: true})
	Discover()
	if calls["t-exp/disc"] != 1 {
		t.Fatalf("enabled: %v", calls)
	}
	if !strings.Contains(fp(), "path-t-exp") {
		t.Fatal("fingerprint misses the enabled driver")
	}

	SetOptions(Options{Experimental: true, Off: map[string]bool{"t-exp": true}})
	Discover()
	if calls["t-exp/disc"] != 1 {
		t.Fatal("individually switched off driver ran")
	}
	if Enabled(mk("t-exp", true)) || !Enabled(mk("x", false)) {
		t.Fatal("Enabled disagrees")
	}

	// the stored options are copies
	o := Options{Experimental: true, Off: map[string]bool{"a": true}}
	SetOptions(o)
	o.Off["a"] = false
	if !CurrentOptions().Off["a"] {
		t.Fatal("options alias the caller's map")
	}
}

func TestRegisterRejectsBadDrivers(t *testing.T) {
	for name, d := range map[string]Driver{
		"no id":       {Name: "x", Kind: KindDisplay, Models: []Model{{}}, Enumerate: func() ([]string, error) { return nil, nil }, Discover: func() ([]Panel, error) { return nil, nil }},
		"no models":   {ID: "bad1", Name: "x", Kind: KindDisplay, Enumerate: func() ([]string, error) { return nil, nil }, Discover: func() ([]Panel, error) { return nil, nil }},
		"no discover": {ID: "bad2", Name: "x", Kind: KindDisplay, Models: []Model{{}}, Enumerate: func() ([]string, error) { return nil, nil }},
		"dup":         {ID: "tllcd", Name: "x", Kind: KindDisplay, Models: []Model{{}}, Enumerate: func() ([]string, error) { return nil, nil }, Discover: func() ([]Panel, error) { return nil, nil }},
		"kind":        {ID: "bad3", Name: "x", Kind: "other", Models: []Model{{}}, Enumerate: func() ([]string, error) { return nil, nil }},
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%s: no panic", name)
				}
			}()
			Register(d)
		}()
	}
}

func TestIsExperimental(t *testing.T) {
	if IsExperimental(&hydroShift{}) != true {
		t.Error("pump head panel not experimental")
	}
	if IsExperimental(&tlLCD{}) || IsExperimental(&universal88{}) {
		t.Error("confirmed panels flagged experimental")
	}
	if !IsExperimental(&universal88{m: &usModel92}) || !IsExperimental(&universal88{m: &usModelSL}) {
		t.Error("new bars not flagged")
	}
}

type pathPanel struct {
	Panel
	path string
}

func (p pathPanel) DevicePath() string { return p.path }

func TestUnopenedDisplays(t *testing.T) {
	old := enumerateDisplays
	defer func() { enumerateDisplays = old }()
	enumerateDisplays = func() []Driver {
		return []Driver{{Enumerate: func() ([]string, error) { return []string{`\?\USB#A`, `\?\usb#b`}, nil }}}
	}
	if got := UnopenedDisplays(nil); len(got) != 2 {
		t.Fatalf("nothing open: %v", got)
	}
	got := UnopenedDisplays([]Panel{pathPanel{path: `\?\usb#a`}})
	if len(got) != 1 || got[0] != `\?\usb#b` {
		t.Fatalf("one open (case-insensitive): %v", got)
	}
	if got := UnopenedDisplays([]Panel{pathPanel{path: `\?\usb#a`}, pathPanel{path: `\?\USB#B`}}); len(got) != 0 {
		t.Fatalf("all open: %v", got)
	}
}
