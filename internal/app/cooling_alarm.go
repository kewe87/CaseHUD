package app

import (
	"context"
	"log"
	"strings"
	"time"
)

const (
	coolingStaleAfter  = 30 * time.Second // a loop without a tick for this long has stopped
	coolingAlarmAfter  = 15 * time.Second // what passes faster is no reason to interrupt anybody
	coolingAlarmRepeat = 10 * time.Minute // the same alarm comes back this late at the earliest
	coolingAlarmEvery  = 2 * time.Second
)

// Kinds of CoolingAlarm. The GUI has a text for each.
const (
	coolingAlarmPump        = "pump"
	coolingAlarmPumpUnknown = "pumpUnknown"
	coolingAlarmStale       = "stale"
	coolingAlarmConfig      = "config"
	coolingAlarmDevice      = "device"
	coolingAlarmWrite       = "write"
	coolingAlarmSensor      = "sensor"
	coolingAlarmStalled     = "stalled"
	coolingAlarmService     = "service"
	coolingAlarmConfigCopy  = "configBackup"
	coolingAlarmHalted      = "halted"
	coolingAlarmEmergency   = "emergency"
)

// CoolingAlarm is the worst thing wrong with the cooling at the moment. The zero
// value means nothing is.
type CoolingAlarm struct {
	Kind    string `json:"kind"`
	Name    string `json:"name,omitempty"`    // the output it concerns
	Program string `json:"program,omitempty"` // with a block by a program
}

// coolingAlarmOf reads the alarm out of a view. lastTick is zero until the loop
// has run once: before that the outputs have no state to judge.
//
// The order is by what it means for the hardware: a standing pump first, then what
// leaves every fan unattended, then what concerns a single one.
func coolingAlarmOf(v CoolingView, lastTick, now time.Time) *CoolingAlarm {
	if v.Error != "" {
		return &CoolingAlarm{Kind: coolingAlarmConfig}
	}
	// Least urgent: the control runs, on the safety copy of its settings. It is
	// told whatever the mode, and anything that concerns the hardware comes first.
	var restored *CoolingAlarm
	if v.Restored {
		restored = &CoolingAlarm{Kind: coolingAlarmConfigCopy, Name: v.RestoredFile}
	}
	started := !lastTick.IsZero()
	find := func(hit func(CoolingOutputView) bool) (string, bool) {
		if !started {
			return "", false
		}
		for _, o := range v.Outputs {
			if hit(o) {
				if o.Name != "" {
					return o.Name, true
				}
				return o.ID, true
			}
		}
		return "", false
	}
	// A pump that stands is told in a preview too: the speed is read from the real
	// pump there, and nobody else is going to notice.
	if name, ok := find(func(o CoolingOutputView) bool { return o.PumpDown }); ok {
		return &CoolingAlarm{Kind: coolingAlarmPump, Name: name}
	}
	if !v.Live {
		return restored // a preview drives nothing
	}
	if started && now.Sub(lastTick) >= coolingStaleAfter {
		return &CoolingAlarm{Kind: coolingAlarmStale}
	}
	if v.Blocked != "" {
		return &CoolingAlarm{Kind: v.Blocked, Program: v.Program}
	}
	if started && v.SensorsDown {
		return &CoolingAlarm{Kind: coolingAlarmService}
	}
	// Readings that are old or frozen are the cause of the lost sensors and of a pump
	// whose speed cannot be read, so they are named first.
	if v.SensorsStale {
		if name, ok := find(func(o CoolingOutputView) bool { return o.SensorLost }); ok {
			return &CoolingAlarm{Kind: coolingAlarmSensor, Name: name}
		}
	}
	for _, c := range []struct {
		kind string
		hit  func(CoolingOutputView) bool
	}{
		{coolingAlarmEmergency, func(o CoolingOutputView) bool { return o.Emergency }},
		{coolingAlarmPumpUnknown, func(o CoolingOutputView) bool { return o.Pump && o.RPM == nil }},
		{coolingAlarmDevice, func(o CoolingOutputView) bool { return o.DeviceLost }},
		{coolingAlarmWrite, func(o CoolingOutputView) bool { return o.WriteFail }},
		{coolingAlarmSensor, func(o CoolingOutputView) bool { return o.SensorLost }},
		{coolingAlarmStalled, func(o CoolingOutputView) bool { return o.Stalled }},
	} {
		if name, ok := find(c.hit); ok {
			return &CoolingAlarm{Kind: c.kind, Name: name}
		}
	}
	return restored
}

