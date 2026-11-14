package cooling

import (
	"errors"
	"fmt"
	"slices"
)

// PartlyLost is the error of Hardware.Speeds when only some outputs cannot be
// read. The speeds it returns alongside are good: the other outputs go on.
type PartlyLost struct {
	IDs []string
	Err error
	// Safe lists those of IDs whose device was left on purpose, with the outputs at a
	// safe duty (see ErrLeftSafe).
	Safe []string
}

func (e *PartlyLost) Error() string { return fmt.Sprintf("%v (%d outputs)", e.Err, len(e.IDs)) }
func (e *PartlyLost) Unwrap() error { return e.Err }

// Route says which device drives which outputs.
type Route struct {
	Device Hardware
	IDs    []string
}

// Join makes several devices one Hardware. A device that fails affects only its
// own outputs, so a fan controller that drops off the bus cannot block the pump.
func Join(routes ...Route) Hardware {
	j := joined{routes: routes, byID: map[string]Hardware{}}
	for _, r := range routes {
		for _, id := range r.IDs {
			j.byID[id] = r.Device
		}
	}
	return j
}

type joined struct {
	routes []Route
	byID   map[string]Hardware
}

func (j joined) device(id string) (Hardware, error) {
	if d, ok := j.byID[id]; ok {
		return d, nil
	}
	return nil, fmt.Errorf("cooling: no device for output %q", id)
}

func (j joined) TakeControl(id string) error {
	d, err := j.device(id)
	if err != nil {
		return err
	}
	return d.TakeControl(id)
}

func (j joined) SetDuty(id string, percent int) error {
	d, err := j.device(id)
	if err != nil {
		return err
	}
	return d.SetDuty(id, percent)
}

func (j joined) Speeds() (map[string]int, error) {
	out := map[string]int{}
	var lost, safe []string
	var errs []error
	for _, r := range j.routes {
		speeds, err := r.Device.Speeds()
		if err != nil {
			lost = append(lost, r.IDs...)
			if errors.Is(err, ErrLeftSafe) {
				safe = append(safe, r.IDs...)
			}
			errs = append(errs, err)
			continue
		}
		for _, id := range r.IDs {
			if v, ok := speeds[id]; ok {
				out[id] = v
			}
		}
	}
	if len(errs) == 0 {
		return out, nil
	}
	slices.Sort(lost)
	return out, &PartlyLost{IDs: lost, Safe: safe, Err: errors.Join(errs...)}
}
