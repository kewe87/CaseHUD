// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAcquireInstance(t *testing.T) {
	never := func() bool { return false }
	always := func() bool { return true }

	if got := acquireInstance(always, never, time.Second, time.Millisecond); got != lockHeld {
		t.Fatalf("free lock: %v", got)
	}
	if got := acquireInstance(never, always, time.Second, time.Millisecond); got != lockHandedOver {
		t.Fatalf("holder answers: %v", got)
	}
	start := time.Now()
	if got := acquireInstance(never, never, 50*time.Millisecond, time.Millisecond); got != lockTimeout {
		t.Fatalf("nobody answers: %v", got)
	}
	if time.Since(start) < 50*time.Millisecond {
		t.Fatal("gave up before the wait was over")
	}
	// The lock becomes free while waiting: this start takes over.
	n := 0
	freed := func() bool { n++; return n >= 4 }
	if got := acquireInstance(freed, never, time.Second, time.Millisecond); got != lockHeld {
		t.Fatalf("lock freed: %v", got)
	}
}

func TestRemoveMarkerKeepsForeign(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "instance.pid")
	if err := os.WriteFile(p, []byte("4711"), 0o600); err != nil {
		t.Fatal(err)
	}
	removeMarker(p, "4712") // another instance's marker stays
	if _, err := os.Stat(p); err != nil {
		t.Fatal("foreign marker was removed")
	}
	removeMarker(p, "4711")
	if _, err := os.Stat(p); err == nil {
		t.Fatal("own marker stayed")
	}
	removeMarker(p, "4711") // already gone is fine
}

func TestInstanceURLMustBeOnTheLoopback(t *testing.T) {
	good := map[string]string{
		"http://127.0.0.1:8086/?t=abc":   "http://127.0.0.1:8086",
		"http://localhost:51234/?t=abc":  "http://localhost:51234",
		"http://[::1]:8086/?t=abc":       "http://[::1]:8086",
		" http://127.0.0.1:8086/?t=abc ": "http://127.0.0.1:8086",
	}
	for raw, base := range good {
		b, tok, ok := splitInstanceURL(raw)
		if !ok || b != base || tok != "abc" {
			t.Errorf("%q: %q %q %v", raw, b, tok, ok)
		}
	}
	for _, raw := range []string{
		"", "http://192.168.1.5:8086/?t=abc", "http://example.com:8086/?t=abc", "https://127.0.0.1:8086/?t=abc",
		"http://127.0.0.1/?t=abc", "http://127.0.0.1:8086/", "http://user@127.0.0.1:8086/?t=abc",
		"http://127.0.0.1.evil.example:8086/?t=abc", "not a url", "ftp://127.0.0.1:21/?t=abc",
	} {
		if b, _, ok := splitInstanceURL(raw); ok {
			t.Errorf("%q accepted as %q", raw, b)
		}
	}
}

func TestLockTimeoutTextSaysWhatIsKnown(t *testing.T) {
	for _, german := range []bool{true, false} {
		busy := lockTimeoutText(german, "")
		failed := lockTimeoutText(german, "the handle is invalid")
		if busy == failed || !strings.Contains(busy, runningMutexName) || !strings.Contains(failed, "the handle is invalid") {
			t.Errorf("german=%v: %q / %q", german, busy, failed)
		}
		// A foreign program holding the name is not CaseHUD "running".
		if strings.Contains(busy, "läuft bereits, antwortet aber nicht.") || strings.Contains(busy, "is already running but does not respond.") {
			t.Errorf("claims CaseHUD runs: %q", busy)
		}
	}
}
