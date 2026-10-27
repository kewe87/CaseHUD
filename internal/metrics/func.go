package metrics

// Func is a provider fed by a callback, for sensors that come from devices
// CaseHUD talks to itself (for example a pump head or a temperature hub).
type Func struct {
	// Fn returns the current readings; it must not block for long.
	Fn  func() []Reading
	all []Reading
}

// Refresh implements Provider.
func (f *Func) Refresh() error {
	if f.Fn != nil {
		f.all = f.Fn()
	} else {
		f.all = nil
	}
	return nil
}

// Get implements Provider.
func (f *Func) Get(source string) (Reading, bool) { return match(f.all, source) }

// All implements Provider.
func (f *Func) All() []Reading { return f.all }
