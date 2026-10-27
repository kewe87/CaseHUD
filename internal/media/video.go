package media

import (
	"bufio"
	"context"
	"fmt"
	"image"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/kewe87/casehud/internal/procjob"
)

// Video decoding shells out to ffmpeg, which is looked up next to the
// executable first and then on PATH (winget install ffmpeg).

const (
	videoFPS      = 30 // decoded rate; playback is limited per panel
	maxVideoSecs  = 60 // frames are kept in memory; bound long clips
	maxVideoFrame = videoFPS * maxVideoSecs
	videoTimeout  = 3 * time.Minute // upper bound for decoding one clip
)

// FFmpegPath returns the ffmpeg binary to use, or "" if none is available.
func FFmpegPath() string {
	name := "ffmpeg"
	if exe, err := os.Executable(); err == nil {
		for _, n := range []string{"ffmpeg.exe", "ffmpeg"} {
			p := filepath.Join(filepath.Dir(exe), n)
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	return ""
}

func loadVideo(path string, w, h int) (*Clip, error) {
	ff := FFmpegPath()
	if ff == "" {
		return nil, fmt.Errorf("videos need ffmpeg: install it (winget install ffmpeg) or put ffmpeg.exe next to casehud")
	}
	// Scale to cover, center-crop, fixed fps, raw RGBA out.
	vf := fmt.Sprintf("fps=%d,scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d", videoFPS, w, h, w, h)
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	// Only the file protocol and known containers, no stdin (see devvideo.go);
	// the timeout keeps a stuck ffmpeg from blocking the read forever.
	ctx, cancel := context.WithTimeout(context.Background(), videoTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, ff, "-nostdin", "-v", "error",
		"-protocol_whitelist", "file",
		"-format_whitelist", "mov,mp4,matroska,webm,avi,gif,mpegts,flv,asf,mpeg,ogg",
		"-i", "file:"+path, "-t", strconv.Itoa(maxVideoSecs),
		"-vf", vf, "-pix_fmt", "rgba", "-f", "rawvideo", "-")
	hideWindow(cmd)
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	procjob.Assign(cmd.Process)
	r := bufio.NewReaderSize(out, w*h*4)
	var frames []Frame
	for len(frames) < maxVideoFrame {
		img := image.NewRGBA(image.Rect(0, 0, w, h))
		if _, err := io.ReadFull(r, img.Pix); err != nil {
			break
		}
		frames = append(frames, Frame{Image: img, Delay: time.Second / videoFPS})
	}
	cmd.Process.Kill()
	cmd.Wait()
	if len(frames) == 0 {
		return nil, fmt.Errorf("ffmpeg produced no frames for %s", filepath.Base(path))
	}
	return newClip(frames)
}
