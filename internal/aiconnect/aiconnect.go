// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

// Package aiconnect finds installed AI apps that can start local MCP servers
// and adds or removes the CaseHUD entry in their configuration. It only ever
// touches the one key "casehud", keeps the rest of the file, backs the file up
// first and refuses anything it cannot read as JSON. All folders come from the
// Env, never from a request.
package aiconnect

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// ServerName is the stable name of the entry in every app.
const ServerName = "casehud"

// MaxConfigSize caps the config files that are read and written.
const MaxConfigSize = 1 << 20

// States of an app.
const (
	Missing   = "missing"   // not installed
	Found     = "found"     // installed, not connected
	Connected = "connected" // our entry is there
)

// Errors with a meaning for the caller.
var (
	// ErrUnknownApp: the id is not in the fixed list.
	ErrUnknownApp = errors.New("unknown app")
	// ErrConflict: an entry named "casehud" exists that does not start this CaseHUD.
	ErrConflict = errors.New("another entry named \"casehud\" exists")
	// ErrNotFound: the app is not installed.
	ErrNotFound = errors.New("the app is not installed")
)

// Codes of the errors: stable names for the window, whatever the text says.
const (
	CodeInvalidJSON   = "invalid_json"   // the config is not valid JSON (or has comments)
	CodeDuplicateKeys = "duplicate_keys" // an object holds a key twice
	CodeSymlink       = "symlink"        // a link or junction on the way to the config
	CodeNotFound      = "not_found"      // the app or its config is not there
	CodeWriteFailed   = "write_failed"   // the config (or its backup) could not be written
	CodeCLIFailed     = "cli_failed"     // the claude command failed
	CodeUnsupported   = "unsupported"    // this case is not handled
	CodeConflict      = "conflict"       // a foreign entry named "casehud" exists
)

// InvalidError says why a config file was left alone. Its text names the
// file: it is for the log, not for the window (see CodeOf).
type InvalidError struct {
	Path   string
	Code   string
	Reason string
}

func (e *InvalidError) Error() string {
	return fmt.Sprintf("%s was not changed: %s", e.Path, e.Reason)
}

// CLIError: running the claude command failed.
type CLIError struct{ Err error }

func (e *CLIError) Error() string { return e.Err.Error() }
func (e *CLIError) Unwrap() error { return e.Err }

// CodeOf names the kind of an error of Connect or Disconnect with one of the
// Code constants.
func CodeOf(err error) string {
	var inv *InvalidError
	var cli *CLIError
	switch {
	case errors.Is(err, ErrConflict):
		return CodeConflict
	case errors.Is(err, ErrNotFound), errors.Is(err, ErrUnknownApp):
		return CodeNotFound
	case errors.As(err, &inv):
		if inv.Code != "" {
			return inv.Code
		}
		return CodeInvalidJSON
	case errors.As(err, &cli):
		return CodeCLIFailed
	case errors.Is(err, errNoExe):
		return CodeUnsupported
	}
	return CodeWriteFailed
}

var errNoExe = errors.New("casehud-cli was not found next to CaseHUD")

// parseFail turns the error of parseObject into the error of the caller.
func parseFail(p string, err error) error {
	if errors.Is(err, errDuplicateKey) {
		return &InvalidError{Path: p, Code: CodeDuplicateKeys, Reason: "it contains a key twice"}
	}
	return notJSON(p)
}

type kind int

const (
	kindFile kind = iota
	kindCLI
)

type app struct {
	id, name string
	kind     kind
	// dir must exist for the app to count as installed; file is the config.
	// Both are relative to base. For the CLI app they serve detection and the
	// read-only check of the state.
	base func(Env) string
	dir  string
	file string
	key  string // top-level object that holds the servers
	typ  bool   // the entry carries "type": "stdio"
}

