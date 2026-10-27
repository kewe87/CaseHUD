package main

import (
	"errors"
	"testing"

	"github.com/kewe87/casehud/internal/app"
)

func TestParseOnOff(t *testing.T) {
	for v, want := range map[string][2]bool{"": {false, false}, "on": {true, true}, "ON": {true, true}, "1": {true, true}, "off": {false, true}, "0": {false, true}} {
		on, given, err := parseOnOff(v)
		if err != nil || on != want[0] || given != want[1] {
			t.Errorf("%q: %v %v %v", v, on, given, err)
		}
	}
	if _, _, err := parseOnOff("maybe"); err == nil {
		t.Error("expected error")
	}
}

func TestSetAPISettingWritesConfig(t *testing.T) {
	dir := t.TempDir()
	st := &app.Store{Dir: dir}
	c, _ := st.LoadConfig()
	c.Fahrenheit = true
	if err := st.SaveConfig(c); err != nil {
		t.Fatal(err)
	}
	if err := setAPISetting(dir, true); err != nil {
		t.Fatal(err)
	}
	c, _ = st.LoadConfig()
	if !c.API || !c.Fahrenheit {
		t.Fatalf("%+v", c)
	}
	if err := setAPISetting(dir, false); err != nil {
		t.Fatal(err)
	}
	if c, _ = st.LoadConfig(); c.API {
		t.Fatal("API still on")
	}
}

func TestRefuseService(t *testing.T) {
	for sid, refused := range map[string]bool{"S-1-5-18": true, "S-1-5-19": true, "S-1-5-20": true, "S-1-5-21-1-2-3-1001": false} {
		err := refuseService(func() (string, error) { return sid, nil })
		if (err != nil) != refused {
			t.Errorf("%s: err %v", sid, err)
		}
	}
	if refuseService(func() (string, error) { return "", errors.New("x") }) == nil {
		t.Error("lookup failure must refuse")
	}
}
