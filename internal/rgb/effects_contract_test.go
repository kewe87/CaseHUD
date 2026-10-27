package leds

import (
	"sort"
	"strings"
	"testing"
)

// The effect names and parameter keys are a public contract: lighting presets
// in the design library refer to them. A change here must be deliberate and
// announced (changelog).
var effectContract = map[string]string{
	"rainbow":      "scale speed vertical",
	"solid":        "color useAccent",
	"breathing":    "color period useAccent",
	"wave":         "background color speed useAccent width",
	"sensor":       "high low max min sensor smoothing",
	"cycle":        "speed",
	"meteor":       "color1 color2 color3 color4 color5 color6 colors mirror path reverse speed tail",
	"fill":         "alternate color1 color2 color3 color4 color5 color6 colors mirror path reverse speed",
	"burst":        "color1 color2 color3 color4 color5 color6 colors mirror path reverse speed",
	"shuttle":      "color1 color2 colors mirror path reverse size speed",
	"bounce":       "balls color1 color2 color3 color4 color5 color6 colors mirror path reverse speed",
	"stack":        "color1 color2 color3 color4 color5 color6 colors mirror path reverse speed",
	"sparkle":      "color1 color2 color3 color4 color5 color6 colors density mirror path reverse speed",
	"stripes":      "color1 color2 colors count mirror path reverse speed",
	"sand":         "color1 color2 color3 color4 colors mirror path reverse speed",
	"spark":        "color1 color2 color3 color4 colors mirror path reverse speed",
	"meet":         "color1 color2 colors mirror path reverse speed",
	"rainbow-gaps": "gaps mirror path reverse speed",
	"fire":         "color height speed tip",
}

func TestEffectContract(t *testing.T) {
	got := map[string]string{}
	for _, e := range BuiltinEffects() {
		var keys []string
		for _, p := range e.Params {
			keys = append(keys, p.Key)
		}
		sort.Strings(keys)
		got[e.Name] = strings.Join(keys, " ")
	}
	for name, keys := range effectContract {
		if got[name] != keys {
			t.Errorf("effect %q: params %q, contract %q", name, got[name], keys)
		}
	}
	for name := range got {
		if _, ok := effectContract[name]; !ok {
			t.Errorf("effect %q is not in the contract (add it deliberately)", name)
		}
	}
}
