package lianlitl

import (
	"errors"
	"image/color"
	"testing"
	"time"

	leds "github.com/kewe87/casehud/internal/rgb"
)

// controller emulates the TL controller: answers A1 with the fan list and
// records every write. stale is queued before the answer (another program's
// reply on the same device).
type controller struct {
	fans   []byte          // A1 entries
	models map[byte]string // A6 model code per address
	stale  [][]byte
	queue  [][]byte
	writes [][]byte
}

func (c *controller) Write(r []byte, _ time.Duration) error {
	c.writes = append(c.writes, append([]byte(nil), r...))
	if r[offCmd] == cmdHandshake {
		ans := make([]byte, reportLen)
		ans[0], ans[offCmd], ans[offLen] = reportID, cmdHandshake, byte(len(c.fans))
		copy(ans[offPayload:], c.fans)
		c.queue = append(c.queue, c.stale...)
		c.queue = append(c.queue, ans)
	} else if r[offCmd] == cmdProductInfo {
		target, addr := r[offPayload], r[offPayload+1]
		m, ok := c.models[addr]
		if target == 0 {
			m, ok = "TL12028-Controller", true
		}
		if ok {
			for part, text := range []string{"N76E,B1,TL,TLA," + m + ",V1.00.62", "2024-05-06 07:08:09"} {
				a := make([]byte, reportLen)
				a[0], a[offCmd] = reportID, cmdProductInfo
				a[offPayload], a[offPayload+1], a[offPayload+2] = target, addr, byte(part+1)
				copy(a[offPayload+3:], text)
				a[offLen] = byte(3 + len(text))
				c.queue = append(c.queue, a)
			}
		}
	} else {
		ack := make([]byte, reportLen)
		ack[0], ack[offCmd] = reportID, r[offCmd]
		c.queue = append(c.queue, ack)
	}
	return nil
}

func (c *controller) Read(buf []byte, timeout time.Duration) (int, error) {
	if len(c.queue) == 0 {
		return 0, errTimeout{}
	}
	n := copy(buf, c.queue[0])
	c.queue = c.queue[1:]
	return n, nil
}

func (c *controller) Close() error { return nil }

type errTimeout struct{}

func (errTimeout) Error() string { return "timeout" }

// port 0 fan 0, port 1 fans 0..2, RPM bytes arbitrary
var sevenFans = []byte{
	0x80, 0x05, 0xDC,
	0x90, 0x05, 0xDC, 0x91, 0x05, 0xDC, 0x92, 0x05, 0xDC,
	0xA0, 0x05, 0xDC, 0xA1, 0x05, 0xDC, 0xA2, 0x05, 0xDC,
	0x30, 0x00, 0x00, // not detected
}

func newTest(t *testing.T) (*device, *controller) {
	c := &controller{fans: sevenFans, stale: [][]byte{packet(0xAA, []byte{1, 2})}}
	d, err := newDevice(c, reportLen)
	if err != nil {
		t.Fatal(err)
	}
	c.writes = nil // discovery (A1, A6) is checked in TestDiscoverZones
	return d, c
}

func configure(t *testing.T, d *device, p leds.Params) {
	full := leds.Params{}
	for _, s := range d.Settings() {
		full[s.Key] = s.Default
	}
	full["allowFlashWrites"] = true // tests of the flash-writing modes
	for k, v := range p {
		full[k] = v
	}
	if err := d.Configure(full); err != nil {
		t.Fatal(err)
	}
}

func lights(c *controller) [][]byte {
	var out [][]byte
	for _, w := range c.writes {
		if w[offCmd] == cmdFanLight {
			out = append(out, w)
		}
	}
	return out
}

func frame(cols ...color.RGBA) map[string][]color.RGBA {
	return map[string][]color.RGBA{"port1": cols[:2], "port2": cols[2:8], "port3": cols[8:14]}
}

func TestDiscoverZones(t *testing.T) {
	c := &controller{fans: sevenFans, stale: [][]byte{packet(0xAA, []byte{1, 2})}}
	d, err := newDevice(c, reportLen)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.writes) != 2 || c.writes[0][offCmd] != cmdHandshake || c.writes[1][offCmd] != cmdProductInfo ||
		c.writes[1][offPayload] != 0 || c.writes[1][offPayload+1] != 0 {
		t.Fatalf("discovery wrote %d reports", len(c.writes))
	}
	if d.Firmware() != "0.62" {
		t.Fatalf("firmware %q", d.Firmware())
	}
	zs := d.Zones()
	if len(zs) != 3 || zs[0].ID != "port1" || zs[0].Count != 2 || zs[1].Count != 6 || zs[2].ID != "port3" || zs[2].Count != 6 {
		t.Fatalf("zones %+v", zs)
	}
}

