package app

import (
	"context"
	"sync"
	"testing"
	"time"
)

// haltRig is a live engine whose alarms are collected.
type haltRig struct {
	*liveRig
	mu     sync.Mutex
	alarms []CoolingAlarm
}

func newHaltRig(t *testing.T) *haltRig {
	r := &haltRig{liveRig: newLiveRig(t, false)}
	r.e.OnCoolingAlarm = func(a CoolingAlarm) {
		r.mu.Lock()
		r.alarms = append(r.alarms, a)
		r.mu.Unlock()
	}
	r.run(t)
	go r.e.watchHalt(r.e.ctx)
	return r
}

func (r *haltRig) told() []CoolingAlarm {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]CoolingAlarm(nil), r.alarms...)
}

func (r *haltRig) waitFor(n int) bool {
	for range 100 {
		if len(r.told()) >= n {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

func TestAnUnplannedStopIsTold(t *testing.T) {
	r := newHaltRig(t)
	if v := r.e.CoolingView(); v.Alarm != nil {
		t.Fatalf("alarm while running: %+v", v.Alarm)
	}
	r.e.StopUnplanned()
	if !r.waitFor(1) {
		t.Fatal("the stop was not told")
	}
	if got := r.told(); len(got) != 1 || got[0].Kind != coolingAlarmHalted {
		t.Fatalf("%+v", got)
	}
	v := r.e.CoolingView()
	if v.Alarm == nil || v.Alarm.Kind != coolingAlarmHalted {
		t.Fatalf("the cooling tab does not say it: %+v", v.Alarm)
	}
	if a := apiAlarm(v); a == nil || a.Code != "halted" {
		t.Fatalf("api: %+v", a)
	}
	if coolingAlarmGerman[coolingAlarmHalted] == "" || coolingAlarmEnglish[coolingAlarmHalted] == "" {
		t.Fatal("no text in both languages")
	}
}

func TestAContextEndedFromOutsideIsTold(t *testing.T) {
	r := newHaltRig(t)
	r.e.cancelFn()
	if !r.waitFor(1) {
		t.Fatal("not told")
	}
}

func TestThePlannedWaysOfEndingRaiseNoAlarm(t *testing.T) {
	t.Run("stop (tray, --quit, interrupt)", func(t *testing.T) {
		r := newHaltRig(t)
		r.e.Stop()
		time.Sleep(200 * time.Millisecond)
		if got := r.told(); len(got) != 0 {
			t.Fatalf("%+v", got)
		}
		if v := r.e.CoolingView(); v.Alarm != nil && v.Alarm.Kind == coolingAlarmHalted {
			t.Fatal("the view says halted")
		}
	})
	t.Run("cooling switched off in the settings", func(t *testing.T) {
		r := newHaltRig(t)
		if err := r.e.SetModules(Modules{Display: true, Lighting: true}); err != nil {
			t.Fatal(err)
		}
		r.e.Stop()
		time.Sleep(200 * time.Millisecond)
		if got := r.told(); len(got) != 0 {
			t.Fatalf("%+v", got)
		}
	})
	t.Run("preview: no hardware is driven", func(t *testing.T) {
		r := newHaltRig(t)
		c := r.e.coolingSnapshot()
		c.fans = nil
		r.e.setCoolingState(c)
		r.e.StopUnplanned()
		time.Sleep(200 * time.Millisecond)
		if got := r.told(); len(got) != 0 {
			t.Fatalf("%+v", got)
		}
		if v := r.e.CoolingView(); v.Alarm != nil && v.Alarm.Kind == coolingAlarmHalted {
			t.Fatal("the view says halted")
		}
	})
	t.Run("no control running", func(t *testing.T) {
		e := &Engine{Store: &Store{Dir: t.TempDir()}}
		var n int
		e.OnCoolingAlarm = func(CoolingAlarm) { n++ }
		e.cfg = Config{Panels: map[string]PanelConfig{}}
		e.ctx, e.cancelFn = ctxCancel()
		done := make(chan struct{})
		go func() { e.watchHalt(e.ctx); close(done) }()
		e.StopUnplanned()
		<-done
		if n != 0 {
			t.Fatal("alarm without a control")
		}
	})
}

func ctxCancel() (context.Context, context.CancelFunc) {
	return context.WithCancel(context.Background())
}
