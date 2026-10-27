package panel

import (
	"sync"
	"time"
)

// Several devices keep brightness and rotation in flash memory, and flash
// survives only a limited number of writes. SettingsCache remembers the last
// value written to each device (across restarts, the host persists it), so a
// start-up, a rescan or a repeated request does not write a value the device
// already holds. A device is identified by a stable key (see KeyedPanel).

// Written is what was last written to one device; nil means unknown.
type Written struct {
	Brightness *int `json:"b,omitempty"`
	Rotation   *int `json:"r,omitempty"`
	// OfflineClock is the flag of the built-in clock that the device keeps
	// (shown when no host feeds it); nil = never written by CaseHUD.
	OfflineClock *bool `json:"oc,omitempty"`
}

// SettingsCache is safe for concurrent use.
type SettingsCache struct {
	mu       sync.Mutex
	m        map[string]Written
	onChange func(map[string]Written)
}

// NewSettingsCache returns a cache holding a copy of init.
func NewSettingsCache(init map[string]Written) *SettingsCache {
	c := &SettingsCache{m: map[string]Written{}}
	for k, v := range init {
		c.m[k] = v
	}
	return c
}

// OnChange registers a function called (outside the lock) with a copy of the
// cache after every change.
func (c *SettingsCache) OnChange(f func(map[string]Written)) {
	c.mu.Lock()
	c.onChange = f
	c.mu.Unlock()
}

// Get returns the last written values of the device.
func (c *SettingsCache) Get(key string) Written {
	if c == nil {
		return Written{}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.m[key]
}

// PutBrightness and PutRotation record a successful write.
func (c *SettingsCache) PutBrightness(key string, v int) { c.put(key, &v, nil) }
func (c *SettingsCache) PutRotation(key string, v int)   { c.put(key, nil, &v) }

// PutOfflineClock records a successful write of the offline-clock flag.
func (c *SettingsCache) PutOfflineClock(key string, v bool) {
	if c == nil || key == "" {
		return
	}
	c.mu.Lock()
	w := c.m[key]
	if w.OfflineClock != nil && *w.OfflineClock == v {
		c.mu.Unlock()
		return
	}
	w.OfflineClock = &v
	c.m[key] = w
	f, snap := c.onChange, c.snapshotLocked()
	c.mu.Unlock()
	if f != nil {
		f(snap)
	}
}

func (c *SettingsCache) snapshotLocked() map[string]Written {
	snap := make(map[string]Written, len(c.m))
	for k, v := range c.m {
		snap[k] = v
	}
	return snap
}

func (c *SettingsCache) put(key string, b, r *int) {
	if c == nil || key == "" {
		return
	}
	c.mu.Lock()
	w := c.m[key]
	changed := false
	if b != nil && (w.Brightness == nil || *w.Brightness != *b) {
		w.Brightness, changed = b, true
	}
	if r != nil && (w.Rotation == nil || *w.Rotation != *r) {
		w.Rotation, changed = r, true
	}
	var snap map[string]Written
	var f func(map[string]Written)
	if changed {
		c.m[key] = w
		f = c.onChange
		if f != nil {
			snap = make(map[string]Written, len(c.m))
			for k, v := range c.m {
				snap[k] = v
			}
		}
	}
	c.mu.Unlock()
	if f != nil {
		f(snap)
	}
}

var (
	cacheMu      sync.RWMutex
	currentCache *SettingsCache
)

// SetSettingsCache installs the cache the drivers use (call before discovery).
func SetSettingsCache(c *SettingsCache) {
	cacheMu.Lock()
	currentCache = c
	cacheMu.Unlock()
}

// Cache is the installed cache (nil: none, drivers then write as if unknown).
func Cache() *SettingsCache {
	cacheMu.RLock()
	defer cacheMu.RUnlock()
	return currentCache
}

// FlashStored marks panels whose brightness and rotation commands are stored
// by the device (flash). The host skips values the device already holds and
// never changes them for effects like screen-off.
type FlashStored interface {
	// SettingsKey identifies the device in the SettingsCache.
	SettingsKey() string
}

// SettingsRateLimit is the shortest time between two settings writes to one
// device.
const SettingsRateLimit = time.Second

// SleepByFrame is implemented by panels that must not get brightness 0 for
// screen-off (the value would be stored): the host shows black frames.
type SleepByFrame interface{ SleepByFrame() bool }
