// Package diag keeps a bounded in-memory record for diagnostic reports: log
// lines (always) and, while tracing is on, the raw packets exchanged with the
// devices. Nothing is written to disk.
package diag

import (
	"encoding/hex"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Entry is one log line or packet.
type Entry struct {
	Time   time.Time `json:"time"`
	Source string    `json:"source"`         // e.g. "engine", "hid 0b05:1aa6", "smbus 0x71"
	Kind   string    `json:"kind"`           // "log", "out", "in", "error"
	Text   string    `json:"text,omitempty"` // log message or error
	Hex    string    `json:"hex,omitempty"`  // packet bytes
}

const (
	maxLog     = 1000
	maxPackets = 4000
	maxHex     = 600 // bytes per packet kept (large frames are cut)
)

type ring struct {
	mu      sync.Mutex
	entries []Entry
	next    int
	full    bool
	size    int
}

func (r *ring) add(e Entry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.entries == nil {
		r.entries = make([]Entry, r.size)
	}
	r.entries[r.next] = e
	r.next = (r.next + 1) % r.size
	if r.next == 0 {
		r.full = true
	}
}

func (r *ring) snapshot() []Entry {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.full {
		return append([]Entry(nil), r.entries[:r.next]...)
	}
	return append(append([]Entry(nil), r.entries[r.next:]...), r.entries[:r.next]...)
}

func (r *ring) reset() {
	r.mu.Lock()
	r.entries, r.next, r.full = nil, 0, false
	r.mu.Unlock()
}

var (
	logs    = &ring{size: maxLog}
	packets = &ring{size: maxPackets}
	tracing atomic.Bool
)

// SetTracing switches the packet record on or off; switching it on clears
// older packets.
func SetTracing(on bool) {
	if on && !tracing.Load() {
		packets.reset()
	}
	tracing.Store(on)
}

// Tracing reports whether packets are recorded.
func Tracing() bool { return tracing.Load() }

// Logf records a log line.
func Logf(source, format string, args ...any) {
	logs.add(Entry{Time: time.Now(), Source: source, Kind: "log", Text: fmt.Sprintf(format, args...)})
}

// Packet records bytes sent ("out") or received ("in") while tracing is on.
func Packet(source, dir string, data []byte) {
	if !tracing.Load() {
		return
	}
	cut := data
	if len(cut) > maxHex {
		cut = cut[:maxHex]
	}
	e := Entry{Time: time.Now(), Source: source, Kind: dir, Hex: hex.EncodeToString(cut)}
	if len(data) > maxHex {
		e.Text = fmt.Sprintf("%d bytes, first %d shown", len(data), maxHex)
	}
	packets.add(e)
}

// Text records a non-binary exchange (e.g. a JSON request) while tracing is on.
func Text(source, dir, text string) {
	if !tracing.Load() {
		return
	}
	if len(text) > 4*maxHex {
		text = text[:4*maxHex] + "…"
	}
	packets.add(Entry{Time: time.Now(), Source: source, Kind: dir, Text: text})
}

// Error records a failed transfer while tracing is on.
func Error(source string, err error) {
	if err == nil || !tracing.Load() {
		return
	}
	packets.add(Entry{Time: time.Now(), Source: source, Kind: "error", Text: err.Error()})
}

// Logs returns the recorded log lines, oldest first.
func Logs() []Entry { return logs.snapshot() }

// Packets returns the recorded packets, oldest first.
func Packets() []Entry { return packets.snapshot() }
