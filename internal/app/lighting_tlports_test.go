package app

import (
	"image/color"
	"strings"
	"testing"

	leds "github.com/kewe87/casehud/internal/rgb"
	"github.com/kewe87/casehud/internal/sketch"
)

func tlPorts(t *testing.T, v LightingView) map[int]LightingPortView {
	t.Helper()
	d, ok := demoDevice(v, demoFansID)
	if !ok {
		t.Fatal("demo controller missing")
	}
	out := map[int]LightingPortView{}
	for _, p := range d.Ports {
		out[p.Port] = p
	}
	return out
}

// With "follow" the demo controller (port 2: the rear fan, port 3: the three
// side fans) gets colours, effect and speed from the CaseHUD effect and a
// direction per port from the sketch, as an overlay that is never stored; a
// manual port setting wins.
func TestTLPortsFollow(t *testing.T) {
	e := startLit(t)
	withTLParts(t, e)
	fan := func(id, facing, label string, x, y, z float64) sketch.Placement {
		return sketch.Placement{ID: id, Part: "uni-fan-tl-lcd-reverse-120", Facing: facing, Pos: sketch.Vec3{X: x, Y: y, Z: z}, Label: label}
	}
	if _, err := e.SetSketch(testSketch(fan("rear", sketch.FacingRear, "Rear", 60, 90, 14),
		fan("s1", sketch.FacingSide, "Seite 1", 14, 112, 300), fan("s2", sketch.FacingSide, "Seite 2", 14, 236, 300),
		fan("s3", sketch.FacingSide, "Seite 3", 14, 360, 300))); err != nil {
		t.Fatal(err)
	}
	effect := "meteor"
	patch := func(p LightingPatch) LightingView {
		t.Helper()
		v, err := e.PatchLighting(p)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	patch(LightingPatch{Effect: &effect, Params: map[string]any{"path": "vertical", "colors": 2.0, "color1": "#ff0000", "color2": "#00ff00", "speed": 45.0}})

	// mainboard sync: no port plan
	if d, _ := demoDevice(e.Lighting(), demoFansID); len(d.Ports) != 0 {
		t.Fatalf("ports in sync mode: %+v", d.Ports)
	}
	// own effect without follow: every port like all ports
	v := patch(LightingPatch{Devices: map[string]LightingDevicePatch{demoFansID: {Settings: map[string]any{"mode": "hardware", "allowFlashWrites": true, "effect": "wave"}}}})
	ps := tlPorts(t, v)
	if len(ps) != 2 || ps[3].Source != "all" || ps[3].Effect != "wave" || strings.Join(ps[3].Labels, ",") != "Seite 1,Seite 2,Seite 3" || ps[2].Labels[0] != "Rear" {
		t.Fatalf("own effect: %+v", ps)
	}
	if ov := e.light.engPtr.Load().DeviceOverlay(demoFansID); ov != nil {
		t.Fatalf("overlay without follow: %v", ov)
	}

	// follow: derived from the effect and the sketch
	v = patch(LightingPatch{Devices: map[string]LightingDevicePatch{demoFansID: {Settings: map[string]any{"follow": true}}}})
	ps = tlPorts(t, v)
	if ps[3].Source != "sketch" || ps[3].Direction != "down" || ps[3].Effect != "meteor" {
		t.Fatalf("follow, side fans: %+v", ps[3])
	}
	ov := e.light.engPtr.Load().DeviceOverlay(demoFansID)
	if ov.String("effect") != "meteor" || ov.String("color1") != "#ff0000" || ov.String("color2") != "#00ff00" || ov.Float("colors") != 2 ||
		ov.Float("speed") != 2 || ov.String("port3Direction") != "down" {
		t.Fatalf("overlay %v", ov)
	}
	stored := e.light.engPtr.Load().Config().Devices[demoFansID].Settings
	if _, ok := stored["port3Direction"]; ok || stored.String("effect") != "wave" {
		t.Fatalf("derived values stored: %v", stored)
	}
	// the demo controller is configured with them
	ctl := e.light.demoCtl.Load()
	waitUntil(t, "overlay configured", func() bool { return ctl.configured().String("port3Direction") == "down" })

	// a manual port setting wins
	v = patch(LightingPatch{Devices: map[string]LightingDevicePatch{demoFansID: {Settings: map[string]any{"port3Direction": "up", "port2Effect": "static"}}}})
	ps = tlPorts(t, v)
	if ps[3].Source != "manual" || ps[3].Direction != "up" || ps[2].EffectSource != "manual" || ps[2].Effect != "static" {
		t.Fatalf("manual: %+v", ps)
	}
	if ov := e.light.engPtr.Load().DeviceOverlay(demoFansID); ov["port3Direction"] != nil || ov["port2Effect"] != nil {
		t.Fatalf("overlay overrides the manual setting: %v", ov)
	}

	// an effect without motion: colours only, the ports run like all ports
	solid := "solid"
	v = patch(LightingPatch{Effect: &solid, Params: map[string]any{"color": "#123456"}})
	ps = tlPorts(t, v)
	ov = e.light.engPtr.Load().DeviceOverlay(demoFansID)
	if ps[2].Source != "all" || ov.String("effect") != "static" || ov.String("color1") != "#123456" || ov.Float("colors") != 1 {
		t.Fatalf("solid: %+v %v", ps, ov)
	}

	// back to mainboard sync: the overlay goes away
	patch(LightingPatch{Devices: map[string]LightingDevicePatch{demoFansID: {Settings: map[string]any{"mode": "motherboard", "allowFlashWrites": false}}}})
	if ov := e.light.engPtr.Load().DeviceOverlay(demoFansID); ov != nil {
		t.Fatalf("overlay in sync mode: %v", ov)
	}
}

func TestEffectColors(t *testing.T) {
	cases := []struct {
		p    leds.Params
		want string
	}{
		{leds.Params{"color1": "#ff0000", "color2": "#00ff00", "color3": "#0000ff", "colors": 2.0}, "#ff0000,#00ff00"},
		{leds.Params{"color1": "#010101", "color2": "#020202", "color3": "#030303", "color4": "#040404", "color5": "#050505", "colors": 6.0}, "#010101,#020202,#030303,#040404"},
		{leds.Params{"color": "#3a8dff", "background": "#000000"}, "#3a8dff"},
		{leds.Params{"color": "#3a8dff", "background": "#1a0830", "useAccent": true}, "#009bde,#1a0830"},
		{leds.Params{"low": "#0060ff", "high": "#ff2000"}, "#0060ff,#ff2000"},
		{leds.Params{"color": "#ff3c00", "tip": "#ffd040"}, "#ff3c00,#ffd040"},
		{leds.Params{"speed": 50.0}, ""},
	}
	for _, c := range cases {
		got := strings.Join(effectColors(c.p, color.RGBA{0, 155, 222, 255}), ",")
		if got != c.want {
			t.Errorf("%v: %q, want %q", c.p, got, c.want)
		}
	}
}

func TestDropStalePortSettings(t *testing.T) {
	decl := []leds.Param{{Key: "port2Effect"}, {Key: "mode"}}
	got := dropStalePortSettings(decl, leds.Params{"port2Effect": "x", "port4Effect": "y", "port4Direction": "z", "mode": "hardware", "other": 1})
	if len(got) != 3 || got["port2Effect"] != "x" || got["other"] != 1 {
		t.Fatalf("%v", got)
	}
}
