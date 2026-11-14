// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

package aiconnect

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func testEnv(t *testing.T) Env {
	t.Helper()
	root := t.TempDir()
	e := Env{AppData: filepath.Join(root, "appdata"), Home: filepath.Join(root, "home"), Exe: filepath.Join(root, "bin", "casehud-cli.exe"),
		LookPath: func(string) (string, error) { return "", errors.New("none") },
		Now:      func() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) }}
	for _, d := range []string{e.AppData, e.Home} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return e
}

func mk(t *testing.T, p, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func state(e Env, id string) string {
	for _, s := range e.Detect() {
		if s.ID == id {
			return s.State
		}
	}
	return ""
}

func TestDetect(t *testing.T) {
	e := testEnv(t)
	for _, s := range e.Detect() {
		if s.State != Missing {
			t.Errorf("%s: %s", s.ID, s.State)
		}
	}
	mk(t, filepath.Join(e.AppData, "Claude", "x"), "")
	if state(e, "claude-desktop") != Found {
		t.Fatal("not found")
	}
	if state(e, "cursor") != Missing {
		t.Fatal("cursor")
	}
}

func TestConnectKeepsOtherContentAndBacksUp(t *testing.T) {
	e := testEnv(t)
	p := filepath.Join(e.AppData, "Claude", "claude_desktop_config.json")
	orig := "{\n  \"zeta\": {\"b\": 1, \"a\": [1,2]},\n  \"mcpServers\": {\n    \"other\": {\"command\": \"x\"}\n  },\n  \"alpha\": true\n}\n"
	mk(t, p, orig)
	res, err := e.Connect(context.Background(), "claude-desktop", false, nil)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(res.Backup)
	if string(b) != orig || !strings.HasSuffix(res.Backup, ".casehud-backup-20260102-030405") {
		t.Fatalf("backup %q", res.Backup)
	}
	got, _ := os.ReadFile(p)
	s := string(got)
	if strings.Index(s, "zeta") > strings.Index(s, "mcpServers") || strings.Index(s, "mcpServers") > strings.Index(s, "alpha") {
		t.Fatalf("order changed:\n%s", s)
	}
	for _, want := range []string{`"other"`, `"casehud"`, `"--client"`, `"claude-desktop"`, `"a": [`} {
		if !strings.Contains(s, want) {
			t.Fatalf("%s missing:\n%s", want, s)
		}
	}
	if state(e, "claude-desktop") != Connected {
		t.Fatal("not connected")
	}
	// A second connect changes nothing but still works.
	if _, err := e.Connect(context.Background(), "claude-desktop", false, nil); err != nil {
		t.Fatal(err)
	}
	// Disconnect removes only our key; the rest equals the original content.
	if _, err := e.Disconnect(context.Background(), "claude-desktop"); err != nil {
		t.Fatal(err)
	}
	got, _ = os.ReadFile(p)
	if strings.Contains(string(got), "casehud") || !strings.Contains(string(got), `"other"`) || !strings.Contains(string(got), `"zeta"`) {
		t.Fatalf("after disconnect:\n%s", got)
	}
	if state(e, "claude-desktop") != Found {
		t.Fatal("still connected")
	}
}

func TestConnectCreatesMissingFileAndVSCodeFormat(t *testing.T) {
	e := testEnv(t)
	mk(t, filepath.Join(e.AppData, "Code", "User", "settings.json"), "{}")
	res, err := e.Connect(context.Background(), "vscode", false, nil)
	if err != nil || res.Backup != "" {
		t.Fatal(err, res)
	}
	b, _ := os.ReadFile(filepath.Join(e.AppData, "Code", "User", "mcp.json"))
	if !strings.Contains(string(b), `"servers"`) || !strings.Contains(string(b), `"type": "stdio"`) {
		t.Fatal(string(b))
	}
}

func TestInvalidJSONRefusedNothingWritten(t *testing.T) {
	e := testEnv(t)
	p := filepath.Join(e.Home, ".cursor", "mcp.json")
	mk(t, p, "{ // comment\n\"mcpServers\": {} }")
	called := false
	_, err := e.Connect(context.Background(), "cursor", false, func() (func(), error) { called = true; return nil, nil })
	var inv *InvalidError
	if !errors.As(err, &inv) || called {
		t.Fatalf("%v called=%v", err, called)
	}
	if b, _ := os.ReadFile(p); string(b) != "{ // comment\n\"mcpServers\": {} }" {
		t.Fatal("file changed")
	}
	if m, _ := filepath.Glob(p + ".*"); len(m) != 0 {
		t.Fatal("backup written", m)
	}
}

func TestForeignEntryNeedsReplace(t *testing.T) {
	e := testEnv(t)
	p := filepath.Join(e.Home, ".lmstudio", "mcp.json")
	mk(t, p, `{"mcpServers":{"casehud":{"command":"C:\\other\\thing.exe"}}}`)
	if _, err := e.Connect(context.Background(), "lmstudio", false, nil); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	if _, err := e.Disconnect(context.Background(), "lmstudio"); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	if _, err := e.Connect(context.Background(), "lmstudio", true, nil); err != nil {
		t.Fatal(err)
	}
	if state(e, "lmstudio") != Connected {
		t.Fatal("not connected")
	}
}

func TestPreFailureAndUndo(t *testing.T) {
	e := testEnv(t)
	mk(t, filepath.Join(e.Home, ".cursor", "mcp.json"), "{}")
	if _, err := e.Connect(context.Background(), "cursor", false, func() (func(), error) { return nil, errors.New("no token") }); err == nil {
		t.Fatal("expected error")
	}
	if state(e, "cursor") != Found {
		t.Fatal("written despite error")
	}
	// A write that fails undoes the token.
	undone := false
	e.Home = filepath.Join(e.Home, "nope") // the folder is gone: ErrNotFound before pre
	if _, err := e.Connect(context.Background(), "cursor", false, func() (func(), error) { undone = true; return nil, nil }); !errors.Is(err, ErrNotFound) || undone {
		t.Fatal(err, undone)
	}
}

func TestLinksAreRefused(t *testing.T) {
	e := testEnv(t)
	dir := filepath.Join(e.Home, ".cursor")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(e.Home, "real.json")
	mk(t, target, "{}")
	if err := os.Symlink(target, filepath.Join(dir, "mcp.json")); err != nil {
		t.Skip("no symlinks here:", err)
	}
	_, err := e.Connect(context.Background(), "cursor", false, nil)
	var inv *InvalidError
	if !errors.As(err, &inv) {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(target); string(b) != "{}" {
		t.Fatal("target changed")
	}
}

func TestSizeCap(t *testing.T) {
	e := testEnv(t)
	mk(t, filepath.Join(e.Home, ".cursor", "mcp.json"), "{\"x\":\""+strings.Repeat("a", MaxConfigSize)+"\"}")
	var inv *InvalidError
	if _, err := e.Connect(context.Background(), "cursor", false, nil); !errors.As(err, &inv) {
		t.Fatal(err)
	}
}

func TestClaudeCodeViaRunner(t *testing.T) {
	e := testEnv(t)
	var calls [][]string
	e.LookPath = func(string) (string, error) { return "claude", nil }
	e.Run = func(_ context.Context, name string, args ...string) ([]byte, error) {
		calls = append(calls, append([]string{name}, args...))
		return nil, nil
	}
	if state(e, "claude-code") != Found {
		t.Fatal(state(e, "claude-code"))
	}
	if _, err := e.Connect(context.Background(), "claude-code", false, nil); err != nil {
		t.Fatal(err)
	}
	want := []string{"claude", "mcp", "add", "--transport", "stdio", "--scope", "user", "casehud", "--", e.Exe, "mcp", "--client", "claude-code"}
	if len(calls) != 1 || strings.Join(calls[0], "|") != strings.Join(want, "|") {
		t.Fatalf("%v", calls)
	}
	// The CLI wrote its file: now an entry of ours is there and a removal runs.
	mk(t, filepath.Join(e.Home, ".claude.json"), `{"numStartups":3,"mcpServers":{"casehud":{"type":"stdio","command":`+quote(e.Exe)+`,"args":["mcp"]}}}`)
	if state(e, "claude-code") != Connected {
		t.Fatal("not connected")
	}
	calls = nil
	if _, err := e.Disconnect(context.Background(), "claude-code"); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 || strings.Join(calls[0], " ") != "claude mcp remove casehud --scope user" {
		t.Fatalf("%v", calls)
	}
}

func TestClaudeCodeWithoutCLIGivesCommand(t *testing.T) {
	e := testEnv(t)
	mk(t, filepath.Join(e.Home, ".claude", "x"), "")
	res, err := e.Connect(context.Background(), "claude-code", false, nil)
	if err != nil || !res.Manual || !strings.Contains(res.Command, "claude mcp add") || !strings.Contains(res.Command, e.Exe) {
		t.Fatal(err, res)
	}
}

func TestRunnerFailureUndoesToken(t *testing.T) {
	e := testEnv(t)
	e.LookPath = func(string) (string, error) { return "claude", nil }
	e.Run = func(context.Context, string, ...string) ([]byte, error) {
		return []byte("boom\nmore"), errors.New("exit 1")
	}
	undone := false
	_, err := e.Connect(context.Background(), "claude-code", false, func() (func(), error) { return func() { undone = true }, nil })
	if err == nil || !undone || !strings.Contains(err.Error(), "boom") {
		t.Fatal(err, undone)
	}
}

func TestUnknownApp(t *testing.T) {
	e := testEnv(t)
	if _, err := e.Connect(context.Background(), "..\\evil", false, nil); !errors.Is(err, ErrUnknownApp) {
		t.Fatal(err)
	}
}

func quote(s string) string {
	return `"` + strings.ReplaceAll(s, `\`, `\\`) + `"`
}

func TestDuplicateKeysAreRefused(t *testing.T) {
	e := testEnv(t)
	p := filepath.Join(e.AppData, "Claude", "claude_desktop_config.json")
	orig := `{"mcpServers":{"a":{"command":"x"},"a":{"command":"y"}}}`
	mk(t, p, orig)
	_, err := e.Connect(context.Background(), "claude-desktop", false, nil)
	var inv *InvalidError
	if !errors.As(err, &inv) || CodeOf(err) != CodeDuplicateKeys {
		t.Fatalf("duplicate keys: %v (%s)", err, CodeOf(err))
	}
	if b, _ := os.ReadFile(p); string(b) != orig {
		t.Fatal("the file was changed")
	}
}

func TestCodes(t *testing.T) {
	e := testEnv(t)
	mk(t, filepath.Join(e.AppData, "Claude", "claude_desktop_config.json"), "{ // c\n}")
	_, err := e.Connect(context.Background(), "claude-desktop", false, nil)
	if CodeOf(err) != CodeInvalidJSON {
		t.Errorf("comments: %s", CodeOf(err))
	}
	if CodeOf(ErrConflict) != CodeConflict || CodeOf(ErrNotFound) != CodeNotFound || CodeOf(ErrUnknownApp) != CodeNotFound {
		t.Error("sentinel codes")
	}
	if CodeOf(&CLIError{errors.New("x")}) != CodeCLIFailed || CodeOf(errors.New("disk")) != CodeWriteFailed || CodeOf(errNoExe) != CodeUnsupported {
		t.Error("other codes")
	}
	_, err = e.Connect(context.Background(), "cursor", false, nil)
	if CodeOf(err) != CodeNotFound {
		t.Errorf("not installed: %v", err)
	}
}

func TestBackupsAreCapped(t *testing.T) {
	e := testEnv(t)
	p := filepath.Join(e.AppData, "Claude", "claude_desktop_config.json")
	mk(t, p, `{"mcpServers":{}}`)
	n := 0
	e.Now = func() time.Time { n++; return time.Date(2026, 1, 2, 3, 4, n, 0, time.UTC) }
	for range 12 {
		if _, err := e.Connect(context.Background(), "claude-desktop", true, nil); err != nil {
			t.Fatal(err)
		}
		// every Connect rewrites the file, so each leaves a backup
		if _, err := e.Disconnect(context.Background(), "claude-desktop"); err != nil {
			t.Fatal(err)
		}
	}
	left, _ := filepath.Glob(p + ".casehud-backup-*")
	if len(left) != maxBackups {
		t.Fatalf("%d backups left, want %d", len(left), maxBackups)
	}
}

func TestLinkOnTheWayIsRefused(t *testing.T) {
	e := testEnv(t)
	target := filepath.Join(t.TempDir(), "elsewhere")
	if err := os.MkdirAll(filepath.Join(target, "User"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Code/User is the folder of the VS Code config; make "Code" a link.
	link := filepath.Join(e.AppData, "Code")
	if err := os.Symlink(target, link); err != nil {
		// Windows without the right to make symbolic links: a junction needs none.
		if runtime.GOOS != "windows" || exec.Command("cmd", "/c", "mklink", "/J", link, target).Run() != nil {
			t.Skip("links are not available here:", err)
		}
	}
	_, err := e.Connect(context.Background(), "vscode", false, nil)
	if CodeOf(err) != CodeSymlink {
		t.Fatalf("link on the way: %v (%s)", err, CodeOf(err))
	}
	if _, serr := os.Stat(filepath.Join(target, "User", "mcp.json")); serr == nil {
		t.Fatal("wrote through the link")
	}
}

func TestBatchArguments(t *testing.T) {
	if !isBatch(`C:\x\claude.CMD`) || isBatch(`C:\x\claude.exe`) {
		t.Fatal("isBatch")
	}
	if !safeBatchArgs([]string{"mcp", "add", `C:\Program Files\CaseHUD\casehud-cli.exe`}) {
		t.Fatal("plain arguments refused")
	}
	for _, bad := range []string{`a&calc`, `%PATH%`, `a|b`, "a\nb", `a"b`, `^x`} {
		if safeBatchArgs([]string{bad}) {
			t.Errorf("%q passed", bad)
		}
	}
}
