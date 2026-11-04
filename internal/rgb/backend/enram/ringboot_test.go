package enram

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net"
	"reflect"
	"testing"
)

// fakeRingBootServer mimics the service side for setRingBoot requests: it
// records the raw JSON object and answers with the given response.
func fakeRingBootServer(conn net.Conn, got chan<- map[string]any, resp bridgeResponse) {
	defer conn.Close()
	for {
		var hdr [4]byte
		if _, err := io.ReadFull(conn, hdr[:]); err != nil {
			return
		}
		body := make([]byte, binary.LittleEndian.Uint32(hdr[:]))
		if _, err := io.ReadFull(conn, body); err != nil {
			return
		}
		var m map[string]any
		if err := json.Unmarshal(body, &m); err != nil {
			return
		}
		got <- m
		out, _ := json.Marshal(resp)
		msg := binary.LittleEndian.AppendUint32(nil, uint32(len(out)))
		if _, err := conn.Write(append(msg, out...)); err != nil {
			return
		}
	}
}

func ringBootTransport(resp bridgeResponse, got chan map[string]any) (*PipeTransport, *int) {
	dials := new(int)
	return NewStreamTransport(func() (io.ReadWriteCloser, error) {
		*dials++
		client, server := net.Pipe()
		go fakeRingBootServer(server, got, resp)
		return client, nil
	}), dials
}

func TestSetRingBootStateWire(t *testing.T) {
	cases := []struct {
		mode    string
		r, g, b uint8
		want    map[string]any
	}{
		{RingBootColor, 10, 20, 30, map[string]any{"v": 1.0, "type": "setRingBoot", "mode": "color", "r": 10.0, "g": 20.0, "b": 30.0}},
		{RingBootOff, 9, 9, 9, map[string]any{"v": 1.0, "type": "setRingBoot", "mode": "off", "r": 0.0, "g": 0.0, "b": 0.0}},
		{RingBootNone, 1, 2, 3, map[string]any{"v": 1.0, "type": "setRingBoot", "mode": "none", "r": 0.0, "g": 0.0, "b": 0.0}},
	}
	for _, c := range cases {
		got := make(chan map[string]any, 2)
		pt, _ := ringBootTransport(bridgeResponse{OK: true}, got)
		if err := pt.SetRingBootState(c.mode, c.r, c.g, c.b); err != nil {
			t.Fatalf("%s: %v", c.mode, err)
		}
		pt.Close()
		if m := <-got; !reflect.DeepEqual(m, c.want) {
			t.Fatalf("%s: sent %v, want %v", c.mode, m, c.want)
		}
	}
}

func TestSetRingBootStateRejectsBadMode(t *testing.T) {
	got := make(chan map[string]any, 1)
	pt, dials := ringBootTransport(bridgeResponse{OK: true}, got)
	defer pt.Close()
	for _, m := range []string{"", "rainbow", "OFF", "color "} {
		if err := pt.SetRingBootState(m, 0, 0, 0); err == nil {
			t.Fatalf("mode %q accepted", m)
		}
	}
	if *dials != 0 {
		t.Fatal("a refused request must not reach the pipe")
	}
}

func TestSetRingBootStateErrors(t *testing.T) {
	for _, c := range []struct {
		resp bridgeResponse
		want error
	}{
		{bridgeResponse{Error: "denied"}, ErrDenied},
		{bridgeResponse{Error: "timeout"}, ErrBusy},
	} {
		pt, _ := ringBootTransport(c.resp, make(chan map[string]any, 1))
		if err := pt.SetRingBootState(RingBootOff, 0, 0, 0); !errors.Is(err, c.want) {
			t.Fatalf("%v: got %v", c.resp.Error, err)
		}
		pt.Close()
	}
	pt, _ := ringBootTransport(bridgeResponse{Error: "io"}, make(chan map[string]any, 1))
	defer pt.Close()
	var ioErr *IOError
	if err := pt.SetRingBootState(RingBootOff, 0, 0, 0); !errors.As(err, &ioErr) {
		t.Fatalf("io: got %v", err)
	}
}

func TestSetRingBootStateUnavailable(t *testing.T) {
	pt := NewStreamTransport(func() (io.ReadWriteCloser, error) { return nil, ErrUntrusted })
	if err := pt.SetRingBootState(RingBootOff, 0, 0, 0); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("got %v", err)
	}
}

// The service closes idle connections; the next call reconnects.
func TestSetRingBootStateReconnects(t *testing.T) {
	got := make(chan map[string]any, 4)
	var servers []net.Conn
	pt := NewStreamTransport(func() (io.ReadWriteCloser, error) {
		client, server := net.Pipe()
		servers = append(servers, server)
		go fakeRingBootServer(server, got, bridgeResponse{OK: true})
		return client, nil
	})
	defer pt.Close()
	if err := pt.SetRingBootState(RingBootOff, 0, 0, 0); err != nil {
		t.Fatal(err)
	}
	servers[0].Close()
	if err := pt.SetRingBootState(RingBootColor, 1, 2, 3); err != nil {
		t.Fatal(err)
	}
	if len(servers) != 2 {
		t.Fatalf("%d connections", len(servers))
	}
}

func TestRingPipeName(t *testing.T) {
	if RingPipe != `\\.\pipe\CaseHUD.Ring` || RingPipe == BridgePipe || PipeName(RingPipe) != RingPipe {
		t.Fatalf("ring pipe name %q", RingPipe)
	}
}
