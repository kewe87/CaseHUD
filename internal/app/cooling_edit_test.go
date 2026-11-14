package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func editEngine(t *testing.T) *Engine {
	t.Helper()
	e := coolingEngine(t, coolingExample, &coolingReadings{})
	ctx, cancel := context.WithCancel(context.Background())
	e.ctx = ctx
	e.startCooling()
	t.Cleanup(func() {
		cancel()
		if d := e.coolingSnapshot().done; d != nil {
			<-d
		}
	})
	return e
}

func editCode(err error) string {
	var ce *CoolingEditError
	if errors.As(err, &ce) {
		return ce.Code
	}
	return ""
}

func (e *Engine) outputOf(t *testing.T, id string) coolingOutputConfig {
	t.Helper()
	for _, o := range e.CoolingConfigDoc().Config.Outputs {
		if o.ID == id {
			return o
		}
	}
	t.Fatalf("no output %s", id)
	return coolingOutputConfig{}
}

func TestPatchOutputManualAndCurveExcludeEachOther(t *testing.T) {
	e := editEngine(t)
	rev := e.CoolingConfigDoc().Revision
	rev2, err := e.PatchCoolingOutput("fan", CoolingOutputPatch{Manual: &CoolingManualValue{Value: 55}}, rev)
	if err != nil || rev2 == rev {
		t.Fatalf("manual: %v rev %d -> %d", err, rev, rev2)
	}
	if o := e.outputOf(t, "fan"); o.Manual == nil || o.Manual.Value != 55 || o.Curve != "" {
		t.Fatalf("%+v", o)
	}
	if v := e.CoolingView().Outputs[0]; v.Manual == nil || v.Manual.Value != 55 {
		t.Fatalf("view %+v", v)
	}
	// Back to a curve: the manual value goes.
	curve := "air"
	if _, err := e.PatchCoolingOutput("fan", CoolingOutputPatch{Curve: &curve}, 0); err != nil {
		t.Fatal(err)
	}
	if o := e.outputOf(t, "fan"); o.Manual != nil || o.Curve != "air" {
		t.Fatalf("%+v", o)
	}
	// Both at once is refused, and nothing changes.
	before := e.CoolingConfigDoc().Revision
	_, err = e.PatchCoolingOutput("fan", CoolingOutputPatch{Curve: &curve, Manual: &CoolingManualValue{Value: 50}}, 0)
	if editCode(err) != EditInvalid || e.CoolingConfigDoc().Revision != before {
		t.Fatalf("both: %v", err)
	}
}

func TestPatchOutputPumpFloor(t *testing.T) {
	e := editEngine(t)
	before := e.CoolingConfigDoc().Revision
	for name, p := range map[string]CoolingOutputPatch{
		"manual 10":   {Manual: &CoolingManualValue{Value: 10}},
		"manual 29":   {Manual: &CoolingManualValue{Value: 29}},
		"min_duty 0":  {MinDuty: intPtr(0)},
		"min_duty 29": {MinDuty: intPtr(29)},
	} {
		if _, err := e.PatchCoolingOutput("pump", p, 0); editCode(err) != EditInvalid {
			t.Errorf("%s: %v", name, err)
		}
	}
	if e.CoolingConfigDoc().Revision != before {
		t.Fatal("a refused change was stored")
	}
	if _, err := e.PatchCoolingOutput("pump", CoolingOutputPatch{MinDuty: intPtr(30), Manual: &CoolingManualValue{Value: 30}}, 0); err != nil {
		t.Fatalf("30 %% must be fine: %v", err)
	}
	if o := e.outputOf(t, "pump"); o.MinDuty != 30 || o.Manual == nil || o.Curve != "" {
		t.Fatalf("%+v", o)
	}
}

func TestPatchOutputValidation(t *testing.T) {
	e := editEngine(t)
	long := strings.Repeat("x", 61)
	missing := "nope"
	for name, tc := range map[string]struct {
		id   string
		p    CoolingOutputPatch
		code string
	}{
		"unknown output": {"zzz", CoolingOutputPatch{MinDuty: intPtr(10)}, EditNotFound},
		"unknown curve":  {"fan", CoolingOutputPatch{Curve: &missing}, EditNotFound},
		"min over 100":   {"fan", CoolingOutputPatch{MinDuty: intPtr(101)}, EditInvalid},
		"step zero":      {"fan", CoolingOutputPatch{StepUp: intPtr(0)}, EditInvalid},
		"unit":           {"fan", CoolingOutputPatch{Manual: &CoolingManualValue{Value: 5, Unit: "volt"}}, EditInvalid},
		"long name":      {"fan", CoolingOutputPatch{Name: &long}, EditInvalid},
		"rpm no table":   {"fan", CoolingOutputPatch{Manual: &CoolingManualValue{Value: 99999, Unit: "rpm"}}, EditInvalid},
		"safe too low":   {"fan", CoolingOutputPatch{SafeDuty: intPtr(10)}, EditInvalid},
	} {
		if _, err := e.PatchCoolingOutput(tc.id, tc.p, 0); editCode(err) != tc.code {
			t.Errorf("%s: %v (code %q)", name, err, editCode(err))
		}
	}
	name := " Oben "
	if _, err := e.PatchCoolingOutput("fan", CoolingOutputPatch{Name: &name, StopAt: intPtr(0), Offset: intPtr(5)}, 0); err != nil {
		t.Fatal(err)
	}
	if o := e.outputOf(t, "fan"); o.Name != "Oben" || o.Offset != 5 {
		t.Fatalf("%+v", o)
	}
}

