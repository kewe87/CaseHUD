package app

import (
	"context"
	"slices"
	"testing"
)

func fanIDs(e *Engine) []string {
	var out []string
	for _, p := range e.Panels() {
		if isFanID(p.ID) {
			out = append(out, p.ID)
		}
	}
	return out
}

func TestAssignFanNumbers(t *testing.T) {
	ids := []string{"a", "b", "c", "d"}
	// No stored numbers: chain order.
	got := assignFanNumbers(ids, nil)
	if got["a"] != 1 || got["b"] != 2 || got["c"] != 3 || got["d"] != 4 {
		t.Fatalf("%v", got)
	}
	// Stored numbers stay; the rest fill the gaps. A duplicate loses.
	got = assignFanNumbers(ids, map[string]int{"a": 2, "c": 2, "d": 0})
	if got["a"] != 2 || got["b"] != 1 || got["c"] != 3 || got["d"] != 4 {
		t.Fatalf("%v", got)
	}
}

func TestFanNumbersBackCompat(t *testing.T) {
	e := startDemo(t)
	ps := e.Panels()
	// Demo: pump head, then four fans. Without stored numbers the fans are
	// numbered in chain order and the other display continues behind them.
	var nums []int
	for _, p := range ps {
		if isFanID(p.ID) {
			nums = append(nums, p.Number)
		} else if p.Number != 5 {
			t.Fatalf("other display number %d", p.Number)
		}
	}
	if !slices.Equal(nums, []int{1, 2, 3, 4}) {
		t.Fatalf("%v", nums)
	}
	if got := fanIDs(e); got[0] != "tllcd-p1-i0" || got[3] != "tllcd-p2-i2" {
		t.Fatalf("%v", got)
	}
}

func TestSetFanNumberSwapsAndSorts(t *testing.T) {
	e := startDemo(t)
	// Owner's numbering: port 2 fan = 2; port 3 chain = 4, 1, 3 (so one number is 2 ... swap).
	if err := e.SetFanNumber("tllcd-p2-i0", 4); err != nil { // was 2; 4 belongs to p2-i2 -> swaps to 2
		t.Fatal(err)
	}
	n := e.FanNumbers()
	if n["tllcd-p2-i0"] != 4 || n["tllcd-p2-i2"] != 2 || n["tllcd-p1-i0"] != 1 || n["tllcd-p2-i1"] != 3 {
		t.Fatalf("%v", n)
	}
	if err := e.SetFanNumber("tllcd-p2-i1", 1); err != nil { // swaps with p1-i0
		t.Fatal(err)
	}
	if err := e.SetFanNumber("tllcd-p1-i0", 2); err != nil { // swaps with p2-i2 (had 2)
		t.Fatal(err)
	}
	n = e.FanNumbers()
	want := map[string]int{"tllcd-p1-i0": 2, "tllcd-p2-i0": 4, "tllcd-p2-i1": 1, "tllcd-p2-i2": 3}
	for id, v := range want {
		if n[id] != v {
			t.Fatalf("%s = %d, want %d (%v)", id, n[id], v, n)
		}
	}
	// Panels are ordered by number now, the pump head keeps its slot.
	if got := fanIDs(e); !slices.Equal(got, []string{"tllcd-p2-i1", "tllcd-p1-i0", "tllcd-p2-i2", "tllcd-p2-i0"}) {
		t.Fatalf("%v", got)
	}
	// Persisted and applied after a restart.
	e2 := &Engine{Store: e.Store, Demo: true}
	if err := e2.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer e2.Stop()
	if got := e2.FanNumbers(); got["tllcd-p2-i1"] != 1 || got["tllcd-p2-i0"] != 4 {
		t.Fatalf("after restart %v", got)
	}
	// Errors, and reset.
	if e.SetFanNumber("tllcd-p1-i0", 5) == nil || e.SetFanNumber("tllcd-p1-i0", 0) == nil || e.SetFanNumber("aio-7398-0", 1) == nil || e.SetFanNumber("nope", 1) == nil {
		t.Fatal("bad numbers accepted")
	}
	if err := e.ResetFanNumbers(); err != nil {
		t.Fatal(err)
	}
	if got := fanIDs(e); got[0] != "tllcd-p1-i0" {
		t.Fatalf("%v", got)
	}
}

