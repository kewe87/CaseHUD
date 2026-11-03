package app

import (
	"context"
	"errors"
	"image"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	leds "github.com/kewe87/casehud/internal/rgb"
)

// fakeCapture replaces the screen and audio effects by ones that read nothing,
// so no test touches the real screen or audio device.
func fakeCapture(t *testing.T) {
	t.Helper()
	orig := lightingEffects
	swapped := make([]leds.EffectInfo, len(orig))
	for i, info := range orig {
		if isCaptureEffect(info.Name) {
			info.New = func(leds.Params) (leds.Effect, error) { return blankEffect{}, nil }
		}
		swapped[i] = info
	}
	lightingEffects = swapped
	t.Cleanup(func() { lightingEffects = orig })
}

type blankEffect struct{}

func (blankEffect) Render(leds.Frame, *image.RGBA) {}

// Names and parameter keys of the capture effects are part of the API.
func TestCaptureEffectContract(t *testing.T) {
	want := map[string]string{
		"screen": "area display fps saturation smoothing",
		"audio":  "bands decay high low mode rainbow sensitivity useAccent",
	}
	for name, keys := range want {
		info, ok := lightingEffect(name)
		if !ok {
			t.Fatalf("effect %q missing", name)
		}
		var got []string
		for _, p := range info.Params {
			got = append(got, p.Key)
		}
		sort.Strings(got)
		if strings.Join(got, " ") != keys {
			t.Errorf("%s params %q, contract %q", name, got, keys)
		}
		if !lightingIsFeatured(name) || !isCaptureEffect(name) {
			t.Errorf("%s must be featured and a capture effect", name)
		}
	}
	info, _ := lightingEffect("screen")
	for _, p := range info.Params {
		if p.Key == "area" && p.Default != "edges" {
			t.Errorf("area default %v", p.Default)
		}
		if p.Key == "fps" && p.Default != 15.0 {
			t.Errorf("fps default %v", p.Default)
		}
	}
	if _, ok := lightingEffect("video"); ok {
		t.Error("the video effect reads files and must not be offered")
	}
}

func TestCaptureNeedsConsent(t *testing.T) {
	fakeCapture(t)
	e := startDemo(t)
	for _, fx := range []string{"screen", "audio"} {
		if _, err := e.PatchLighting(LightingPatch{Effect: ptr(fx)}); !errors.Is(err, ErrAckRequired) {
			t.Fatalf("%s without consent: %v", fx, err)
		}
	}
	if e.Acks().AckCapture {
		t.Fatal("consent on record by default")
	}
	if v := e.Lighting(); v.Effect == "screen" || v.Effect == "audio" {
		t.Fatalf("effect changed despite the refusal: %s", v.Effect)
	}
	if err := e.Acknowledge("capture"); err != nil {
		t.Fatal(err)
	}
	if !e.Acks().AckCapture {
		t.Fatal("consent not recorded")
	}
	v, err := e.PatchLighting(LightingPatch{Enabled: ptr(true), Effect: ptr("screen"), Params: map[string]any{"area": "full", "fps": 500.0}})
	if err != nil {
		t.Fatal(err)
	}
	if v.Effect != "screen" || v.Params["area"] != "full" || v.Params["fps"] != 30.0 {
		t.Fatalf("view: %+v", v.Params)
	}
	if _, err := e.PatchLighting(LightingPatch{Params: map[string]any{"area": "everywhere"}}); !errors.Is(err, ErrLighting) {
		t.Fatalf("bad enum: %v", err)
	}
	e.saveLighting()
	cfg, err := e.Store.LoadConfig()
	if err != nil || !cfg.AckCapture {
		t.Fatalf("consent not persisted: %v %v", cfg.AckCapture, err)
	}
}

