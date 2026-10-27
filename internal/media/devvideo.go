package media

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/kewe87/casehud/internal/procjob"
)

// Videos for displays that play H.264 themselves are encoded once with the
// user's ffmpeg (found the same way as for decoding, never downloaded) into an
// Annex-B stream in the device's orientation and cached on disk.

const (
	// DeviceVideoSecs is the longest clip that is encoded.
	DeviceVideoSecs = 60
	// DeviceVideoMaxBytes caps one encoded file (the output stops there).
	DeviceVideoMaxBytes = 64 << 20
	// DeviceVideoFPS is the frame rate of the encoded stream.
	DeviceVideoFPS = 30

	deviceVideoTimeout = 10 * time.Minute
	// deviceVideoVersion is part of the cache key: raise it when the encoder
	// settings change.
	deviceVideoVersion = "1"
)

// DeviceVideoSpec says how a video is turned into a device stream.
type DeviceVideoSpec struct {
	W, H int     // size of the picture that is cover-cropped from the video, before the turn
	Turn int     // clockwise quarter turns (0..3) into the device frame
	Dim  float64 // brightness factor, (0, 1]; 1 leaves the picture as it is
}

func (s DeviceVideoSpec) valid() error {
	switch {
	case s.W < 16 || s.H < 16 || s.W > 4096 || s.H > 4096 || s.W%2 != 0 || s.H%2 != 0:
		return fmt.Errorf("video size %dx%d not usable", s.W, s.H)
	case s.Turn < 0 || s.Turn > 3:
		return fmt.Errorf("video turn %d not usable", s.Turn)
	case !(s.Dim > 0 && s.Dim <= 1):
		return fmt.Errorf("video brightness %v not usable", s.Dim)
	}
	return nil
}

// filter is the ffmpeg video filter chain. Only numbers go into it.
func (s DeviceVideoSpec) filter() string {
	f := fmt.Sprintf("fps=%d,scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d",
		DeviceVideoFPS, s.W, s.H, s.W, s.H)
	if s.Dim < 1 {
		f += fmt.Sprintf(",format=rgb24,lutrgb=r=val*%.3f:g=val*%.3f:b=val*%.3f", s.Dim, s.Dim, s.Dim)
	}
	switch s.Turn {
	case 1:
		f += ",transpose=1"
	case 2:
		f += ",transpose=1,transpose=1"
	case 3:
		f += ",transpose=2"
	}
	return f
}

// deviceVideoArgs are the ffmpeg arguments for src -> dst. Both are absolute
// and given as file: URLs, only the file protocol may be read, and no stdin is
// used, so neither a crafted name nor a crafted container can reach further.
func deviceVideoArgs(src, dst string, s DeviceVideoSpec) ([]string, error) {
	if err := s.valid(); err != nil {
		return nil, err
	}
	if !filepath.IsAbs(src) || !filepath.IsAbs(dst) {
		return nil, errors.New("video paths must be absolute")
	}
	return []string{
		"-hide_banner", "-nostdin", "-v", "error",
		"-protocol_whitelist", "file",
		"-format_whitelist", "mov,mp4,matroska,webm,avi,gif,mpegts,flv,asf,mpeg,ogg",
		"-threads", "2",
		"-i", "file:" + src,
		"-t", fmt.Sprint(DeviceVideoSecs),
		"-an", "-sn", "-dn",
		"-vf", s.filter(),
		"-c:v", "libx264", "-preset", "ultrafast", "-profile:v", "baseline", "-level", "4.1",
		"-bf", "0", "-pix_fmt", "yuv420p",
		"-crf", "23", "-maxrate", "4M", "-bufsize", "8M",
		"-fs", fmt.Sprint(DeviceVideoMaxBytes),
		"-f", "h264", "-y", "file:" + dst,
	}, nil
}

// EncodeDeviceVideo encodes src into dst (an absolute path) with the ffmpeg
// found by FFmpegPath. The file appears under its name only when it is
// complete.
func EncodeDeviceVideo(ctx context.Context, src, dst string, s DeviceVideoSpec) error {
	ff := FFmpegPath()
	if ff == "" {
		return errors.New("ffmpeg not found")
	}
	return encodeDeviceVideo(ctx, ff, src, dst, s)
}