var apps = []app{
	{id: "claude-desktop", name: "Claude Desktop", base: func(e Env) string { return e.AppData }, dir: "Claude", file: "Claude/claude_desktop_config.json", key: "mcpServers"},
	{id: "claude-code", name: "Claude Code", kind: kindCLI, base: func(e Env) string { return e.Home }, dir: ".claude", file: ".claude.json", key: "mcpServers"},
	{id: "cursor", name: "Cursor", base: func(e Env) string { return e.Home }, dir: ".cursor", file: ".cursor/mcp.json", key: "mcpServers", typ: true},
	{id: "vscode", name: "Visual Studio Code", base: func(e Env) string { return e.AppData }, dir: "Code/User", file: "Code/User/mcp.json", key: "servers", typ: true},
	{id: "lmstudio", name: "LM Studio", base: func(e Env) string { return e.Home }, dir: ".lmstudio", file: ".lmstudio/mcp.json", key: "mcpServers"},
}

// Runner starts a program with fixed arguments (no shell) and returns its output.
type Runner func(ctx context.Context, name string, args ...string) ([]byte, error)

// Env is where things are. Tests fill it with temporary folders.
type Env struct {
	AppData string // %APPDATA%
	Home    string // user profile
	// Exe is the casehud-cli the entries start.
	Exe string
	// LookPath finds a program on PATH (default exec.LookPath).
	LookPath func(string) (string, error)
	// Run starts a program (default exec.CommandContext).
	Run Runner
	// Now is the clock for backup names.
	Now func() time.Time
}

// Status is one app as shown to the user.
type Status struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	State string `json:"state"`
	// Manual: the app is set up with a command the user runs (no CLI found).
	Manual bool `json:"manual,omitempty"`
	// Command is that command, for copying.
	Command string `json:"command,omitempty"`
}

// Result of connecting or disconnecting.
type Result struct {
	Backup  string `json:"backup,omitempty"`
	Manual  bool   `json:"manual,omitempty"`
	Command string `json:"command,omitempty"`
}

func (e Env) lookPath(n string) (string, error) {
	if e.LookPath != nil {
		return e.LookPath(n)
	}
	return exec.LookPath(n)
}

func (e Env) now() time.Time {
	if e.Now != nil {
		return e.Now()
	}
	return time.Now()
}

// isBatch tells a program that Windows starts through cmd.exe.
func isBatch(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return ext == ".cmd" || ext == ".bat"
}

// safeBatchArgs: cmd.exe reads some characters in an argument as commands
// whatever the quoting, so a batch file only gets arguments without them.
func safeBatchArgs(args []string) bool {
	for _, a := range args {
		if strings.ContainsAny(a, "%^&|<>\"!\r\n\x00") {
			return false
		}
	}
	return true
}

func (e Env) run(ctx context.Context, name string, args ...string) ([]byte, error) {
	if e.Run != nil {
		return e.Run(ctx, name, args...)
	}
	if isBatch(name) && !safeBatchArgs(args) {
		return nil, errors.New("an argument is not safe for a batch file")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, name, args...).CombinedOutput() //nolint:gosec // fixed program and arguments, no shell
}

func find(id string) (app, bool) {
	for _, a := range apps {
		if a.id == id {
			return a, true
		}
	}
	return app{}, false
}

// Name returns the display name of an app id.
func Name(id string) (string, bool) {
	a, ok := find(id)
	return a.name, ok
}

func (a app) path(e Env, rel string) string {
	b := a.base(e)
	if b == "" {
		return ""
	}
	return filepath.Join(b, filepath.FromSlash(rel))
}

