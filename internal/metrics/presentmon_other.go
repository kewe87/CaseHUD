//go:build !windows

package metrics

type pmNoop struct{}

func newPMRunner(string) pmRunner { return pmNoop{} }
func (pmNoop) ensure(*pmStats)    {}
func (pmNoop) foreground() pmFg   { return pmFg{} }
func (pmNoop) status() string     { return "unsupported" }