func encodeDeviceVideo(ctx context.Context, ff, src, dst string, s DeviceVideoSpec) error {
	src, err := filepath.Abs(src)
	if err != nil {
		return err
	}
	if st, err := os.Stat(src); err != nil || !st.Mode().IsRegular() {
		return fmt.Errorf("video %s not readable", filepath.Base(src))
	}
	if Kind(src) != "video" {
		return fmt.Errorf("%s is not a video", filepath.Base(src))
	}
	tmp := dst + ".part"
	args, err := deviceVideoArgs(src, tmp, s)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}
	os.Remove(tmp)
	ctx, cancel := context.WithTimeout(ctx, deviceVideoTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, ff, args...)
	hideWindow(cmd)
	var stderr limitedBuffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	procjob.Assign(cmd.Process)
	if err := cmd.Wait(); err != nil {
		os.Remove(tmp)
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return fmt.Errorf("ffmpeg: %s", msg)
		}
		return fmt.Errorf("ffmpeg: %w", err)
	}
	st, err := os.Stat(tmp)
	if err != nil || st.Size() == 0 || st.Size() >= DeviceVideoMaxBytes {
		os.Remove(tmp)
		return errors.New("ffmpeg produced no usable video")
	}
	if err := os.Rename(tmp, dst); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// limitedBuffer keeps the first few KB of what it is given.
type limitedBuffer struct{ b bytes.Buffer }

func (l *limitedBuffer) Write(p []byte) (int, error) {
	if room := 4096 - l.b.Len(); room > 0 {
		l.b.Write(p[:min(room, len(p))])
	}
	return len(p), nil
}

func (l *limitedBuffer) String() string { return l.b.String() }

// DeviceVideoName is the cache file name for src encoded with s: it changes
// with the file (size, time) and the settings.
func DeviceVideoName(src string, s DeviceVideoSpec) (string, error) {
	st, err := os.Stat(src)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	fmt.Fprintf(h, "v%s\n%s\n%d\n%d\n%dx%d\n%d\n%.3f\n%d\n", deviceVideoVersion, filepath.Base(src),
		st.Size(), st.ModTime().UnixNano(), s.W, s.H, s.Turn, s.Dim, DeviceVideoFPS)
	return hex.EncodeToString(h.Sum(nil))[:32] + ".h264", nil
}

// CleanDeviceVideos keeps the encoded files in dir within maxBytes: the least
// recently used go first, the file keep (if any) stays. Leftovers of aborted
// encodes (.part) older than an hour are removed too. Only files of the cache
// naming scheme are touched.
func CleanDeviceVideos(dir string, maxBytes int64, keep ...string) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	type file struct {
		path string
		size int64
		mod  time.Time
	}
	var files []file
	var total int64
	for _, e := range ents {
		name := e.Name()
		if e.IsDir() || !isCacheName(name) {
			continue
		}
		fi, err := e.Info()
		if err != nil {
			continue
		}
		p := filepath.Join(dir, name)
		if strings.HasSuffix(name, ".part") {
			if time.Since(fi.ModTime()) > time.Hour {
				os.Remove(p)
			}
			continue
		}
		files = append(files, file{p, fi.Size(), fi.ModTime()})
		total += fi.Size()
	}
	sort.Slice(files, func(i, j int) bool { return files[i].mod.Before(files[j].mod) })
	for _, f := range files {
		if total <= maxBytes {
			return
		}
		if slices.Contains(keep, f.path) {
			continue
		}
		if os.Remove(f.path) == nil {
			total -= f.size
		}
	}
}

// isCacheName reports whether name is 32 hex digits plus .h264 or .h264.part.
func isCacheName(name string) bool {
	stem, ok := strings.CutSuffix(name, ".part")
	if ok && !strings.HasSuffix(stem, ".h264") {
		return false
	}
	stem, ok = strings.CutSuffix(stem, ".h264")
	if !ok || len(stem) != 32 {
		return false
	}
	for _, c := range stem {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

// Touch marks a cached file as used now, so it is cleaned last.
func TouchDeviceVideo(path string) {
	now := time.Now()
	os.Chtimes(path, now, now)
}
