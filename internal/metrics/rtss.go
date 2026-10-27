package metrics

// RTSS reads the frame rate of the foreground game from RivaTuner Statistics
// Server's shared memory (free, ships with MSI Afterburner). Readings appear
// as "RTSS/FPS/Framerate" (FPS) and "RTSS/FPS/Frametime" (ms) while a game
// with the RTSS overlay hook is running; otherwise the list is empty.
type RTSS struct {
	readings []Reading
}

// NewRTSS returns an RTSS provider.
func NewRTSS() *RTSS { return &RTSS{} }

// Refresh implements Provider. A missing RTSS is not an error.
func (r *RTSS) Refresh() error {
	r.readings = r.readings[:0]
	fps, frametime, ok := readRTSS()
	if ok {
		r.readings = append(r.readings,
			Reading{Key: "/rtss/fps", Path: "RTSS/FPS/Framerate", Value: fps, Unit: "FPS"},
			Reading{Key: "/rtss/frametime", Path: "RTSS/FPS/Frametime", Value: frametime, Unit: "ms"})
	}
	return nil
}

// Get implements Provider.
func (r *RTSS) Get(source string) (Reading, bool) { return match(r.readings, source) }

// All implements Provider.
func (r *RTSS) All() []Reading { return r.readings }

// rtssApp is the part of RTSS_SHARED_MEMORY_APP_ENTRY we use.
type rtssApp struct {
	pid                         uint32
	time0, time1, frames, ftime uint32 // ms ticks, frame count, frametime in µs
}

// pickRTSSApp chooses the foreground process if RTSS tracks it, otherwise
// the most recently updated app, ignoring entries idle for over 2 seconds.
func pickRTSSApp(apps []rtssApp, foreground, now uint32) (fps, frametime float64, ok bool) {
	var best *rtssApp
	for i := range apps {
		a := &apps[i]
		if a.pid == 0 || a.time1 <= a.time0 || now-a.time1 > 2000 {
			continue
		}
		if a.pid == foreground {
			best = a
			break
		}
		if best == nil || a.time1 > best.time1 {
			best = a
		}
	}
	if best == nil {
		return 0, 0, false
	}
	fps = 1000 * float64(best.frames) / float64(best.time1-best.time0)
	return fps, float64(best.ftime) / 1000, true
}