func isDir(p string) bool {
	if p == "" {
		return false
	}
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

// ServerArgs are the arguments of the entry of an app.
func ServerArgs(id string) []string { return []string{"mcp", "--client", id} }

// cliCommand is the command that sets up Claude Code by hand.
func (e Env) cliCommand() string {
	return "claude mcp add --transport stdio --scope user " + ServerName + ` -- "` + e.Exe + `" mcp --client claude-code`
}

// Detect lists all apps with their state.
func (e Env) Detect() []Status {
	out := make([]Status, 0, len(apps))
	for _, a := range apps {
		out = append(out, e.status(a))
	}
	return out
}

func (e Env) status(a app) Status {
	st := Status{ID: a.id, Name: a.name, State: Missing}
	if a.kind == kindCLI {
		_, err := e.lookPath("claude")
		if err != nil && !isDir(a.path(e, a.dir)) {
			return st
		}
		st.State = Found
		st.Manual = err != nil
		if st.Manual {
			st.Command = e.cliCommand()
		}
	} else {
		if !isDir(a.path(e, a.dir)) {
			return st
		}
		st.State = Found
	}
	if state := e.entryState(a); state == entryOurs {
		st.State = Connected
	}
	return st
}

type entryKind int

const (
	entryNone entryKind = iota
	entryOurs
	entryForeign
	entryUnreadable
)

// entryState reads the config (never writes) and classifies the entry.
func (e Env) entryState(a app) entryKind {
	raw, err := readConfig(a.base(e), a.path(e, a.file))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return entryNone
		}
		return entryUnreadable
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return entryNone
	}
	top, err := parseObject(raw)
	if err != nil {
		return entryUnreadable
	}
	servers, ok := top.get(a.key)
	if !ok {
		return entryNone
	}
	sv, err := parseObject(servers)
	if err != nil {
		return entryUnreadable
	}
	ent, ok := sv.get(ServerName)
	if !ok {
		return entryNone
	}
	if e.isOurs(ent) {
		return entryOurs
	}
	return entryForeign
}

// isOurs tells whether an entry starts this CaseHUD's casehud-cli.
func (e Env) isOurs(ent json.RawMessage) bool {
	var v struct {
		Command string `json:"command"`
	}
	if json.Unmarshal(ent, &v) != nil || v.Command == "" || e.Exe == "" {
		return false
	}
	return samePath(v.Command, e.Exe)
}

func samePath(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func notJSON(p string) error {
	return &InvalidError{Path: p, Code: CodeInvalidJSON, Reason: "it is not valid JSON (comments are not supported)"}
}

// Connect adds the entry. pre runs after every check passed and just before
// the write (it hands out the token); the undo function it returns runs when
// the write fails. replace allows overwriting a foreign entry named "casehud".
func (e Env) Connect(ctx context.Context, id string, replace bool, pre func() (undo func(), err error)) (Result, error) {
	a, ok := find(id)
	if !ok {
		return Result{}, ErrUnknownApp
	}
	if e.Exe == "" {
		return Result{}, errNoExe
	}
	if a.kind == kindCLI {
		return e.connectCLI(ctx, a, replace, pre)
	}
	if !isDir(a.path(e, a.dir)) {
		return Result{}, ErrNotFound
	}
	p := a.path(e, a.file)
	raw, err := readConfig(a.base(e), p)
	exists := err == nil
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return Result{}, err
	}
	top := object{}
	if exists && len(bytes.TrimSpace(raw)) > 0 {
		if top, err = parseObject(raw); err != nil {
			return Result{}, parseFail(p, err)
		}
	}
	var servers object
	if sraw, has := top.get(a.key); has {
		if servers, err = parseObject(sraw); err != nil {
			if errors.Is(err, errDuplicateKey) {
				return Result{}, parseFail(p, err)
			}
			return Result{}, &InvalidError{Path: p, Code: CodeInvalidJSON, Reason: `"` + a.key + `" is not an object`}
		}
	}
	if ent, has := servers.get(ServerName); has && !e.isOurs(ent) && !replace {
		return Result{}, ErrConflict
	}
	entry := map[string]any{"command": e.Exe, "args": ServerArgs(id)}
	if a.typ {
		entry["type"] = "stdio"
	}
	eb, _ := json.Marshal(entry)
	servers.set(ServerName, eb)
	top.set(a.key, servers.marshal())
	out, err := indent(top.marshal())
	if err != nil {
		return Result{}, err
	}
	var undo func()
	if pre != nil {
		if undo, err = pre(); err != nil {
			return Result{}, err
		}
	}
	backup, err := e.writeConfig(a.base(e), p, out, exists)
	if err != nil {
		if undo != nil {
			undo()
		}
		return Result{}, err
	}
	return Result{Backup: backup}, nil
}

