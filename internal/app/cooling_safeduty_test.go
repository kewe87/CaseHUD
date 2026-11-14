package app

import (
	"strings"
	"testing"
)

func TestSafeDutyBelowTheFloorIsRefusedOnARegulatedOutput(t *testing.T) {
	for _, bad := range []string{"0", "1", "29", "101", "-5"} {
		file := strings.Replace(coolingExample, `"safe_duty":80`, `"safe_duty":`+bad, 1)
		if _, err := parseCoolingConfig([]byte(file)); err == nil || !strings.Contains(err.Error(), "safe_duty") {
			t.Errorf("safe_duty %s: %v", bad, err)
		}
		pump := strings.Replace(coolingExample, `"safe_duty":100`, `"safe_duty":`+bad, 1)
		if _, err := parseCoolingConfig([]byte(pump)); err == nil {
			t.Errorf("pump with safe_duty %s was accepted", bad)
		}
	}
	for _, ok := range []string{"30", "80", "100"} {
		file := strings.Replace(coolingExample, `"safe_duty":80`, `"safe_duty":`+ok, 1)
		if _, err := parseCoolingConfig([]byte(file)); err != nil {
			t.Errorf("safe_duty %s: %v", ok, err)
		}
	}
}

// A file saved before the check may carry 0 on an output nobody regulates. It must
// still load, with a value that cannot leave a fan standing once a curve is set.
func TestSafeDutyOfAnUnregulatedOutputLoadsAndIsLifted(t *testing.T) {
	file := `{"outputs":[{"id":"port1-fan1","name":"Free","step_up":5,"step_down":3,"safe_duty":0},
	{"id":"port1-fan2","name":"Free","step_up":5,"step_down":3,"safe_duty":250}]}`
	c, err := parseCoolingConfig([]byte(file))
	if err != nil {
		t.Fatal(err)
	}
	if c.Outputs[0].SafeDuty != 30 || c.Outputs[1].SafeDuty != 100 {
		t.Fatalf("%d %d", c.Outputs[0].SafeDuty, c.Outputs[1].SafeDuty)
	}
	// Putting it on a curve afterwards works with the lifted value.
	c.Outputs[0].Curve = ""
	if err := c.check(); err != nil {
		t.Fatal(err)
	}
}