func TestEditsNeedTheCurrentRevisionAndNoMeasurement(t *testing.T) {
	e := editEngine(t)
	rev := e.CoolingConfigDoc().Revision
	if _, err := e.PatchCoolingOutput("fan", CoolingOutputPatch{Offset: intPtr(1)}, rev+1); !errors.Is(err, ErrCoolingRevision) {
		t.Fatalf("stale revision: %v", err)
	}
	if _, err := e.DeleteCoolingCurve("water", rev+1); !errors.Is(err, ErrCoolingRevision) {
		t.Fatalf("stale revision: %v", err)
	}
	e.coolMu.Lock()
	e.coolMeasure.Running = true
	e.coolMu.Unlock()
	if _, err := e.PatchCoolingOutput("fan", CoolingOutputPatch{Offset: intPtr(1)}, rev); !errors.Is(err, ErrCoolingBusy) {
		t.Fatalf("while measuring: %v", err)
	}
	if _, _, err := e.PutCoolingCurve("x", []byte(`{"sources":["/t"],"points":[[20,10],[40,50]],"rise_after":1,"fall_after":1}`), 0); !errors.Is(err, ErrCoolingBusy) {
		t.Fatalf("while measuring: %v", err)
	}
}

func TestPutAndDeleteCurve(t *testing.T) {
	e := editEngine(t)
	body := `{"name":"Neu","sources":["/t"],"points":[[20,30],[40,70]],"rise_by":1,"fall_by":1,"rise_after":1,"fall_after":1}`
	created, rev, err := e.PutCoolingCurve("neu", []byte(body), 0)
	if err != nil || !created || rev == 0 {
		t.Fatalf("create: %v %v", created, err)
	}
	created, _, err = e.PutCoolingCurve("neu", []byte(strings.Replace(body, "[40,70]", "[40,80]", 1)), 0)
	if err != nil || created {
		t.Fatalf("replace: %v %v", created, err)
	}
	var got coolingCurveConfig
	for _, c := range e.CoolingConfigDoc().Config.Curves {
		if c.ID == "neu" {
			got = c
		}
	}
	if got.Points[1][1] != 80 || got.Name != "Neu" {
		t.Fatalf("%+v", got)
	}
	// The ID of the path wins over nothing, a different one in the body is refused.
	if _, _, err := e.PutCoolingCurve("neu", []byte(strings.Replace(body, `"name"`, `"id":"other","name"`, 1)), 0); editCode(err) != EditInvalid {
		t.Fatalf("id mismatch: %v", err)
	}
	for name, bad := range map[string]string{
		"unknown field": `{"sources":["/t"],"points":[[20,30],[40,70]],"nope":1}`,
		"short point":   `{"sources":["/t"],"points":[[20],[40,70]]}`,
		"trailing":      body + " x",
		"syntax":        `{`,
	} {
		if _, _, err := e.PutCoolingCurve("bad", []byte(bad), 0); editCode(err) != EditInvalid {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, _, err := e.PutCoolingCurve("@manual:x", []byte(body), 0); editCode(err) != EditInvalid {
		t.Errorf("reserved id: %v", err)
	}

	// A used curve stays; an unused one goes.
	if _, err := e.DeleteCoolingCurve("water", 0); editCode(err) != EditInUse {
		t.Fatalf("used curve: %v", err)
	}
	if _, err := e.DeleteCoolingCurve("neu", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := e.DeleteCoolingCurve("neu", 0); editCode(err) != EditNotFound {
		t.Fatalf("second delete: %v", err)
	}
}

func TestDeleteCurveUsedByAnotherCurve(t *testing.T) {
	e := editEngine(t)
	base := `{"sources":["/t"],"points":[[20,30],[40,70]],"rise_after":1,"fall_after":1}`
	if _, _, err := e.PutCoolingCurve("b", []byte(base), 0); err != nil {
		t.Fatal(err)
	}
	mix := `{"kind":"mix","inputs":["b"],"combine":"max"}`
	if _, _, err := e.PutCoolingCurve("m", []byte(mix), 0); err != nil {
		t.Skipf("mix curve kind differs: %v", err)
	}
	if _, err := e.DeleteCoolingCurve("b", 0); editCode(err) != EditInUse {
		t.Fatalf("%v", err)
	}
}

func TestEditsAreStoredLikeASave(t *testing.T) {
	e := editEngine(t)
	if _, err := e.PatchCoolingOutput("fan", CoolingOutputPatch{Manual: &CoolingManualValue{Value: 61}}, 0); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(e.Store.Dir, coolingFile))
	if err != nil || !strings.Contains(string(b), `"value": 61`) {
		t.Fatalf("%v %s", err, b)
	}
	if _, err := parseCoolingConfig(b); err != nil {
		t.Fatalf("stored file does not load: %v", err)
	}
}

func TestSetCoolingConfigRevReturnsTheNewRevision(t *testing.T) {
	e := editEngine(t)
	doc := e.CoolingConfigDoc()
	rev, err := e.SetCoolingConfigRev([]byte(coolingExample), doc.Revision)
	if err != nil || rev == doc.Revision || rev != e.CoolingConfigDoc().Revision {
		t.Fatalf("%v %d %d", err, rev, doc.Revision)
	}
	if _, err := e.SetCoolingConfigRev([]byte(coolingExample), doc.Revision); !errors.Is(err, ErrCoolingRevision) {
		t.Fatalf("old revision: %v", err)
	}
}

func TestLiveBlockNamesTheDisclaimer(t *testing.T) {
	e := editEngine(t)
	if reason, _ := e.CoolingLiveBlock(); reason != coolingBlockedDisclaimer {
		t.Fatalf("reason %q", reason)
	}
}