func TestUpgradingRefused(t *testing.T) {
	c := &controller{fans: []byte{0xC0, 0, 0}}
	if _, err := newDevice(c, reportLen); err == nil {
		t.Fatal("upgrading controller accepted")
	}
	if len(c.writes) != 1 {
		t.Fatal("wrote to an upgrading controller")
	}
}

func writesOf(c *controller, cmd byte) [][]byte {
	var out [][]byte
	for _, w := range c.writes {
		if w[offCmd] == cmd {
			out = append(out, w)
		}
	}
	return out
}

func TestHardwareEffectGroups(t *testing.T) {
	d, c := newTest(t)
	configure(t, d, leds.Params{"mode": "hardware", "effect": "wave", "colors": 2.0, "speed": 3.0, "direction": "counter-clockwise"})
	cols := make([]color.RGBA, 14)
	for i := range cols {
		cols[i] = color.RGBA{9, 9, 9, 255}
	}
	for i := 0; i < 5; i++ { // 3 ports, 2 per frame, then nothing
		d.SetFrame(frame(cols...))
	}
	groups, lightsB0 := writesOf(c, cmdFanGroup), writesOf(c, cmdGroupLite)
	// one A3 per port without sync bit leaves the power-up MB sync
	if len(groups) != 3 || len(lightsB0) != 3 || len(lights(c)) != 3 {
		t.Fatalf("AD %d, B0 %d, A3 %d", len(groups), len(lightsB0), len(lights(c)))
	}
	for _, l := range lights(c) {
		if p := l[offPayload:]; p[0]&1 != 0 || p[2] != modeStatic || p[18] != 0 {
			t.Fatalf("leave sync % x", p[:lightLen])
		}
	}
	// port 1 (0-based): group 8, three fans, both sides
	g := groups[1][offPayload:]
	if groups[1][offLen] != 5 || g[0] != 8 || g[1] != 3 || g[2] != 0xD0 || g[3] != 0xD1 || g[4] != 0xD2 {
		t.Fatalf("grouping % x", groups[1][:12])
	}
	p := lightsB0[1][offPayload:]
	if p[0] != 0 || p[1] != 8 || p[2] != 21 || p[3] != 4 || p[4] != 3 || p[5] != 0x3a || p[6] != 0x8d || p[7] != 0xff ||
		p[8] != 0xff || p[17] != 1 || p[18] != 0 || p[19] != 2 || lightsB0[1][offLen] != lightLen {
		t.Fatalf("B0 payload % x", p[:lightLen])
	}
	for _, w := range c.writes {
		switch w[offCmd] {
		case cmdHandshake, cmdFanGroup, cmdGroupLite, cmdFanLight:
		default:
			t.Fatalf("unexpected command %#02x", w[offCmd])
		}
	}

	// black frame: ports off (static black), groups not re-sent; effect back afterwards
	black := make([]color.RGBA, 14)
	for i := range black {
		black[i].A = 255
	}
	for i := 0; i < 3; i++ {
		d.SetFrame(frame(black...))
	}
	lightsB0 = writesOf(c, cmdGroupLite)
	if len(lightsB0) != 6 || lightsB0[5][offPayload+2] != modeStatic || lightsB0[5][offPayload+5] != 0 || len(writesOf(c, cmdFanGroup)) != 3 {
		t.Fatalf("off: B0 %d", len(lightsB0))
	}
	for i := 0; i < 3; i++ {
		d.SetFrame(frame(cols...))
	}
	if lightsB0 = writesOf(c, cmdGroupLite); len(lightsB0) != 9 || lightsB0[8][offPayload+2] != 21 {
		t.Fatalf("effect not restored: B0 %d", len(lightsB0))
	}
}

