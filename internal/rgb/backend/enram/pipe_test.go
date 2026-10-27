package enram

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"image/color"
	"io"
	"net"
	"testing"
)

// fakeBridge serves the CaseHUD SMBus bridge protocol on one connection,
// enforcing the same rules as the real service.
func fakeBridge(t *testing.T, conn net.Conn, bus Bus, busy func() bool) {
	defer conn.Close()
	exec := NewBusTransport(bus)
	for {
		var hdr [4]byte
		if _, err := io.ReadFull(conn, hdr[:]); err != nil {
			return
		}
		body := make([]byte, binary.LittleEndian.Uint32(hdr[:]))
		if _, err := io.ReadFull(conn, body); err != nil {
			return
		}
		var req bridgeRequest
		resp := bridgeResponse{OK: true}
		if err := json.Unmarshal(body, &req); err != nil || req.V != 1 {
			resp = bridgeResponse{Error: "denied"}
		} else if err := Validate(req.Addr, req.LockMs, req.Ops); err != nil {
			resp = bridgeResponse{Error: "denied"}
		} else if busy() {
			resp = bridgeResponse{Error: "timeout"}
		} else if reads, err := exec.Batch(req.Addr, req.LockMs, req.Ops); err != nil {
			resp = bridgeResponse{Error: "io"}
		} else {
			for _, r := range reads {
				resp.Reads = append(resp.Reads, int(r))
			}
		}
		out, _ := json.Marshal(resp)
		msg := binary.LittleEndian.AppendUint32(nil, uint32(len(out)))
		if _, err := conn.Write(append(msg, out...)); err != nil {
			return
		}
	}
}

func TestPipeTransport(t *testing.T) {
	c := &chip{sticks: map[byte]*eneState{0x71: newENE("AUDA0-E6K5-0101", 8)}, touched: map[byte]bool{}}
	busy := false
	dials := 0
	pt := NewStreamTransport(func() (io.ReadWriteCloser, error) {
		dials++
		client, server := net.Pipe()
		go fakeBridge(t, server, c, func() bool { return busy })
		return client, nil
	})
	defer pt.Close()

	devs, err := NewWithTransport(pt).Discover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(devs) != 1 || devs[0].ID() != "enram:0b00:71" {
		t.Fatalf("devices %v", devs)
	}

	cols := make([]color.RGBA, 8)
	cols[0] = color.RGBA{1, 2, 3, 255}
	if err := devs[0].SetFrame(map[string][]color.RGBA{"leds": cols}); err != nil {
		t.Fatal(err)
	}
	s := c.sticks[0x71]
	if s.regs[regDirectMode] != 1 || s.regs[0x8100] != 1 || s.regs[regApply] != 1 {
		t.Fatal("frame not applied through the bridge")
	}

	// bus busy: the frame is dropped silently
	busy = true
	cols[0] = color.RGBA{9, 9, 9, 255}
	if err := devs[0].SetFrame(map[string][]color.RGBA{"leds": cols}); err != nil {
		t.Fatalf("busy frame: %v", err)
	}
	if s.regs[0x8100] != 1 {
		t.Fatal("busy frame was written")
	}
	busy = false

	// the client refuses forbidden batches before sending
	if _, err := pt.Batch(0x50, 10, []Op{{Op: OpReadReg}}); err == nil {
		t.Fatal("SPD address sent")
	}
	if _, err := pt.Batch(0x71, 10, []Op{{Op: OpReadCmd, Cmd: 0x10}}); err == nil {
		t.Fatal("raw command 0x10 sent")
	}
	if dials != 1 {
		t.Fatalf("dialed %d times", dials)
	}
}

func TestPipeReconnect(t *testing.T) {
	c := &chip{sticks: map[byte]*eneState{0x71: newENE("AUDA0-E6K5-0101", 8)}, touched: map[byte]bool{}}
	dials := 0
	pt := NewStreamTransport(func() (io.ReadWriteCloser, error) {
		dials++
		if dials == 1 {
			return nil, errors.New("service not running")
		}
		client, server := net.Pipe()
		go fakeBridge(t, server, c, func() bool { return false })
		return client, nil
	})
	if _, err := pt.Batch(0x71, 10, []Op{{Op: OpReadCmd, Cmd: 0xA0}}); err == nil {
		t.Fatal("expected dial error")
	}
	r, err := pt.Batch(0x71, 10, []Op{{Op: OpReadCmd, Cmd: 0xA3}})
	if err != nil || len(r) != 1 || r[0] != 3 {
		t.Fatalf("reconnect: %v %v", r, err)
	}
}