// A hand-edited file cannot switch the capture on without the confirmation.
func TestCaptureStoredWithoutConsent(t *testing.T) {
	fakeCapture(t)
	for _, consent := range []bool{false, true} {
		dir := t.TempDir()
		cfg := `{"lighting":{"enabled":true,"engine":{"version":1,"effect":"screen"}},"panels":{}`
		if consent {
			cfg += `,"ack_capture":true`
		}
		cfg += `}`
		if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(cfg), 0o600); err != nil {
			t.Fatal(err)
		}
		e := &Engine{Store: &Store{Dir: dir}, Demo: true}
		if err := e.Start(context.Background()); err != nil {
			t.Fatal(err)
		}
		got := e.Lighting().Effect
		e.Stop()
		if consent && got != "screen" || !consent && got == "screen" {
			t.Fatalf("consent %v: effect %q", consent, got)
		}
	}
}

func TestCaptureNotSavedAsPreset(t *testing.T) {
	fakeCapture(t)
	e := startDemo(t)
	e.Acknowledge("capture")
	if _, err := e.PatchLighting(LightingPatch{Enabled: ptr(true), Effect: ptr("audio")}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.SaveLightingPreset("mine"); err == nil {
		t.Fatal("a capture effect was saved as a preset")
	}
	for _, p := range e.LightingPresetList() {
		if isCaptureEffect(p.Effect) {
			t.Fatalf("preset %s selects %s", p.ID, p.Effect)
		}
	}
}

func TestCaptureBlocksTrace(t *testing.T) {
	fakeCapture(t)
	e := startDemo(t)
	e.Acknowledge("capture")
	if _, err := e.PatchLighting(LightingPatch{Enabled: ptr(true), Effect: ptr("screen")}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.StartLightTrace(); !errors.Is(err, ErrTrace) {
		t.Fatalf("trace while the screen effect runs: %v", err)
	}
	if e.packetsAllowed() {
		t.Fatal("packets exportable while the screen effect runs")
	}
	// a recording that is running when the effect is switched on ends and stays out
	if _, err := e.PatchLighting(LightingPatch{Effect: ptr("solid")}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.StartLightTrace(); err != nil {
		t.Fatal(err)
	}
	if _, err := e.PatchLighting(LightingPatch{Effect: ptr("audio")}); err != nil {
		t.Fatal(err)
	}
	if on, _ := e.LightTraceStatus(); on {
		t.Fatal("recording continues under a capture effect")
	}
	if _, err := e.PatchLighting(LightingPatch{Effect: ptr("solid")}); err != nil {
		t.Fatal(err)
	}
	if e.packetsAllowed() {
		t.Fatal("packets recorded next to a capture effect are exportable")
	}
	if _, err := e.StartLightTrace(); err != nil {
		t.Fatal(err)
	}
	if !e.packetsAllowed() {
		t.Fatal("a new recording is blocked for good")
	}
	e.light.stopTrace()
}

// Withdrawing the confirmation ends a running capture effect and is saved.
func TestRevokeCapture(t *testing.T) {
	fakeCapture(t)
	e := startDemo(t)
	e.Acknowledge("capture")
	if _, err := e.PatchLighting(LightingPatch{Enabled: ptr(true), Effect: ptr("screen")}); err != nil {
		t.Fatal(err)
	}
	if err := e.RevokeCapture(); err != nil {
		t.Fatal(err)
	}
	if e.Acks().AckCapture {
		t.Fatal("consent still on record")
	}
	if v := e.Lighting(); isCaptureEffect(v.Effect) {
		t.Fatalf("capture effect still selected: %s", v.Effect)
	}
	cfg, err := e.Store.LoadConfig()
	if err != nil || cfg.AckCapture {
		t.Fatalf("revocation not persisted: %v %v", cfg.AckCapture, err)
	}
	if _, err := e.PatchLighting(LightingPatch{Effect: ptr("audio")}); !errors.Is(err, ErrAckRequired) {
		t.Fatalf("audio after revocation: %v", err)
	}
}
