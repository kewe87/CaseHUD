// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

// Waiting for the instance that holds the lock: a start that lost the race
// waits for its window (it is still starting) or for the lock (it is quitting).
const (
	lockWait = 20 * time.Second
	lockPoll = 200 * time.Millisecond
)

// runningMutexName is the name of the single-instance lock (a mutex on Windows).
const runningMutexName = "CaseHUD.Running"

type lockResult int

const (
	lockHeld       lockResult = iota // this start owns the lock and may run
	lockHandedOver                   // the instance that owns it answered
	lockTimeout                      // neither: this start must end
)

// acquireInstance takes the single-instance lock through try (atomic, true if
// it was won). If another start holds it, it waits up to wait: when
// answered reports that the holder is up, the start is handed over to it; when
// the lock becomes free in the meantime, this start takes over.
func acquireInstance(try, answered func() bool, wait, poll time.Duration) lockResult {
	if try() {
		return lockHeld
	}
	deadline := time.Now().Add(wait)
	for {
		if answered() {
			return lockHandedOver
		}
		if try() {
			return lockHeld
		}
		if !time.Now().Before(deadline) {
			return lockTimeout
		}
		time.Sleep(poll)
	}
}

// removeMarker deletes a marker file only if it still holds want, so an
// instance that ends never removes the file of another one.
func removeMarker(path, want string) {
	if b, err := os.ReadFile(path); err == nil && strings.TrimSpace(string(b)) == want {
		os.Remove(path)
	}
}

func pidString() string { return strconv.Itoa(os.Getpid()) }

// holdFailure is why the lock could not be taken for a reason other than that
// somebody holds it ("" if there was none). The start then does not run: not
// knowing whether another instance runs is no reason to risk a second one.
var holdFailure atomic.Value // string

func noteHoldFailure(err error) { holdFailure.Store(err.Error()) }

func lastHoldFailure() string {
	s, _ := holdFailure.Load().(string)
	return s
}

// lockTimeoutText is the message of a start that did not get the lock. failure is
// lastHoldFailure(): empty if the lock is held by somebody who did not answer.
func lockTimeoutText(german bool, failure string) string {
	if failure != "" {
		if german {
			return fmt.Sprintf("Die Sperre für die Einzelinstanz („%s“) konnte nicht angelegt werden: %s. Dieser Start wurde abgebrochen, damit nicht zwei Instanzen gleichzeitig auf die Geräte zugreifen.", runningMutexName, failure)
		}
		return fmt.Sprintf("The single-instance lock (\"%s\") could not be created: %s. This start was cancelled so that two instances do not access the devices at the same time.", runningMutexName, failure)
	}
	if german {
		return fmt.Sprintf("Die Sperre „%s“ ist belegt, und es kam keine Antwort: Entweder läuft CaseHUD und antwortet nicht, oder ein anderes Programm belegt diesen Namen. Dieser Start wurde abgebrochen, damit nicht zwei Instanzen gleichzeitig auf die Geräte zugreifen. Einzelheiten stehen in casehud-start.log im Datenordner.", runningMutexName)
	}
	return fmt.Sprintf("The lock \"%s\" is held and nobody answered: either CaseHUD is running and does not respond, or another program holds this name. This start was cancelled so that two instances do not access the devices at the same time. Details are in casehud-start.log in the data folder.", runningMutexName)
}

// splitInstanceURL takes the address of a running instance from instance.url: its
// base URL and the session token. Only an address on the loopback of this
// machine counts; whatever else the file says is not talked to.
func splitInstanceURL(raw string) (base, token string, ok bool) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "http" || u.User != nil || u.Port() == "" {
		return "", "", false
	}
	host := u.Hostname()
	if ip := net.ParseIP(host); !(host == "localhost" || ip != nil && ip.IsLoopback()) {
		return "", "", false
	}
	token = u.Query().Get("t")
	if token == "" {
		return "", "", false
	}
	return "http://" + u.Host, token, true
}
