package app

import "testing"

func TestLightingDeviceOffPersists(t *testing.T) {
	e := startDemo(t)
	if _, err := e.PatchLighting(LightingPatch{Enabled: ptr(true), Effect: ptr("wave")}); err != nil {
		t.Fatal(err)
	}
	v, err := e.PatchLighting(LightingPatch{Devices: map[string]LightingDevicePatch{"demo:ram": {Off: ptr(true)}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range v.Devices {
		if d.Off != (d.ID == "demo:ram") {
			t.Fatalf("%s: off %v", d.ID, d.Off)
		}
	}
	if v.Effect != "wave" {
		t.Fatalf("effect changed: %s", v.Effect)
	}
	e.saveLighting()
	cfg, _ := e.Store.LoadConfig()
	if cfg.Lighting.Engine == nil || !cfg.Lighting.Engine.Devices["demo:ram"].Off {
		t.Fatalf("off not persisted: %+v", cfg.Lighting.Engine)
	}
	v, err = e.PatchLighting(LightingPatch{Devices: map[string]LightingDevicePatch{"demo:ram": {Off: ptr(false)}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range v.Devices {
		if d.Off {
			t.Fatalf("%s still off", d.ID)
		}
	}
}