func TestPipeName(t *testing.T) {
	t.Setenv("CASEHUD_SMBUS_PIPE", `\\.\pipe\dev`)
	if PipeName("") != BridgePipe {
		t.Fatal("the environment must not redirect the bridge")
	}
	if PipeName(`\\.\pipe\x`) != `\\.\pipe\x` || PipeName(`C:\temp\x`) != BridgePipe {
		t.Fatal("only local pipe names are taken")
	}
}

func TestValidateLockMs(t *testing.T) {
	ops := []Op{{Op: OpReadCmd, Cmd: 0xA0}}
	for _, ms := range []uint32{0, 51} {
		if Validate(0x71, ms, ops) == nil {
			t.Fatalf("lockMs %d accepted", ms)
		}
	}
	if err := Validate(0x71, 1, ops); err != nil {
		t.Fatal(err)
	}
}

// The service closes idle connections; the next batch reconnects transparently.
func TestPipeIdleClose(t *testing.T) {
	c := &chip{sticks: map[byte]*eneState{0x71: newENE("AUDA0-E6K5-0101", 8)}, touched: map[byte]bool{}}
	var servers []net.Conn
	pt := NewStreamTransport(func() (io.ReadWriteCloser, error) {
		client, server := net.Pipe()
		servers = append(servers, server)
		go fakeBridge(t, server, c, func() bool { return false })
		return client, nil
	})
	defer pt.Close()
	if _, err := pt.Batch(0x71, 10, []Op{{Op: OpReadCmd, Cmd: 0xA1}}); err != nil {
		t.Fatal(err)
	}
	servers[0].Close() // idle timeout on the service side
	r, err := pt.Batch(0x71, 10, []Op{{Op: OpReadCmd, Cmd: 0xA2}})
	if err != nil || len(r) != 1 || r[0] != 2 {
		t.Fatalf("after idle close: %v %v", r, err)
	}
	if len(servers) != 2 {
		t.Fatalf("%d connections", len(servers))
	}
}

// A connection that breaks after the response started may have run the batch:
// the frame is dropped (ErrBusy), not repeated.
func TestPipeBrokenAnswerNotRetried(t *testing.T) {
	dials := 0
	pt := NewStreamTransport(func() (io.ReadWriteCloser, error) {
		dials++
		client, server := net.Pipe()
		go func() {
			var hdr [4]byte
			io.ReadFull(server, hdr[:])
			io.ReadFull(server, make([]byte, binary.LittleEndian.Uint32(hdr[:])))
			server.Write([]byte{0x10, 0}) // half a length prefix
			server.Close()
		}()
		return client, nil
	})
	if _, err := pt.Batch(0x71, 10, []Op{{Op: OpReadCmd, Cmd: 0xA0}}); !errors.Is(err, ErrBusy) {
		t.Fatalf("got %v", err)
	}
	if dials != 1 {
		t.Fatalf("retried: %d dials", dials)
	}
}

func TestTrustedServerImage(t *testing.T) {
	dir := `C:\Program Files\CaseHUD`
	for image, want := range map[string]bool{
		`C:\Program Files\CaseHUD\sensors\CaseHUD.Sensors.exe`: true,
		`c:\program files\casehud\SENSORS\casehud.sensors.exe`: true,
		`C:\Users\x\AppData\Local\Temp\CaseHUD.Sensors.exe`:    false,
		`C:\Program Files\CaseHUD\CaseHUD.exe`:                 false,
		`C:\Program Files\CaseHUD\sensors\other.exe`:           false,
		``: false,
	} {
		if got := trustedServerImage(image, dir); got != want {
			t.Errorf("trustedServerImage(%q) = %v", image, got)
		}
	}
	if trustedServerImage(`C:\a\sensors\CaseHUD.Sensors.exe`, "") {
		t.Fatal("empty install folder trusted")
	}
}

// Discovery stops at the first unreachable or untrusted bridge: one dial, no
// further per-address opens.
func TestDiscoverStopsWhenBridgeUnavailable(t *testing.T) {
	for _, derr := range []error{ErrUntrusted, errors.New("All pipe instances are busy.")} {
		dials := 0
		pt := NewStreamTransport(func() (io.ReadWriteCloser, error) {
			dials++
			return nil, derr
		})
		b := NewWithTransport(pt)
		_, err := b.Discover(context.Background())
		if err == nil || dials != 1 {
			t.Fatalf("%v: err=%v dials=%d", derr, err, dials)
		}
	}
}
