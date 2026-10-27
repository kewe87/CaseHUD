package conflicts

import (
	"errors"
	"sync"
)

// DemoSystem is a made-up process list for the demo mode: two programs of the
// catalog "run", one can be closed, one is a service. Nothing real is touched.
type DemoSystem struct {
	mu    sync.Mutex
	procs []Proc
}

// NewDemoSystem returns the demo list.
func NewDemoSystem() *DemoSystem {
	return &DemoSystem{procs: []Proc{
		{PID: 4242, Exe: "L-Connect 3.exe", Path: `C:\Demo\L-Connect 3.exe`, Created: 1},
		{PID: 4343, Exe: "L-Connect-Service.exe", Path: `C:\Demo\L-Connect-Service.exe`, Created: 2, Service: true, Restricted: true},
	}}
}

func (d *DemoSystem) Processes(want func(string) bool) ([]Proc, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	var out []Proc
	for _, p := range d.procs {
		if want(p.Exe) {
			out = append(out, p)
		}
	}
	return out, nil
}

func (d *DemoSystem) remove(p Proc) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	for i, q := range d.procs {
		if q.PID == p.PID && q.Created == p.Created {
			d.procs = append(d.procs[:i], d.procs[i+1:]...)
			return nil
		}
	}
	return errors.New("the process changed")
}

func (d *DemoSystem) PostClose(p Proc) (int, error) { return 1, d.remove(p) }
func (d *DemoSystem) Terminate(p Proc) error        { return d.remove(p) }