// Disconnect removes our entry. A foreign entry of the same name stays.
func (e Env) Disconnect(ctx context.Context, id string) (Result, error) {
	a, ok := find(id)
	if !ok {
		return Result{}, ErrUnknownApp
	}
	if a.kind == kindCLI {
		return e.disconnectCLI(ctx, a)
	}
	p := a.path(e, a.file)
	raw, err := readConfig(a.base(e), p)
	if errors.Is(err, os.ErrNotExist) {
		return Result{}, nil
	}
	if err != nil {
		return Result{}, err
	}
	top, err := parseObject(raw)
	if err != nil {
		return Result{}, parseFail(p, err)
	}
	sraw, has := top.get(a.key)
	if !has {
		return Result{}, nil
	}
	servers, err := parseObject(sraw)
	if err != nil {
		if errors.Is(err, errDuplicateKey) {
			return Result{}, parseFail(p, err)
		}
		return Result{}, &InvalidError{Path: p, Code: CodeInvalidJSON, Reason: `"` + a.key + `" is not an object`}
	}
	ent, has := servers.get(ServerName)
	if !has {
		return Result{}, nil
	}
	if !e.isOurs(ent) {
		return Result{}, ErrConflict
	}
	servers.del(ServerName)
	top.set(a.key, servers.marshal())
	out, err := indent(top.marshal())
	if err != nil {
		return Result{}, err
	}
	backup, err := e.writeConfig(a.base(e), p, out, true)
	return Result{Backup: backup}, err
}

func (e Env) connectCLI(ctx context.Context, a app, replace bool, pre func() (func(), error)) (Result, error) {
	claude, lerr := e.lookPath("claude")
	state := e.entryState(a)
	switch {
	case state == entryForeign && !replace:
		return Result{}, ErrConflict
	case state == entryUnreadable && lerr == nil:
		// The CLI owns this file; a damaged file is not ours to guess about.
		return Result{}, &InvalidError{Path: a.path(e, a.file), Code: CodeInvalidJSON, Reason: "it could not be read as JSON"}
	}
	var undo func()
	if pre != nil {
		var perr error
		if undo, perr = pre(); perr != nil {
			return Result{}, perr
		}
	}
	if lerr != nil { // no CLI on PATH: the user runs the command
		return Result{Manual: true, Command: e.cliCommand()}, nil //nolint:nilerr // no CLI on PATH is expected: the user runs the command
	}
	fail := func(err error) (Result, error) {
		if undo != nil {
			undo()
		}
		return Result{}, err
	}
	if state == entryOurs || state == entryForeign {
		if out, err := e.run(ctx, claude, "mcp", "remove", ServerName, "--scope", "user"); err != nil {
			return fail(&CLIError{fmt.Errorf("claude mcp remove failed: %w: %s", err, firstLine(out))})
		}
	}
	args := append([]string{"mcp", "add", "--transport", "stdio", "--scope", "user", ServerName, "--", e.Exe}, ServerArgs(a.id)...)
	if out, err := e.run(ctx, claude, args...); err != nil {
		return fail(&CLIError{fmt.Errorf("claude mcp add failed: %w: %s", err, firstLine(out))})
	}
	return Result{}, nil
}

func (e Env) disconnectCLI(ctx context.Context, a app) (Result, error) {
	claude, lerr := e.lookPath("claude")
	switch e.entryState(a) {
	case entryForeign:
		return Result{}, ErrConflict
	case entryOurs:
	default:
		return Result{}, nil
	}
	if lerr != nil {
		return Result{Manual: true, Command: "claude mcp remove " + ServerName + " --scope user"}, nil //nolint:nilerr // no CLI on PATH is expected: the user runs the command
	}
	if out, err := e.run(ctx, claude, "mcp", "remove", ServerName, "--scope", "user"); err != nil {
		return Result{}, &CLIError{fmt.Errorf("claude mcp remove failed: %w: %s", err, firstLine(out))}
	}
	return Result{}, nil
}

func firstLine(b []byte) string {
	s := strings.TrimSpace(string(b))
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		s = s[:i]
	}
	if len(s) > 200 {
		s = s[:200]
	}
	return s
}