func TestIdentifyUsesFanNumber(t *testing.T) {
	e := startDemo(t)
	if err := e.SetFanNumber("tllcd-p1-i0", 3); err != nil {
		t.Fatal(err)
	}
	if err := e.Identify("tllcd-p1-i0"); err != nil {
		t.Fatal(err)
	}
	if w := e.workers["tllcd-p1-i0"]; w.idNum != 3 {
		t.Fatalf("shows %d", w.idNum)
	}
	if err := e.IdentifyPort(3); err != nil { // device port 2
		t.Fatal(err)
	}
	if e.workers["tllcd-p2-i1"].idNum == 0 || e.IdentifyPort(4) == nil || e.IdentifyPort(1) == nil {
		t.Fatal("port identify")
	}
}

func TestArrangementPortsAndWarnings(t *testing.T) {
	e := startDemo(t)
	a := e.Arrangement()
	if len(a.Ports) != 4 || len(a.Warnings) != 0 {
		t.Fatalf("%+v", a)
	}
	if a.Ports[0].LCD || len(a.Ports[0].Fans) != 0 || len(a.Ports[1].Fans) != 1 || len(a.Ports[2].Fans) != 3 || !a.Ports[1].LCD {
		t.Fatalf("%+v", a.Ports)
	}
	if f := a.Ports[2].Fans[1]; f.ID != "tllcd-p2-i1" || f.Port != 3 || f.Index != 1 || !f.HasLCD || f.Number != 3 {
		t.Fatalf("%+v", f)
	}

	// A fan reporting device port 0 sits on physical port 1: warn.
	// Too many on one port and in total, too many fans on a port.
	var fans []FanPos
	fans = append(fans, FanPos{Port: 1, Index: 0, HasLCD: true, ID: "x"})
	for i := 0; i < 4; i++ {
		fans = append(fans, FanPos{Port: 2, Index: i, HasLCD: true, ID: "y"})
	}
	for p := 3; p <= 4; p++ {
		for i := 0; i < 3; i++ {
			fans = append(fans, FanPos{Port: p, Index: i, HasLCD: true, ID: "z"})
		}
	}
	for i := 0; i < 11; i++ { // a fan without display at an LCD's place loses
		fans = append(fans, FanPos{Port: 5, Index: i})
	}
	fans = append(fans, FanPos{Port: 2, Index: 0})
	b := buildArrangement(fans, nil)
	codes := map[string]bool{}
	for _, w := range b.Warnings {
		codes[w.Code] = true
	}
	for _, c := range []string{"lcdPort1", "portLCD", "portFans"} {
		if !codes[c] {
			t.Errorf("missing warning %s: %+v", c, b.Warnings)
		}
	}
	if codes["totalLCD"] { // 1+4+3+3 = 11 <= 13
		t.Errorf("unexpected totalLCD")
	}
	if !b.Ports[1].Fans[0].HasLCD || len(b.Ports) != 5 {
		t.Errorf("lcd must win at a shared place: %+v", b.Ports[1])
	}
	for i := 0; i < 3; i++ {
		fans = append(fans, FanPos{Port: 3, Index: 3 + i, HasLCD: true, ID: "w"})
	}
	if c := buildArrangement(fans, nil); !slices.ContainsFunc(c.Warnings, func(w ArrangeWarning) bool { return w.Code == "totalLCD" }) {
		t.Errorf("no totalLCD: %+v", c.Warnings)
	}
}

type extraFans []FanPos

func (x extraFans) Fans() []FanPos { return x }

func TestArrangementTopologySource(t *testing.T) {
	e := startDemo(t)
	e.Topology = extraFans{{Port: 1, Index: 0}, {Port: 1, Index: 1}, {Port: 1, Index: 2}, {Port: 2, Index: 0}}
	a := e.Arrangement()
	if len(a.Ports[0].Fans) != 3 || a.Ports[0].Fans[0].HasLCD || !a.Ports[1].Fans[0].HasLCD {
		t.Fatalf("%+v", a.Ports)
	}
}