// Message is the alarm as a sentence, for where the GUI's texts are out of reach.
func (a CoolingAlarm) Message(german bool) string {
	texts := coolingAlarmEnglish
	if german {
		texts = coolingAlarmGerman
	}
	text, ok := texts[a.Kind]
	if !ok {
		text = texts[coolingAlarmStale]
	}
	return strings.NewReplacer("{n}", a.Name, "{p}", a.Program).Replace(text)
}

var coolingAlarmGerman = map[string]string{
	coolingAlarmPump:         "{n} läuft nicht, vermutlich defekt. Last reduzieren und Pumpe prüfen.",
	coolingAlarmPumpUnknown:  "Drehzahl von {n} nicht lesbar. Pumpenbetrieb nicht prüfbar.",
	coolingAlarmStale:        "Lüfterregelung reagiert nicht. Lüfter behalten den letzten Wert.",
	coolingAlarmConfig:       "Lüfterregelung inaktiv: Einstellungen konnten nicht geladen werden.",
	coolingBlockedProgram:    "{p} läuft, keine Regelung durch CaseHUD. Lüfter mindestens auf sicherer Drehzahl, bis {p} übernimmt oder beendet wird.",
	coolingBlockedCheck:      "Keine Regelung: Prüfung auf andere Lüfterprogramme nicht möglich.",
	coolingBlockedDisclaimer: "Keine Regelung, bis der Haftungshinweis bestätigt ist.",
	coolingAlarmDevice:       "Lüfter-Controller nicht erreichbar. {n} und weitere Lüfter werden nicht geregelt.",
	coolingAlarmWrite:        "{n} nimmt keine Drehzahl an.",
	coolingAlarmSensor:       "Sensor für {n} fehlt oder liefert keine neuen Werte. Lüfter auf sicherer Drehzahl.",
	coolingAlarmStalled:      "{n} steht trotz Drehzahlvorgabe.",
	coolingAlarmService:      "Sensordienst antwortet nicht: Temperaturen und Drehzahlen fehlen. Lüfter auf sicherer Drehzahl. Dienst „CaseHUD Sensors“ in den Windows-Diensten prüfen.",
	coolingAlarmConfigCopy:   "cooling.json beschädigt, Betrieb mit der letzten Sicherung. Defekte Datei als {n} im Datenordner. Einstellungen prüfen und neu speichern.",
	coolingAlarmHalted:       "Lüfterregelung angehalten, CaseHUD läuft weiter. Lüfter (und Pumpe) werden nicht geregelt. CaseHUD neu starten.",
	coolingAlarmEmergency:    "Übertemperatur: {n} auf voller Drehzahl. Last reduzieren und Kühlung prüfen.",
}

var coolingAlarmEnglish = map[string]string{
	coolingAlarmPump:         "{n} is not running, probably defective. Reduce load and check the pump.",
	coolingAlarmPumpUnknown:  "Speed of {n} not readable. Pump operation cannot be verified.",
	coolingAlarmStale:        "Fan control not responding. Fans keep their last value.",
	coolingAlarmConfig:       "Fan control inactive: settings could not be loaded.",
	coolingBlockedProgram:    "{p} is running, CaseHUD is not controlling. Fans stay at safe speed or above until {p} takes over or exits.",
	coolingBlockedCheck:      "No control: check for other fan programs not possible.",
	coolingBlockedDisclaimer: "No control until the disclaimer is confirmed.",
	coolingAlarmDevice:       "Fan controller not reachable. {n} and other fans are not controlled.",
	coolingAlarmWrite:        "{n} does not accept a speed.",
	coolingAlarmSensor:       "Sensor for {n} missing or not delivering new values. Fans at safe speed.",
	coolingAlarmStalled:      "{n} is stopped despite a speed target.",
	coolingAlarmService:      "Sensor service not responding: temperatures and speeds are missing. Fans at safe speed. Check the “CaseHUD Sensors” service in Windows Services.",
	coolingAlarmConfigCopy:   "cooling.json damaged, running on the last backup. Damaged file kept as {n} in the data folder. Check the settings and save again.",
	coolingAlarmHalted:       "Fan control stopped while CaseHUD keeps running. Fans (and pump) are not controlled. Restart CaseHUD.",
	coolingAlarmEmergency:    "Overheating: {n} at full speed. Reduce load and check the cooling.",
}

