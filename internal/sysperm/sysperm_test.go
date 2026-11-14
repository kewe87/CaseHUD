package sysperm

import "testing"

func TestCoolingHeaderKeys(t *testing.T) {
	for _, k := range []string{"/lpc/nct6701d/0/control/6", "/lpc/it8689e/0/control/12"} {
		if !ValidCoolingHeader(k) {
			t.Errorf("%q should be valid", k)
		}
	}
	for _, k := range []string{
		"", "/lpc/nct6701d/0/control/6\n", "/lpc/nct6701d/0/control/6 ", "/lpc/../control/6",
		"/gpu-nvidia/0/control/0", "/lpc/nct6701d/0/fan/6", "/lpc/nct6701d/0/control/123",
		"/lpc/NCT/0/control/6", "/lpc/nct6701d/0/control/6\" --x", "/lpc/nct6701d/0/control/6/x",
	} {
		if ValidCoolingHeader(k) {
			t.Errorf("%q should be refused", k)
		}
	}
}

func TestAllowCoolingHeaderRefusesBadKeysBeforeAnyPrompt(t *testing.T) {
	// Nothing is started for a key that does not pass.
	if err := AllowCoolingHeader("/lpc/x/0/control/1\" --evil"); err == nil {
		t.Fatal("expected an error")
	}
}
