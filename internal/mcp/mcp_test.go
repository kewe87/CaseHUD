package mcp

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestServe(t *testing.T) {
	s := &Server{Name: "t", Version: "1", Tools: []Tool{{
		Name: "echo", InputSchema: map[string]any{"type": "object"},
		Handler: func(a json.RawMessage) (ToolResult, error) { return Text(string(a)), nil },
	}}}
	in := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26"}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"echo","arguments":{"x":1}}}`,
		`{"jsonrpc":"2.0","id":3,"method":"nope"}`,
	}, "\n")
	var out bytes.Buffer
	if err := s.Serve(strings.NewReader(in), &out); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("want 3 responses (no reply to notifications), got %d:\n%s", len(lines), out.String())
	}
	if !strings.Contains(lines[0], `"protocolVersion":"2025-03-26"`) {
		t.Errorf("initialize should echo a supported version: %s", lines[0])
	}
	if !strings.Contains(lines[1], `{\"x\":1}`) {
		t.Errorf("tool result: %s", lines[1])
	}
	if !strings.Contains(lines[2], `-32601`) {
		t.Errorf("unknown method: %s", lines[2])
	}
}

func TestNewAPI(t *testing.T) {
	a, err := NewAPI("http://127.0.0.1:1234/?t=abc\n")
	if err != nil || a.Base != "http://127.0.0.1:1234" || a.Token != "abc" {
		t.Fatalf("NewAPI = %+v, %v", a, err)
	}
	if _, err := NewAPI("garbage"); err == nil {
		t.Fatal("invalid instance URL accepted")
	}
}