// alarmWatch decides when an alarm is worth telling. It holds no clock of its own.
type alarmWatch struct {
	seen   CoolingAlarm
	since  time.Time
	shown  bool // an alarm has been told and its end has not
	toldAt map[CoolingAlarm]time.Time
}

// see takes the current alarm. It returns what to tell, if anything: an alarm once
// it has lasted, or the zero alarm to say that the told one is over.
func (w *alarmWatch) see(a *CoolingAlarm, now time.Time) (CoolingAlarm, bool) {
	var cur CoolingAlarm
	if a != nil {
		cur = *a
	}
	if cur != w.seen {
		w.seen, w.since = cur, now
		if w.shown && cur == (CoolingAlarm{}) {
			w.shown = false
			return CoolingAlarm{}, true
		}
		return CoolingAlarm{}, false
	}
	if cur == (CoolingAlarm{}) || now.Sub(w.since) < coolingAlarmAfter {
		return CoolingAlarm{}, false
	}
	if at, told := w.toldAt[cur]; told && (w.shown || now.Sub(at) < coolingAlarmRepeat) {
		return CoolingAlarm{}, false
	}
	if w.toldAt == nil {
		w.toldAt = map[CoolingAlarm]time.Time{}
	}
	w.toldAt[cur] = now
	w.shown = true
	return cur, true
}

// currentCoolingAlarm is the alarm of the moment. Without any control there is
// nothing to look at, and the full view would search the USB bus each time.
func (e *Engine) currentCoolingAlarm() *CoolingAlarm {
	if !e.coolingOn() {
		return nil
	}
	if _, restored := e.crash.restoredFile(); !restored {
		if c := e.coolingSnapshot(); c.ctrl == nil && c.errMsg == "" {
			return nil
		}
	}
	return e.CoolingView().Alarm
}

// watchCoolingAlarms tells of alarms for as long as the engine runs, whichever
// page is open and whether or not a window is.
func (e *Engine) watchCoolingAlarms(ctx context.Context) {
	var watch alarmWatch
	t := time.NewTicker(coolingAlarmEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-t.C:
			a, tell := watch.see(e.currentCoolingAlarm(), now)
			if !tell {
				continue
			}
			if a.Kind == "" {
				log.Print("cooling: alarm over")
			} else {
				log.Printf("cooling: alarm: %s", a.Message(false))
			}
			if e.OnCoolingAlarm != nil {
				e.OnCoolingAlarm(a)
			}
		}
	}
}

// watchHalt tells when the engine ended although the program goes on: after
// ctx ended and Stop was not the reason. The ticker of watchCoolingAlarms is gone
// by then, so this one speaks at once. With the cooling off, no control running
// or only a preview (no hardware is driven, in the demo as well) there is
// nothing to tell.
func (e *Engine) watchHalt(ctx context.Context) {
	<-ctx.Done()
	if c := e.coolingSnapshot(); e.stopPlanned.Load() || !e.coolingOn() || c.ctrl == nil || c.fans == nil {
		return
	}
	e.halted.Store(true)
	a := CoolingAlarm{Kind: coolingAlarmHalted}
	log.Printf("cooling: alarm: %s", a.Message(false))
	if e.OnCoolingAlarm != nil {
		e.OnCoolingAlarm(a)
	}
}
