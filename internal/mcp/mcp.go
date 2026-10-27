// Package mcp is CaseHUD's optional Model Context Protocol server: AI
// assistants (desktop assistants, editors, ...) start "casehud-cli mcp" and can
// then read sensors, design layouts, preview them and put them on a display.
// It talks to the running CaseHUD app over its local API and only works
// while the user has enabled it in the settings.
package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
)

// ProtocolVersion is the newest MCP revision this server speaks.
const ProtocolVersion = "2025-06-18"

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Content is one item of a tool result.
type Content struct {
	Type     string `json:"type"` // "text" or "image"
	Text     string `json:"text,omitempty"`
	Data     string `json:"data,omitempty"` // base64 for images
	MimeType string `json:"mimeType,omitempty"`
}

// ToolResult is what a tool call returns.
type ToolResult struct {
	Content []Content `json:"content"`
	IsError bool      `json:"isError,omitempty"`
}

// Tool is a callable tool.
type Tool struct {
	Name        string                                         `json:"name"`
	Title       string                                         `json:"title,omitempty"`
	Description string                                         `json:"description"`
	InputSchema map[string]any                                 `json:"inputSchema"`
	Handler     func(args json.RawMessage) (ToolResult, error) `json:"-"`
}

// Server dispatches JSON-RPC messages to tools.
type Server struct {
	Name, Version string
	Instructions  string
	Tools         []Tool
}

// Serve handles newline-delimited JSON-RPC on r/w (the stdio transport)
// until r ends.
func (s *Server) Serve(r io.Reader, w io.Writer) error {
	in := bufio.NewScanner(r)
	in.Buffer(make([]byte, 0, 1<<20), 16<<20)
	enc := json.NewEncoder(w)
	for in.Scan() {
		line := in.Bytes()
		if len(line) == 0 {
			continue
		}
		var req request
		if err := json.Unmarshal(line, &req); err != nil {
			enc.Encode(response{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &rpcError{-32700, "parse error"}})
			continue
		}
		if len(req.ID) == 0 {
			continue // notification (e.g. notifications/initialized)
		}
		res, rerr := s.handle(req)
		out := response{JSONRPC: "2.0", ID: req.ID, Result: res, Error: rerr}
		if err := enc.Encode(out); err != nil {
			return err
		}
	}
	return in.Err()
}

func (s *Server) handle(req request) (any, *rpcError) {
	switch req.Method {
	case "initialize":
		var p struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		json.Unmarshal(req.Params, &p)
		v := ProtocolVersion
		switch p.ProtocolVersion {
		case "2024-11-05", "2025-03-26", "2025-06-18":
			v = p.ProtocolVersion
		}
		return map[string]any{
			"protocolVersion": v,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": s.Name, "version": s.Version},
			"instructions":    s.Instructions,
		}, nil
	case "ping":
		return map[string]any{}, nil
	case "tools/list":
		return map[string]any{"tools": s.Tools}, nil
	case "tools/call":
		var p struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return nil, &rpcError{-32602, "invalid params"}
		}
		for _, t := range s.Tools {
			if t.Name == p.Name {
				if len(p.Arguments) == 0 {
					p.Arguments = json.RawMessage("{}")
				}
				res, err := t.Handler(p.Arguments)
				if err != nil {
					log.Printf("mcp: %s: %v", p.Name, err)
					return ToolResult{Content: []Content{{Type: "text", Text: err.Error()}}, IsError: true}, nil
				}
				return res, nil
			}
		}
		return nil, &rpcError{-32602, fmt.Sprintf("unknown tool %q", p.Name)}
	}
	return nil, &rpcError{-32601, "method not found: " + req.Method}
}

// Text is a one-item text result.
func Text(s string) ToolResult { return ToolResult{Content: []Content{{Type: "text", Text: s}}} }
