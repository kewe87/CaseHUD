package sketch

import (
	"fmt"
	"reflect"
	"testing"

	leds "github.com/kewe87/casehud/internal/rgb"
)

func portParts(ps []TLPortFans) []string {
	var out []string
	for _, p := range ps {
		out = append(out, fmt.Sprintf("%d:%v", p.Port, p.Parts))
	}
	return out
}

// The ports are found from the case sketch whether the fans are not bound,
// bound to the ports, or bound to the mainboard header the controller follows.
func TestTLPortLayout(t *testing.T) {
	c := autoCatalog(t)
	want := []string{"1:[p3 p5 p6]", "2:[p13]", "3:[p1 p4 p8]"}
	hw := kevinHW()
	s := ResolveFaces(kevinSketch(false, true), c)
	got := TLPortLayout(s, c, hw.Devices[0], hw.Fans)
	if !reflect.DeepEqual(portParts(got), want) {
		t.Fatalf("unbound: %v", portParts(got))
	}
	for _, p := range got {
		if len(p.Fans) != len(p.Parts) || len(p.Fans[0].Canvas) != 26 || len(p.Fans[0].Local) != 26 {
			t.Fatalf("port %d: fans %d", p.Port, len(p.Fans))
		}
		for _, f := range p.Fans {
			for _, q := range f.Canvas {
				if q.X < 0 || q.X > 1 || q.Y < 0 || q.Y > 1 {
					t.Fatalf("port %d: LED off the canvas %v", p.Port, q)
				}
			}
		}
	}
	if got[2].Labels[0] != "Side 1" {
		t.Fatalf("labels %v", got[2].Labels)
	}

	// bound to the ports (hardware mode)
	bound := ApplyAutoBind(s, AutoBind(s, c, hw))
	if got := TLPortLayout(bound, c, hw.Devices[0], hw.Fans); !reflect.DeepEqual(portParts(got), want) || !got[0].Direct {
		t.Fatalf("bound to ports: %v", portParts(got))
	}

	// bound to the mainboard header (sync mode)
	sync := kevinSyncHW(kevinBoard + "/argb1")
	synced := ApplyAutoBind(s, AutoBind(s, c, sync))
	if synced.Parts[findPart(synced, "p1")].Bind == nil {
		t.Fatal("sync bindings missing")
	}
	if got := TLPortLayout(synced, c, sync.Devices[0], sync.Fans); !reflect.DeepEqual(portParts(got), want) || got[0].Direct {
		t.Fatalf("bound to the header: %v", portParts(got))
	}
}

// The direction follows the effect over the case: a vertical path runs down
// the side fans (and back up when reversed), a horizontal one across the top
// fans; fans the path does not run across in any firmware direction light as
// a whole.
func TestTLDirection(t *testing.T) {
	c := autoCatalog(t)
	hw := kevinHW()
	s := ResolveFaces(kevinSketch(false, true), c)
	ports := TLPortLayout(s, c, hw.Devices[0], hw.Fans)
	type res struct {
		dir   string
		whole bool
	}
	run := func(effect string, p leds.Params) map[int]res {
		u, circ, ok := leds.MotionField(effect, p)
		if !ok {
			t.Fatalf("%s: no motion", effect)
		}
		out := map[int]res{}
		for _, pf := range ports {
			d, _, whole := TLDirection(pf.Fans, u, circ)
			out[pf.Port] = res{d, whole}
		}
		return out
	}
	if r := run("meteor", leds.Params{"path": "vertical"}); r[3] != (res{"down", false}) || r[2] != (res{"down", false}) || !r[1].whole {
		t.Fatalf("vertical: %v", r)
	}
	if r := run("meteor", leds.Params{"path": "vertical", "reverse": true}); r[3] != (res{"up", false}) {
		t.Fatalf("vertical reversed: %v", r)
	}
	if r := run("wave", leds.Params{}); r[1] != (res{"down", false}) || !r[3].whole {
		t.Fatalf("wave: %v", r)
	}
	if r := run("fire", leds.Params{}); r[3] != (res{"up", false}) {
		t.Fatalf("fire: %v", r)
	}
	// around: every fan sits off the centre, the path runs across each of them
	for port, x := range run("meteor", leds.Params{"path": "around"}) {
		if x.whole {
			t.Fatalf("around: port %d whole", port)
		}
	}
	if _, _, ok := leds.MotionField("solid", leds.Params{"color": "#ffffff"}); ok {
		t.Fatal("solid has no motion")
	}
	// constant u: one point
	if _, _, whole := TLDirection(ports[0].Fans, func(_, _ float64) float64 { return 0.3 }, false); !whole {
		t.Fatal("constant field not whole")
	}
}

func TestTLDirectionTable(t *testing.T) {
	var names []string
	for _, d := range TLDirections {
		names = append(names, d.Name)
	}
	// the firmware's direction values, in wire order (lianlitl)
	if !reflect.DeepEqual(names, []string{"clockwise", "counter-clockwise", "up", "down", "spread", "gather"}) {
		t.Fatalf("directions %v", names)
	}
}
