package enram

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/kewe87/casehud/internal/rgb/internal/diag"
	"io"
	"strings"
	"sync"
	"time"
)

// BridgePipe is the named pipe of the CaseHUD SMBus bridge (CaseHUDSensors).
const BridgePipe = `\\.\pipe\CaseHUD.Smbus`

const (
	maxMessage  = 64 << 10
	pipeTimeout = 500 * time.Millisecond
)

type bridgeRequest struct {
	V      int    `json:"v"`
	Addr   byte   `json:"addr"`
	LockMs uint32 `json:"lockMs"`
	Ops    []Op   `json:"ops"`
}

type bridgeResponse struct {
	OK    bool   `json:"ok"`
	Reads []int  `json:"reads"`
	Error string `json:"error"` // "timeout", "denied", "io", ...
}

// PipeTransport sends batches to the CaseHUD bridge service as length-prefixed
// (uint32 LE) JSON messages. It reconnects after errors.
type PipeTransport struct {
	dial func() (io.ReadWriteCloser, error)

	mu   sync.Mutex
	conn io.ReadWriteCloser
}

// PipeName resolves the bridge pipe: name, else BridgePipe. There is no
// environment override in this copy, and only names in the local pipe
// namespace are accepted (anything else falls back to BridgePipe).
func PipeName(name string) string {
	if name == "" || !strings.HasPrefix(name, `\\.\pipe\`) {
		return BridgePipe
	}
	return name
}

// NewPipeTransport connects to the bridge pipe (see PipeName) on first use.
func NewPipeTransport(name string) *PipeTransport {
	name = PipeName(name)
	return NewStreamTransport(func() (io.ReadWriteCloser, error) {
		f, err := openPipe(name)
		if err != nil {
			return nil, err
		}
		// Nothing is sent before the server is known to be the sensor service.
		if err := verifyServer(f); err != nil {
			f.Close()
			return nil, err
		}
		return f, nil
	})
}

// NewStreamTransport uses dial to obtain the connection (tests, other transports).
func NewStreamTransport(dial func() (io.ReadWriteCloser, error)) *PipeTransport {
	return &PipeTransport{dial: dial}
}

// ErrDenied is returned when the bridge refuses the request (disabled, ACL, rules).
var ErrDenied = errors.New("enram: SMBus bridge denied the request")

// ErrUnavailable wraps every failure to reach a trusted bridge (pipe missing or
// busy, server not trusted); discovery stops at the first one.
var ErrUnavailable = errors.New("bridge unavailable")

// Batch implements Transport.
func (p *PipeTransport) Batch(addr byte, lockMs uint32, ops []Op) ([]byte, error) {
	if err := Validate(addr, lockMs, ops); err != nil {
		return nil, err
	}
	req, err := json.Marshal(bridgeRequest{V: 1, Addr: addr, LockMs: lockMs, Ops: ops})
	if err != nil {
		return nil, err
	}

	src := fmt.Sprintf("smbus bridge %#02x", addr)
	diag.Text(src, "out", string(req))
	p.mu.Lock()
	defer p.mu.Unlock()
	var resp *bridgeResponse
	for attempt := 0; ; attempt++ {
		reused := p.conn != nil
		if !reused {
			if p.conn, err = p.dial(); err != nil {
				p.conn = nil
				return nil, fmt.Errorf("enram: bridge: %w: %w", ErrUnavailable, err)
			}
		}
		var answered bool
		resp, answered, err = p.roundTripBounded(req)
		if err == nil {
			break
		}
		p.conn.Close()
		p.conn = nil
		if answered {
			// The batch may have run; drop the frame instead of repeating it.
			return nil, ErrBusy
		}
		// The service closes idle connections: a write error or EOF before the
		// first response byte means nothing was executed. Retry once.
		if !reused || attempt > 0 || !(errors.Is(err, errWrite) || errors.Is(err, io.EOF)) {
			return nil, fmt.Errorf("enram: bridge: %w", err)
		}
	}
	diag.Text(src, "in", fmt.Sprintf("%+v", *resp))
	if !resp.OK {
		switch resp.Error {
		case "timeout":
			return nil, ErrBusy
		case "denied":
			return nil, ErrDenied
		default:
			return nil, &IOError{errors.New(resp.Error)}
		}
	}
	reads := make([]byte, len(resp.Reads))
	for i, v := range resp.Reads {
		reads[i] = byte(v)
	}
	return reads, nil
}

// roundTripBounded runs roundTrip in its own goroutine so that a server that
// accepts the pipe but never answers cannot block the caller, even where
// closing the handle does not interrupt a blocked read.
func (p *PipeTransport) roundTripBounded(req []byte) (*bridgeResponse, bool, error) {
	type result struct {
		resp     *bridgeResponse
		answered bool
		err      error
	}
	conn := p.conn
	ch := make(chan result, 1)
	go func() {
		r, a, e := p.roundTrip(req)
		ch <- result{r, a, e}
	}()
	select {
	case r := <-ch:
		return r.resp, r.answered, r.err
	case <-time.After(pipeTimeout + 300*time.Millisecond):
		conn.Close()
		return nil, false, errors.New("bridge did not answer")
	}
}

// roundTrip writes one request and reads one response; the connection is
// closed by a timer if the service does not answer in time.
var errWrite = errors.New("write failed")

// roundTrip reports answered once the first response byte has arrived; write
// failures wrap errWrite.
func (p *PipeTransport) roundTrip(req []byte) (resp *bridgeResponse, answered bool, err error) {
	conn := p.conn
	timer := time.AfterFunc(pipeTimeout, func() { conn.Close() })
	defer timer.Stop()

	msg := make([]byte, 4+len(req))
	binary.LittleEndian.PutUint32(msg, uint32(len(req)))
	copy(msg[4:], req)
	if _, err := conn.Write(msg); err != nil {
		return nil, false, fmt.Errorf("%w: %w", errWrite, err)
	}
	var hdr [4]byte
	if n, err := io.ReadFull(conn, hdr[:]); err != nil {
		return nil, n > 0, err
	}
	n := binary.LittleEndian.Uint32(hdr[:])
	if n == 0 || n > maxMessage {
		return nil, true, fmt.Errorf("bad message length %d", n)
	}
	body := make([]byte, n)
	if _, err := io.ReadFull(conn, body); err != nil {
		return nil, true, err
	}
	resp = new(bridgeResponse)
	if err := json.Unmarshal(body, resp); err != nil {
		return nil, true, err
	}
	return resp, true, nil
}

// Close drops the connection.
func (p *PipeTransport) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.conn != nil {
		p.conn.Close()
		p.conn = nil
	}
	return nil
}