func TestCanvasPerRing(t *testing.T) {
	d, c := newTest(t)
	now := time.Unix(0, 0)
	d.now = func() time.Time { return now }
	configure(t, d, leds.Params{"mode": "canvas", "interval": 30.0, "minDelta": 12.0})
	cols := make([]color.RGBA, 14)
	cols[3] = color.RGBA{90, 0, 0, 255} // port 2 (0-based 1), fan 1, bottom ring
	for i := 0; i < 7; i++ {            // 14 rings, 2 per frame
		d.SetFrame(frame(cols...))
	}
	groups, b0 := writesOf(c, cmdFanGroup), writesOf(c, cmdGroupLite)
	if len(groups) != 14 || len(b0) != 14 {
		t.Fatalf("AD %d, B0 %d", len(groups), len(b0))
	}
	// the bottom ring of that fan: group 8+1, member bottom bit | port 1 | fan 0
	var ad, light []byte
	for _, g := range groups {
		if g[offPayload] == 9 {
			ad = g[offPayload:]
		}
	}
	for _, l := range b0 {
		if l[offPayload+1] == 9 {
			light = l[offPayload:]
		}
	}
	if ad == nil || ad[1] != 1 || ad[2] != 0x50 || light == nil || light[5] != 90 || light[19] != 1 {
		t.Fatalf("ring group % x, light % x", ad, light)
	}
	cols[3] = color.RGBA{96, 0, 0, 255} // below minDelta
	now = now.Add(30 * time.Second)
	d.SetFrame(frame(cols...))
	cols[3] = color.RGBA{255, 0, 255, 255}
	d.SetFrame(frame(cols...))
	if n := len(writesOf(c, cmdGroupLite)); n != 15 {
		t.Fatalf("after changes: %d", n)
	}
	cols[3] = color.RGBA{0, 255, 0, 255}
	now = now.Add(29 * time.Second)
	d.SetFrame(frame(cols...))
	if n := len(writesOf(c, cmdGroupLite)); n != 15 || len(writesOf(c, cmdFanGroup)) != 14 {
		t.Fatalf("interval broken: %d", n)
	}
}

func TestFlashWritesNeedConsent(t *testing.T) {
	d, _ := newTest(t)
	for _, mode := range []string{"hardware", "canvas"} {
		err := d.Configure(leds.Params{"mode": mode, "effect": "rainbow"})
		if !errors.Is(err, ErrFlashWrites) {
			t.Fatalf("%s without consent: %v", mode, err)
		}
	}
}

// The default: one A3 with the sync bit per port (first fan) switches the
// ports to the ARGB input, once.
func TestMotherboardSyncDefault(t *testing.T) {
	d, c := newTest(t)
	configure(t, d, leds.Params{})
	cols := make([]color.RGBA, 14)
	for i := 0; i < 8; i++ {
		d.SetFrame(frame(cols...))
	}
	ls := lights(c)
	if len(ls) != 3 || len(c.writes) != 3 {
		t.Fatalf("%d A3, %d writes", len(ls), len(c.writes))
	}
	for i, port := range d.ports {
		p := ls[i][offPayload:]
		if p[0] != byte(port<<4|1) || p[1] != byte(port<<4) {
			t.Fatalf("port %d payload % x", port, p[:lightLen])
		}
		for _, b := range p[2:lightLen] {
			if b != 0 {
				t.Fatalf("sync packet not empty: % x", p[:lightLen])
			}
		}
	}
}

func TestPassive(t *testing.T) {
	d, c := newTest(t)
	configure(t, d, leds.Params{"mode": "passive"})
	cols := make([]color.RGBA, 14)
	cols[0] = color.RGBA{200, 0, 0, 255}
	for i := 0; i < 5; i++ {
		d.SetFrame(frame(cols...))
	}
	if len(c.writes) != 0 {
		t.Fatalf("passive mode wrote %d reports", len(c.writes))
	}
}

func TestSetupGroups(t *testing.T) {
	d, c := newTest(t)
	if err := d.SetupGroups(2); err != nil {
		t.Fatal(err)
	}
	g := writesOf(c, cmdFanGroup)
	if len(g) != 1 || g[0][offPayload] != 16 || g[0][offPayload+1] != 3 || g[0][offPayload+2] != 0xE0 {
		t.Fatalf("grouping % x", g[0][:12])
	}
	if err := d.SetupGroups(3); err == nil {
		t.Fatal("empty port accepted")
	}
}

func TestAcksConsumed(t *testing.T) {
	d, c := newTest(t)
	configure(t, d, leds.Params{"mode": "hardware"})
	cols := make([]color.RGBA, 14)
	cols[0] = color.RGBA{1, 1, 1, 255}
	for i := 0; i < 4; i++ {
		d.SetFrame(frame(cols...))
	}
	if len(c.queue) != 0 {
		t.Fatalf("%d unread answers left", len(c.queue))
	}
}

