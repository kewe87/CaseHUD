package coolguard

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
)

// Flag is the command line flag that makes CaseHUD run as the guard of the process
// whose id follows ("--cooling-guard=1234").
const Flag = "--cooling-guard="

type proc struct {
	cmd *exec.Cmd
	in  io.WriteCloser
}

func (p *proc) Write(b []byte) (int, error) { return p.in.Write(b) }
func (p *proc) Close() error                { return p.in.Close() }
func (p *proc) Wait() error                 { return p.cmd.Wait() }
func (p *proc) Kill() error                 { return p.cmd.Process.Kill() }

// Spawn starts the guard of this process: this very executable with Flag. The
// guard must outlive CaseHUD, so it is not tied to its job object and gets no
// window. dataDir is where it writes its log.
func Spawn(dataDir string) (Child, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	args := []string{Flag + strconv.Itoa(os.Getpid()), "--data=" + dataDir}
	// Under a name of its own, so that ending CaseHUD by name does not end the guard. The
	// setup puts that file beside the program. Without one, a copy is made in the data
	// folder. If neither can be started (a policy may forbid programs in a folder), the
	// guard runs from the program itself: better that than none.
	if inst, ok := InstalledGuard(exe); ok {
		if c, err := SpawnProcess(inst, args...); err == nil {
			RemoveCopies(dataDir)
			return c, nil
		}
	}
	if own, err := GuardExe(dataDir, exe); err == nil {
		if c, err := SpawnProcess(own, args...); err == nil {
			return c, nil
		}
	}
	return SpawnProcess(exe, args...)
}

// SpawnProcess starts exe with args as a guard: detached from this process's job,
// without a window, with a pipe for the protocol.
func SpawnProcess(exe string, args ...string) (Child, error) {
	var last error
	for _, breakaway := range []bool{true, false} {
		cmd := exec.Command(exe, args...)
		cmd.SysProcAttr = spawnAttr(breakaway)
		in, err := cmd.StdinPipe()
		if err != nil {
			return nil, err
		}
		if err := cmd.Start(); err != nil {
			in.Close()
			last = fmt.Errorf("starting %s: %w", exe, err)
			continue // a job that does not allow breaking away: stay in it rather than have no guard
		}
		return &proc{cmd: cmd, in: in}, nil
	}
	return nil, errors.Join(errors.New("coolguard: cannot start the guard"), last)
}
