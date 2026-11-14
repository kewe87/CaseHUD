package lianlitl

import (
	"image/color"
	"maps"
	"testing"

	"github.com/kewe87/casehud/internal/cooling"
	leds "github.com/kewe87/casehud/internal/rgb"
)

var _ cooling.Hardware = (*device)(nil)

func TestFanIDs(t *testing.T) {
	d, _ := newTest(t)
	want := []string{"port1-fan1", "port2-fan1", "port2-fan2", "port2-fan3", "port3-fan1", "port3-fan2", "port3-fan3"}
	got := d.FanIDs()
	if len(got) != len(want) {
		t.Fatalf("%v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%v", got)
		}
	}
}

func TestSetDutyAddressesOneFan(t *testing.T) {
	d, c := newTest(t)
	cases := []struct {
		percent int
		wire    byte
	}{{0, 1}, {1, 12}, {11, 12}, {12, 12}, {55, 55}, {100, 100}}
	for _, tc := range cases {
		c.writes = nil
		if err := d.SetDuty("port2-fan3", tc.percent); err != nil {
			t.Fatal(err)
		}
		w := writesOf(c, cmdFanSpeed)
		if len(w) != 1 || len(c.writes) != 1 {
			t.Fatalf("%d%%: %d writes", tc.percent, len(c.writes))
		}
		if w[0][offLen] != 2 || w[0][offPayload] != 0x12 || w[0][offPayload+1] != tc.wire {
			t.Fatalf("%d%%: % x", tc.percent, w[0][:10])
		}
	}
}

func TestFanCommandsRefuseBadInput(t *testing.T) {
	d, c := newTest(t)
	for _, percent := range []int{-1, 101} {
		if err := d.SetDuty("port1-fan1", percent); err == nil {
			t.Fatalf("duty %d accepted", percent)
		}
	}
	if err := d.SetDuty("port4-fan1", 50); err == nil {
		t.Fatal("unknown fan accepted")
	}
	if err := d.TakeControl("port1-fan2"); err == nil {
		t.Fatal("unknown fan accepted")
	}
	if len(c.writes) != 0 {
		t.Fatalf("%d writes", len(c.writes))
	}
}

func TestSpeedSource(t *testing.T) {
	d, c := newTest(t)
	if err := d.TakeControl("port3-fan2"); err != nil {
		t.Fatal(err)
	}
	if err := d.ReleaseControl("port3-fan2"); err != nil {
		t.Fatal(err)
	}
	w := writesOf(c, cmdSpeedSync)
	if len(w) != 2 || w[0][offLen] != 1 || w[0][offPayload] != 0x21 || w[1][offPayload] != 0xA1 {
		t.Fatalf("%d writes: % x", len(w), w)
	}
}

func TestSpeeds(t *testing.T) {
	c := &controller{fans: []byte{0x80, 0x01, 0xE0, 0x91, 0x07, 0x8C, 0x20, 0x09, 0x99}}
	d, err := newDevice(c, reportLen)
	if err != nil {
		t.Fatal(err)
	}
	got, err := d.Speeds()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got["port1-fan1"] != 480 || got["port2-fan2"] != 1932 {
		t.Fatalf("%v", got)
	}
}

// The controller pads its answer with empty slots, whose address reads as the
// first fan of the first port.
func TestSpeedsIgnoreEmptySlots(t *testing.T) {
	c := &controller{fans: append([]byte{
		0x80, 0x01, 0xA4, 0x81, 0x01, 0xA4, 0x82, 0x01, 0xA4, 0x90, 0x05, 0x28,
		0xA0, 0x01, 0xE0, 0xA1, 0x02, 0x1C, 0xA2, 0x02, 0x1C,
	}, make([]byte, 9*3)...)}
	d, err := newDevice(c, reportLen)
	if err != nil {
		t.Fatal(err)
	}
	got, err := d.Speeds()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]int{
		"port1-fan1": 420, "port1-fan2": 420, "port1-fan3": 420, "port2-fan1": 1320,
		"port3-fan1": 480, "port3-fan2": 540, "port3-fan3": 540,
	}
	if !maps.Equal(got, want) {
		t.Fatalf("got %v", got)
	}
}

func TestHandlesOnOneControllerShareALock(t *testing.T) {
	mu := pathLock("a")
	if pathLock("a") != mu || pathLock("b") == mu {
		t.Fatal("wrong lock")
	}
	d, err := newDeviceShared(&controller{fans: []byte{0x80, 0x01, 0xE0}}, reportLen, mu)
	if err != nil {
		t.Fatal(err)
	}
	if d.mu != mu {
		t.Fatal("device has its own lock")
	}
	if !mu.TryLock() {
		t.Fatal("still locked after opening")
	}
	mu.Unlock()
}

func TestSpeedsFailDuringFirmwareUpdate(t *testing.T) {
	d, c := newTest(t)
	c.fans = []byte{0xC0, 0, 0}
	if _, err := d.Speeds(); err == nil {
		t.Fatal("no error during firmware update")
	}
}

func TestSetDutySentOnceWithoutAnswer(t *testing.T) {
	d, c := newTest(t)
	if err := d.SetDuty("port1-fan1", 50); err != nil {
		t.Fatal(err)
	}
	if n := len(writesOf(c, cmdFanSpeed)); n != 1 {
		t.Fatalf("%d writes", n)
	}
	c.writeErr = errTimeout{}
	if err := d.SetDuty("port1-fan1", 60); err == nil {
		t.Fatal("write error not reported")
	}
}

func TestTakeControlNeedsAnswer(t *testing.T) {
	d, c := newTest(t)
	c.dropAcks = true
	if err := d.TakeControl("port1-fan1"); err == nil {
		t.Fatal("missing acknowledgement not reported")
	}
}

func TestLightingNeverSendsSpeedCommands(t *testing.T) {
	for _, mode := range []string{"motherboard", "passive", "hardware", "canvas"} {
		d, c := newTest(t)
		configure(t, d, leds.Params{"mode": mode})
		cols := make([]color.RGBA, 14)
		for i := range cols {
			cols[i] = color.RGBA{uint8(i * 10), 40, 80, 255}
		}
		for i := 0; i < 8; i++ {
			if err := d.SetFrame(frame(cols...)); err != nil {
				t.Fatal(err)
			}
		}
		if err := d.Close(); err != nil {
			t.Fatal(err)
		}
		if n := len(writesOf(c, cmdFanSpeed)) + len(writesOf(c, cmdSpeedSync)); n != 0 {
			t.Fatalf("%s: %d speed commands", mode, n)
		}
	}
}

func TestSendGetsPastAFullQueueWithOneWrite(t *testing.T) {
	c := &controller{fans: []byte{0x80, 0x01, 0xE0}}
	d, err := newDevice(c, reportLen)
	if err != nil {
		t.Fatal(err)
	}
	for range 32 { // what an idle handle collects from the status queries of another one
		a := make([]byte, reportLen)
		a[0], a[offCmd] = reportID, cmdHandshake
		c.queue = append(c.queue, a)
	}
	c.writes = nil
	if err := d.send(cmdBlinkPort, []byte{0}); err != nil {
		t.Fatal(err)
	}
	if len(c.writes) != 1 {
		t.Fatalf("%d writes", len(c.writes))
	}
}
