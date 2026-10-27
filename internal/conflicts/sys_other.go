//go:build !windows

package conflicts

import "errors"

type noSys struct{}

// NewSystem reports nothing outside Windows.
func NewSystem() System { return noSys{} }

func (noSys) Processes(func(string) bool) ([]Proc, error) { return nil, nil }
func (noSys) PostClose(Proc) (int, error)                 { return 0, errors.New("not supported") }
func (noSys) Terminate(Proc) error                        { return errors.New("not supported") }

// OpenServices is not available outside Windows.
func OpenServices() error { return errors.New("not supported") }