func TestMergedEffectOrder(t *testing.T) {
	d, c := newTest(t)
	configure(t, d, leds.Params{"mode": "hardware", "merge": true, "portOrder": "3,1"})
	cols := make([]color.RGBA, 14)
	cols[0] = color.RGBA{1, 1, 1, 255}
	for i := 0; i < 3; i++ {
		d.SetFrame(frame(cols...))
	}
	g, l := writesOf(c, cmdFanGroup), writesOf(c, cmdGroupLite)
	if len(g) != 1 || len(l) != 1 {
		t.Fatalf("AD %d, B0 %d", len(g), len(l))
	}
	// port 3 (0-based 2) first, then port 1 (0), then the rest (port 2)
	want := []byte{0, 7, 0xE0, 0xE1, 0xE2, 0xC0, 0xD0, 0xD1, 0xD2}
	got := g[0][offPayload : offPayload+len(want)]
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("merged group % x, want % x", got, want)
		}
	}
	if l[0][offPayload+1] != 0 || l[0][offPayload+2] != 1 {
		t.Fatalf("B0 % x", l[0][offPayload:offPayload+8])
	}
	if err := d.Configure(leds.Params{"mode": "hardware", "allowFlashWrites": true, "effect": "rainbow", "portOrder": "9"}); err == nil {
		t.Fatal("unknown port accepted")
	}
}

func TestIdentify(t *testing.T) {
	d, c := newTest(t)
	if err := d.TestLight(1, 2, true); err != nil {
		t.Fatal(err)
	}
	if err := d.TestLight(2, -1, false); err != nil {
		t.Fatal(err)
	}
	if err := d.BlinkPort(0); err != nil {
		t.Fatal(err)
	}
	tl := writesOf(c, cmdTestLight)
	if len(tl) != 2 || tl[0][offPayload] != 1 || tl[0][offPayload+1] != 0x12 || tl[0][offPayload+2] != 1 ||
		tl[1][offPayload] != 3 || tl[1][offPayload+1] != 2 {
		t.Fatalf("test light % x / % x", tl[0][:9], tl[1][:9])
	}
	if b := writesOf(c, cmdBlinkPort); len(b) != 1 || b[0][offPayload] != 0 {
		t.Fatal("blink")
	}
	if d.TestLight(1, 5, false) == nil || d.BlinkPort(3) == nil {
		t.Fatal("unknown fan or port accepted")
	}
}

func TestTopology(t *testing.T) {
	d, c := newTest(t)
	c.models = map[byte]string{0x00: "TL1201", 0x10: "TLC1201", 0x11: "TLC1201", 0x12: "TLFC1401", 0x20: "TLF1201"}
	top, err := d.Topology()
	if err != nil {
		t.Fatal(err)
	}
	if len(top.Fans) != 7 {
		t.Fatalf("%d fans", len(top.Fans))
	}
	f := top.Fans[3] // port 2, fan 3
	if f.Port != 2 || f.Index != 3 || !f.LCD || !f.Reverse || f.SizeMM != 140 || f.RPM != 0x05DC {
		t.Fatalf("fan %+v", f)
	}
	if top.Fans[0].LCD || top.Fans[0].SizeMM != 120 || top.Fans[4].LCD || !top.Fans[4].Reverse {
		t.Fatalf("fans %+v", top.Fans)
	}
	if top.Fans[6].Model != "" { // no answer: unknown, not an error
		t.Fatalf("fan 7 %+v", top.Fans[6])
	}
	for _, w := range c.writes {
		switch w[offCmd] {
		case cmdHandshake, cmdProductInfo:
		default:
			t.Fatalf("topology wrote command %#02x", w[offCmd])
		}
	}
	bad := Topology{Fans: []FanInfo{{Port: 1, Index: 1, LCD: true}, {Port: 2, LCD: true}, {Port: 2, LCD: true}, {Port: 2, LCD: true}, {Port: 2, LCD: true}}}
	if w := bad.Warnings(); len(w) != 2 {
		t.Fatalf("warnings %v", w)
	}
}

func TestShutdownAllPorts(t *testing.T) {
	d, c := newTest(t)
	configure(t, d, leds.Params{"mode": "canvas"})
	cols := make([]color.RGBA, 14)
	for i := range cols {
		cols[i] = color.RGBA{50, 50, 50, 255}
	}
	for i := 0; i < 7; i++ {
		d.SetFrame(frame(cols...))
	}
	before := len(writesOf(c, cmdGroupLite))
	black := make([]color.RGBA, 14)
	for i := range black {
		black[i].A = 255
	}
	d.SetFrame(frame(black...)) // one shutdown frame reaches all 14 rings
	if n := len(writesOf(c, cmdGroupLite)) - before; n != 14 {
		t.Fatalf("shutdown frame reached %d rings", n)
	}
	for _, l := range lights(c) {
		if l[offPayload]&1 != 0 {
			t.Fatal("shutdown switched fans back to mainboard sync")
		}
	}
}

func TestFirmwareVersion(t *testing.T) {
	for in, want := range map[string]string{"V1.00.62": "0.62", " V1.01.05 ": "1.5", "V2": "2", "": ""} {
		if got := firmwareVersion(in); got != want {
			t.Errorf("firmwareVersion(%q) = %q, want %q", in, got, want)
		}
	}
}
