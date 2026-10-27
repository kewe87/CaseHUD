package panel

import (
	"strings"
	"testing"
	"time"
)

func rateSetup(t *testing.T, interval time.Duration) {
	t.Helper()
	old := tlMinInterval
	tlMinInterval = interval
	t.Cleanup(func() { tlMinInterval = old; tlNow = time.Now })
}

func nSettings(f *echoHID) int {
	tlChain.enter()
	defer tlChain.leave()
	return len(f.settingsWrites())
}

func TestTLSettingsCoalescedToLastValue(t *testing.T) {
	rateSetup(t, 60*time.Millisecond)
	f := &echoHID{}
	p := &tlLCD{dev: f, brightness: 50}
	p.SetBrightness(10) // now
	for _, v := range []int{20, 30, 40, 80} {
		p.SetBrightness(v) // postponed, coalesced
	}
	if n := nSettings(f); n != 1 {
		t.Fatalf("%d writes inside the interval, want 1", n)
	}
	time.Sleep(250 * time.Millisecond)
	if n := nSettings(f); n != 2 {
		t.Fatalf("%d writes after the interval, want 2", n)
	}
	tlChain.enter()
	last := f.settingsWrites()[1]
	tlChain.leave()
	if last[11+4] != 80 {
		t.Fatalf("applied brightness %d, want the last value 80", last[11+4])
	}
}

func TestTLSettingsPendingGoesIntoClose(t *testing.T) {
	rateSetup(t, time.Hour)
	f := &echoHID{}
	p := &tlLCD{dev: f, brightness: 50}
	p.SetBrightness(10)
	p.SetBrightness(90) // pending
	p.Close()
	tlChain.enter()
	ws := f.settingsWrites()
	tlChain.leave()
	if len(ws) != 2 || ws[1][11] != byte(tlViewStored) || ws[1][11+4] != 90 {
		t.Fatalf("writes %d", len(ws))
	}
}

func TestTLSettingsDailyCap(t *testing.T) {
	rateSetup(t, 0)
	f := &echoHID{}
	p := &tlLCD{dev: f, brightness: 50}
	for i := 0; i < tlDailyMax+50; i++ {
		p.SetBrightness(i % 100)
	}
	n := nSettings(f)
	if n > tlDailyMax {
		t.Fatalf("%d settings writes, cap is %d", n, tlDailyMax)
	}
	if !p.rate.capped {
		t.Fatal("cap not reached")
	}
	// Still capped after the day changes: only a restart lifts it.
	tlNow = func() time.Time { return time.Now().Add(48 * time.Hour) }
	p.SetBrightness(99)
	if nSettings(f) != n {
		t.Fatal("written after the cap")
	}
}

func TestTLSerialNeedsDedupeWithoutPath(t *testing.T) {
	resetSerialTries()
	h := newFanHID(tlSerial("", 1, 2))
	p := &tlLCD{dev: h, brightness: 50} // no USB path
	p.start()
	h2 := newFanHID(tlSerial("", 1, 2))
	q := &tlLCD{dev: h2, brightness: 50}
	q.start()
	if len(serialWrites(&h.fakeHID))+len(serialWrites(&h2.fakeHID)) != 1 {
		t.Fatal("serial written twice for the same position")
	}
	// Neither path nor position: nothing is written.
	h3 := newFanHID(tlSerial("", 9, 9))
	r := &tlLCD{dev: h3, brightness: 50}
	r.assignSerialIfEmpty(make([]byte, 40))
	if len(serialWrites(&h3.fakeHID)) != 0 {
		t.Fatal("serial written for a device that cannot be told apart")
	}
}

func TestTLSerialSwitchOff(t *testing.T) {
	resetSerialTries()
	SetOptions(Options{NoSerial: true})
	defer SetOptions(Options{})
	h := newFanHID(tlSerial("", 1, 2))
	p := &tlLCD{dev: h, path: "dev-off", brightness: 50}
	p.start()
	if len(serialWrites(&h.fakeHID)) != 0 || strings.Contains(p.serial, "-") {
		t.Fatal("serial written although switched off")
	}
}
