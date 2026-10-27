package app

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/kewe87/casehud/internal/media"
)

// Encoded videos for displays that play H.264 themselves live in
// <data dir>/devvideo, named by a hash of the source file and the settings.
// They can be deleted at any time; they are made again when needed. The
// folder is kept below devVideoCacheMax by removing the least recently used
// files after every encode.

const (
	devVideoCacheMax = 512 << 20
	// videoGrace is how long a display waits for the encoding of a video
	// before it draws the frames itself.
	videoGrace = 20 * time.Second
	// devVideoRetry is how long a failed encode is remembered.
	devVideoRetry = 5 * time.Minute
)

type jobState int

const (
	jobPending jobState = iota
	jobReady
	jobFailed
)

// devVideoJob is one encode (or a file that was already there).
type devVideoJob struct {
	path    string
	started time.Time
	done    chan struct{}
	err     error // set before done is closed
}

func (j *devVideoJob) state() jobState {
	select {
	case <-j.done:
		if j.err != nil {
			return jobFailed
		}
		if _, err := os.Stat(j.path); err != nil {
			return jobFailed // the file was removed meanwhile
		}
		return jobReady
	default:
		return jobPending
	}
}

// devVideoState is the engine's list of encodes.
type devVideoState struct {
	mu   sync.Mutex
	jobs map[string]*devVideoJob
	sem  chan struct{} // one encode at a time
}

// devVideoJobsMax bounds the table of jobs.
const devVideoJobsMax = 64

// pruneLocked drops failed jobs whose retry time is over and, when the table
// is still too big, the oldest finished ones (a finished job is rebuilt from
// its file on demand). Running jobs and keep stay. The caller holds mu.
func (d *devVideoState) pruneLocked(keep string) {
	for p, j := range d.jobs {
		if p != keep && j.state() == jobFailed && time.Since(j.started) >= devVideoRetry {
			delete(d.jobs, p)
		}
	}
	for len(d.jobs) >= devVideoJobsMax {
		oldest := ""
		for p, j := range d.jobs {
			if p == keep || j.state() == jobPending {
				continue
			}
			if oldest == "" || j.started.Before(d.jobs[oldest].started) {
				oldest = p
			}
		}
		if oldest == "" {
			return
		}
		delete(d.jobs, oldest)
	}
}

// ffmpegProbe finds ffmpeg (replaced in tests); the answer is remembered for a
// few seconds, so installing ffmpeg is noticed without a restart.
var ffmpegProbe = media.FFmpegPath

var ffmpegMemo struct {
	mu   sync.Mutex
	at   time.Time
	path string
}

func ffmpegAvailable() bool {
	ffmpegMemo.mu.Lock()
	defer ffmpegMemo.mu.Unlock()
	if ffmpegMemo.at.IsZero() || time.Since(ffmpegMemo.at) > 10*time.Second {
		ffmpegMemo.path, ffmpegMemo.at = ffmpegProbe(), time.Now()
	}
	return ffmpegMemo.path != ""
}

// deviceVideoEnabled reports whether videos may be played by the displays:
// the option is on (default) and ffmpeg is there.
func (e *Engine) deviceVideoEnabled() bool {
	e.mu.Lock()
	off := e.cfg.NoDeviceVideo
	e.mu.Unlock()
	return !off && ffmpegAvailable()
}

// deviceVideoJob returns the encode of media file name for spec: ready when the
// file is already in the cache, otherwise started in the background. nil
// means the file is not a usable video (or the option is off).
func (e *Engine) deviceVideoJob(name string, spec media.DeviceVideoSpec) *devVideoJob {
	if !e.deviceVideoEnabled() {
		return nil
	}
	src, err := e.Store.MediaPath(name)
	if err != nil || media.Kind(src) != "video" {
		return nil
	}
	fname, err := media.DeviceVideoName(src, spec)
	if err != nil {
		return nil
	}
	dir := filepath.Join(e.Store.Dir, "devvideo")
	out := filepath.Join(dir, fname)

	e.dv.mu.Lock()
	defer e.dv.mu.Unlock()
	if e.dv.jobs == nil {
		e.dv.jobs = map[string]*devVideoJob{}
		e.dv.sem = make(chan struct{}, 1)
	}
	if j := e.dv.jobs[out]; j != nil {
		if _, gone := os.Stat(out); j.err == nil && gone != nil && j.state() == jobFailed {
			// finished, but the file is gone: encode again
		} else if j.state() != jobFailed || time.Since(j.started) < devVideoRetry {
			return j
		}
	}
	j := &devVideoJob{path: out, started: time.Now(), done: make(chan struct{})}
	e.dv.pruneLocked(out)
	e.dv.jobs[out] = j
	if st, err := os.Stat(out); err == nil && st.Mode().IsRegular() && st.Size() > 0 {
		media.TouchDeviceVideo(out) // in use: cleaned last
		close(j.done)
		return j
	}
	go e.runDeviceVideoJob(j, src, dir, spec)
	return j
}

func (e *Engine) runDeviceVideoJob(j *devVideoJob, src, dir string, spec media.DeviceVideoSpec) {
	ctx := e.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case e.dv.sem <- struct{}{}:
		defer func() { <-e.dv.sem }()
	case <-ctx.Done():
		j.err = ctx.Err()
		close(j.done)
		return
	}
	j.err = media.EncodeDeviceVideo(ctx, src, j.path, spec)
	if j.err != nil {
		log.Printf("display video: %s: %v", filepath.Base(src), j.err)
	} else {
		e.dv.mu.Lock()
		keep := []string{j.path}
		for p, o := range e.dv.jobs {
			if o.state() == jobReady {
				keep = append(keep, p)
			}
		}
		e.dv.mu.Unlock()
		media.CleanDeviceVideos(dir, devVideoCacheMax, keep...)
	}
	close(j.done)
}
