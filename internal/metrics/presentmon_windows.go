//go:build windows

package metrics

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/kewe87/casehud/internal/procjob"
)

type pmProc struct {
	exe     string
	mu      sync.Mutex
	running bool
	state   string
	next    time.Time // earliest restart
}

func newPMRunner(exe string) pmRunner { return &pmProc{exe: exe, state: "missing"} }

// PresentMonPath finds PresentMon next to the CaseHUD executable
// (PresentMon.exe or the versioned release name).
func PresentMonPath() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	dir := filepath.Dir(exe)
	if p := filepath.Join(dir, "PresentMon.exe"); fileExists(p) {
		return p
	}
	matches, _ := filepath.Glob(filepath.Join(dir, "PresentMon*.exe"))
	if len(matches) > 0 {
		return matches[len(matches)-1]
	}
	return ""
}

func fileExists(p string) bool { _, err := os.Stat(p); return err == nil }

func (p *pmProc) ensure(stats *pmStats) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.running || time.Now().Before(p.next) {
		return
	}
	exe := p.exe
	if exe == "" {
		exe = PresentMonPath()
	}
	if exe == "" {
		p.state = "missing"
		p.next = time.Now().Add(10 * time.Second)
		return
	}
	cmd := exec.Command(exe, "--output_stdout", "--no_console_stats", "--session_name", "CaseHUD",
		"--stop_existing_session", "--track_frame_type", "--track_pc_latency")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	out, err := cmd.StdoutPipe()
	if err != nil {
		p.state = "missing"
		return
	}
	var errBuf strings.Builder
	cmd.Stderr = &limitWriter{w: &errBuf, n: 4096}
	if err := cmd.Start(); err != nil {
		p.state = "missing"
		p.next = time.Now().Add(10 * time.Second)
		return
	}
	procjob.Assign(cmd.Process) // dies with CaseHUD, even on a crash
	p.running, p.state = true, "running"
	go func() {
		stats.consume(bufio.NewReaderSize(out, 64<<10))
		cmd.Wait()
		p.mu.Lock()
		defer p.mu.Unlock()
		p.running = false
		msg := strings.ToLower(errBuf.String())
		if strings.Contains(msg, "access") || strings.Contains(msg, "privilege") || strings.Contains(msg, "admin") || strings.Contains(msg, "performance log users") {
			p.state = "no-permission"
			p.next = time.Now().Add(30 * time.Second)
		} else {
			p.state = "stopped"
			p.next = time.Now().Add(5 * time.Second)
		}
	}()
}

func (p *pmProc) status() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.state
}

func (p *pmProc) foreground() pmFg { return foregroundInfo() }

type limitWriter struct {
	w interface{ WriteString(string) (int, error) }
	n int
}

func (l *limitWriter) Write(b []byte) (int, error) {
	if l.n > 0 {
		s := string(b)
		if len(s) > l.n {
			s = s[:l.n]
		}
		l.n -= len(s)
		l.w.WriteString(s)
	}
	return len(b), nil
}

// foregroundPID is the process owning the focused window.
func foregroundPID() uint32 {
	var pid uint32
	if hwnd, _, _ := procGetForegroundWnd.Call(); hwnd != 0 {
		procGetWindowThreadPI.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	}
	return pid
}
