package media

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The test binary doubles as a fake ffmpeg: with CASEHUD_FAKE_FFMPEG set it
// records its arguments and writes a stream, so the encoder is tested without
// the real program.
func TestMain(m *testing.M) {
	if out := os.Getenv("CASEHUD_FAKE_FFMPEG"); out != "" {
		args := os.Args[1:]
		os.WriteFile(out+".args", []byte(strings.Join(args, "\n")), 0o600)
		if os.Getenv("CASEHUD_FAKE_FAIL") != "" {
			os.Stderr.WriteString("boom: invalid data")
			os.Exit(1)
		}
		dst := strings.TrimPrefix(args[len(args)-1], "file:")
		os.WriteFile(dst, []byte{0, 0, 0, 1, 0x67, 66, 0, 0x1f}, 0o600)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func fakeFFmpeg(t *testing.T, fail bool) (ff, argsFile string) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	argsFile = filepath.Join(t.TempDir(), "rec")
	t.Setenv("CASEHUD_FAKE_FFMPEG", argsFile)
	if fail {
		t.Setenv("CASEHUD_FAKE_FAIL", "1")
	}
	return exe, argsFile
}

func TestDeviceVideoFilter(t *testing.T) {
	cases := []struct {
		s    DeviceVideoSpec
		want string
	}{
		{DeviceVideoSpec{1920, 480, 1, 1}, "fps=30,scale=1920:480:force_original_aspect_ratio=increase,crop=1920:480,transpose=1"},
		{DeviceVideoSpec{1920, 480, 3, 1}, "fps=30,scale=1920:480:force_original_aspect_ratio=increase,crop=1920:480,transpose=2"},
		{DeviceVideoSpec{480, 1920, 2, 1}, "fps=30,scale=480:1920:force_original_aspect_ratio=increase,crop=480:1920,transpose=1,transpose=1"},
		{DeviceVideoSpec{480, 1920, 0, 0.35}, "fps=30,scale=480:1920:force_original_aspect_ratio=increase,crop=480:1920,format=rgb24,lutrgb=r=val*0.350:g=val*0.350:b=val*0.350"},
	}
	for _, c := range cases {
		if got := c.s.filter(); got != c.want {
			t.Errorf("%+v:\n got %s\nwant %s", c.s, got, c.want)
		}
	}
}

func TestDeviceVideoSpecValidation(t *testing.T) {
	good := DeviceVideoSpec{1920, 480, 1, 1}
	if good.valid() != nil {
		t.Fatal("good spec rejected")
	}
	for name, s := range map[string]DeviceVideoSpec{
		"odd width": {1921, 480, 1, 1}, "tiny": {8, 8, 0, 1}, "huge": {9000, 480, 0, 1},
		"turn": {1920, 480, 4, 1}, "negative turn": {1920, 480, -1, 1},
		"dim zero": {1920, 480, 0, 0}, "dim big": {1920, 480, 0, 1.5},
	} {
		if s.valid() == nil {
			t.Errorf("%s accepted", name)
		}
		if _, err := deviceVideoArgs("/x/a.mp4", "/x/b", s); err == nil {
			t.Errorf("%s: args built", name)
		}
	}
}

func TestDeviceVideoArgs(t *testing.T) {
	src, dst := filepath.Join(t.TempDir(), "-evil name.mp4"), filepath.Join(t.TempDir(), "out.part")
	args, err := deviceVideoArgs(src, dst, DeviceVideoSpec{1920, 480, 1, 1})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(args, " ")
	for _, want := range []string{"-nostdin", "-protocol_whitelist file", "-i file:" + src, "-profile:v baseline", "-bf 0",
		"-preset ultrafast", "-pix_fmt yuv420p", "-fs 67108864", "-f h264", "-y file:" + dst, "-t 60"} {
		if !strings.Contains(joined, want) {
			t.Errorf("arguments lack %q:\n%s", want, joined)
		}
	}
	// the input name is one argument after -i, prefixed so it cannot be taken for an option
	for i, a := range args {
		if a == "-i" && !strings.HasPrefix(args[i+1], "file:") {
			t.Errorf("input not given as a file: URL: %s", args[i+1])
		}
	}
	// relative paths are refused
	if _, err := deviceVideoArgs("a.mp4", dst, DeviceVideoSpec{1920, 480, 1, 1}); err == nil {
		t.Error("relative input accepted")
	}
}

func TestEncodeDeviceVideoWithFakeFFmpeg(t *testing.T) {
	ff, rec := fakeFFmpeg(t, false)
	dir := t.TempDir()
	src := filepath.Join(dir, "clip.mp4")
	os.WriteFile(src, []byte("x"), 0o600)
	dst := filepath.Join(dir, "cache", "out.h264")
	if err := encodeDeviceVideo(context.Background(), ff, src, dst, DeviceVideoSpec{1920, 480, 1, 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dst); err != nil {
		t.Fatal("output missing")
	}
	if _, err := os.Stat(dst + ".part"); err == nil {
		t.Fatal("temporary file left")
	}
	b, _ := os.ReadFile(rec + ".args")
	if !strings.Contains(string(b), "file:"+src) {
		t.Fatalf("source not passed: %s", b)
	}
	// the stream is written under a temporary name and moved into place at the end
	if !strings.HasSuffix(strings.Split(string(b), "\n")[len(strings.Split(string(b), "\n"))-1], ".part") {
		t.Fatal("encoder writes straight to the final name")
	}
}

func TestEncodeDeviceVideoFailureLeavesNothing(t *testing.T) {
	ff, _ := fakeFFmpeg(t, true)
	dir := t.TempDir()
	src := filepath.Join(dir, "clip.mp4")
	os.WriteFile(src, []byte("x"), 0o600)
	dst := filepath.Join(dir, "out.h264")
	err := encodeDeviceVideo(context.Background(), ff, src, dst, DeviceVideoSpec{1920, 480, 1, 1})
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("err = %v", err)
	}
	if ents, _ := os.ReadDir(dir); len(ents) != 1 {
		t.Fatalf("files left: %v", ents)
	}
	// not a video, not a file
	os.WriteFile(filepath.Join(dir, "a.png"), []byte("x"), 0o600)
	if encodeDeviceVideo(context.Background(), ff, filepath.Join(dir, "a.png"), dst, DeviceVideoSpec{1920, 480, 1, 1}) == nil {
		t.Fatal("image accepted")
	}
	if encodeDeviceVideo(context.Background(), ff, dir, dst, DeviceVideoSpec{1920, 480, 1, 1}) == nil {
		t.Fatal("directory accepted")
	}
}

func TestDeviceVideoNameChangesWithInputs(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a.mp4")
	os.WriteFile(p, []byte("one"), 0o600)
	s := DeviceVideoSpec{1920, 480, 1, 1}
	n1, err := DeviceVideoName(p, s)
	if err != nil || !isCacheName(n1) {
		t.Fatalf("%q %v", n1, err)
	}
	if n2, _ := DeviceVideoName(p, s); n2 != n1 {
		t.Fatal("name not stable")
	}
	s2 := s
	s2.Dim = 0.5
	if n2, _ := DeviceVideoName(p, s2); n2 == n1 {
		t.Fatal("dimming not part of the name")
	}
	s3 := s
	s3.Turn = 3
	if n2, _ := DeviceVideoName(p, s3); n2 == n1 {
		t.Fatal("turn not part of the name")
	}
	os.WriteFile(p, []byte("changed!"), 0o600)
	if n2, _ := DeviceVideoName(p, s); n2 == n1 {
		t.Fatal("file change not part of the name")
	}
}

func TestCleanDeviceVideos(t *testing.T) {
	dir := t.TempDir()
	mk := func(name string, size int, age time.Duration) string {
		p := filepath.Join(dir, name)
		os.WriteFile(p, make([]byte, size), 0o600)
		ts := time.Now().Add(-age)
		os.Chtimes(p, ts, ts)
		return p
	}
	hex := func(c string) string { return strings.Repeat(c, 32) }
	oldest := mk(hex("a")+".h264", 400, 3*time.Hour)
	mid := mk(hex("b")+".h264", 400, 2*time.Hour)
	newest := mk(hex("c")+".h264", 400, time.Hour)
	staleTmp := mk(hex("d")+".h264.part", 10, 2*time.Hour)
	freshTmp := mk(hex("e")+".h264.part", 10, time.Minute)
	foreign := mk("notes.txt", 5000, 5*time.Hour)

	CleanDeviceVideos(dir, 900, "")
	exists := func(p string) bool { _, err := os.Stat(p); return err == nil }
	if exists(oldest) || !exists(mid) || !exists(newest) {
		t.Fatal("did not remove the oldest first")
	}
	if exists(staleTmp) || !exists(freshTmp) {
		t.Fatal("stale partial file handling wrong")
	}
	if !exists(foreign) {
		t.Fatal("removed a file that is not ours")
	}
	// the file in use stays even when it is the oldest and the cap is exceeded
	CleanDeviceVideos(dir, 100, mid)
	if !exists(mid) || exists(newest) {
		t.Fatal("kept the wrong file")
	}
}

func TestIsCacheName(t *testing.T) {
	ok := strings.Repeat("0", 32)
	for name, want := range map[string]bool{
		ok + ".h264": true, ok + ".h264.part": true, ok + ".mp4": false, ok[:31] + ".h264": false,
		strings.Repeat("g", 32) + ".h264": false, "../" + ok + ".h264": false, ok + ".part": false, "": false,
	} {
		if isCacheName(name) != want {
			t.Errorf("%q: %v", name, !want)
		}
	}
}

// With a real ffmpeg: a small test clip becomes a baseline Annex-B stream in
// the requested orientation.
func TestEncodeDeviceVideoRealFFmpeg(t *testing.T) {
	ff := FFmpegPath()
	if ff == "" {
		t.Skip("no ffmpeg")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "in.mp4")
	if out, err := exec.Command(ff, "-v", "error", "-f", "lavfi", "-i", "testsrc=duration=1:size=320x240:rate=10",
		"-pix_fmt", "yuv420p", "-y", src).CombinedOutput(); err != nil {
		t.Skipf("cannot make a test clip: %v %s", err, out)
	}
	dst := filepath.Join(dir, "out.h264")
	if err := EncodeDeviceVideo(context.Background(), src, dst, DeviceVideoSpec{1920, 480, 1, 0.5}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(dst)
	if err != nil || len(b) < 100 {
		t.Fatalf("output %d bytes, %v", len(b), err)
	}
	if !bytes.HasPrefix(b, []byte{0, 0, 0, 1}) && !bytes.HasPrefix(b, []byte{0, 0, 1}) {
		t.Fatal("not an Annex-B stream")
	}
	i := bytes.Index(b, []byte{0, 0, 1, 0x67}) // SPS
	if i < 0 || b[i+4] != 66 {
		t.Fatalf("profile is not baseline (66): % x", b[max(i, 0):min(len(b), max(i, 0)+8)])
	}
	out, _ := exec.Command(ff, "-hide_banner", "-i", dst, "-f", "null", "-").CombinedOutput()
	if !strings.Contains(string(out), "480x1920") {
		t.Fatalf("not a 480x1920 portrait stream:\n%s", out)
	}
}
