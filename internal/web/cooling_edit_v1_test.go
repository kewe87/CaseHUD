package web

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/kewe87/casehud/internal/app"
)

// v1h is v1 with extra headers (If-Match).
func v1h(t *testing.T, ts *httptest.Server, method, path, body string, hdr map[string]string, out any) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(method, ts.URL+path, strings.NewReader(body))
	req.Header.Set("X-Token", "client")
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b := readAll(resp)
	if out != nil && len(b) > 0 {
		if err := json.Unmarshal(b, out); err != nil {
			t.Fatalf("%s %s: bad JSON %q", method, path, b)
		}
	}
	resp.Body = nopBody(b)
	return resp
}

func ifm(rev int64) map[string]string {
	return map[string]string{"If-Match": strconv.FormatInt(rev, 10)}
}

func coolingDoc(t *testing.T, ts *httptest.Server) app.CoolingConfigDoc {
	t.Helper()
	var d app.CoolingConfigDoc
	resp := v1(t, ts, "GET", "/api/v1/cooling/config", "", &d)
	wantStatus(t, resp, 200)
	if resp.Header.Get("ETag") != strconv.FormatInt(d.Revision, 10) {
		t.Fatalf("ETag %q, revision %d", resp.Header.Get("ETag"), d.Revision)
	}
	return d
}

func TestV1CoolingConfigReadAndReplace(t *testing.T) {
	_, ts := testServer(t)
	configureCooling(t, ts, demoCoolingAt("60"))
	d := coolingDoc(t, ts)
	if len(d.Config.Outputs) != 2 || len(d.Config.Curves) != 1 || d.Config.Mode != "preview" || d.Revision == 0 {
		t.Fatalf("%+v", d)
	}
	cfg := demoCoolingAt("25")

	wantError(t, v1(t, ts, "PUT", "/api/v1/cooling/config", cfg, nil), 428, "precondition_required")
	wantError(t, v1h(t, ts, "PUT", "/api/v1/cooling/config", cfg, ifm(d.Revision+1), nil), 409, "revision_mismatch")
	wantError(t, v1h(t, ts, "PUT", "/api/v1/cooling/config", cfg, map[string]string{"If-Match": "abc"}, nil), 400, "bad_request")
	wantError(t, v1h(t, ts, "PUT", "/api/v1/cooling/config", `{"outputs":[],"nope":1}`, ifm(d.Revision), nil), 400, "invalid_value")
	wantError(t, v1h(t, ts, "PUT", "/api/v1/cooling/config", `{`, ifm(d.Revision), nil), 400, "invalid_value")
	wantError(t, v1h(t, ts, "PUT", "/api/v1/cooling/config", `{"outputs":[]}`+strings.Repeat(" ", 2<<20), ifm(d.Revision), nil), 413, "payload_too_large")

	var done v1Revision
	resp := v1h(t, ts, "PUT", "/api/v1/cooling/config", cfg, ifm(d.Revision), &done)
	wantStatus(t, resp, 200)
	if done.Revision == d.Revision || resp.Header.Get("ETag") != strconv.FormatInt(done.Revision, 10) {
		t.Fatalf("%+v %v", done, resp.Header)
	}
	if now := coolingDoc(t, ts); now.Revision != done.Revision || now.Config.Curves[0].Points[0][1] != 25 {
		t.Fatalf("%+v", now)
	}
	// The old revision is refused now.
	wantError(t, v1h(t, ts, "PUT", "/api/v1/cooling/config", cfg, ifm(d.Revision), nil), 409, "revision_mismatch")
}

func TestV1CoolingOutputPatch(t *testing.T) {
	_, ts := testServer(t)
	configureCooling(t, ts, demoCoolingAt("60"))
	d := coolingDoc(t, ts)

	var done v1Revision
	wantStatus(t, v1h(t, ts, "PATCH", "/api/v1/cooling/outputs/port1-fan1", `{"manual":{"value":55}}`, ifm(d.Revision), &done), 200)
	if done.Revision == d.Revision {
		t.Fatal("revision did not change")
	}
	v := getCooling(t, ts)
	if o := v.Outputs[0]; o.Mode != "manual" || o.Manual == nil || o.Manual.Value != 55 || o.Unregulated {
		t.Fatalf("%+v", o)
	}
	if v.Mode != "preview" || v.Revision != done.Revision {
		t.Fatalf("mode %q revision %d", v.Mode, v.Revision)
	}
	// Back to the curve.
	wantStatus(t, v1(t, ts, "PATCH", "/api/v1/cooling/outputs/port1-fan1", `{"curve":"c","min_duty":20}`, nil), 200)
	if o := coolingDoc(t, ts).Config.Outputs[0]; o.Manual != nil || o.MinDuty != 20 {
		t.Fatalf("%+v", o)
	}
	// Refused.
	wantError(t, v1h(t, ts, "PATCH", "/api/v1/cooling/outputs/port1-fan1", `{"min_duty":1}`, ifm(d.Revision), nil), 409, "revision_mismatch")
	wantError(t, v1(t, ts, "PATCH", "/api/v1/cooling/outputs/pump", `{"manual":{"value":29}}`, nil), 400, "invalid_value")
	wantError(t, v1(t, ts, "PATCH", "/api/v1/cooling/outputs/pump", `{"min_duty":10}`, nil), 400, "invalid_value")
	wantError(t, v1(t, ts, "PATCH", "/api/v1/cooling/outputs/pump", `{"manual":{"value":0}}`, nil), 400, "invalid_value")
	wantError(t, v1(t, ts, "PATCH", "/api/v1/cooling/outputs/pump", `{"curve":"c","manual":{"value":50}}`, nil), 400, "invalid_value")
	wantError(t, v1(t, ts, "PATCH", "/api/v1/cooling/outputs/nope", `{"min_duty":1}`, nil), 404, "not_found")
	wantError(t, v1(t, ts, "PATCH", "/api/v1/cooling/outputs/pump", `{"curve":"zzz"}`, nil), 404, "not_found")
	wantError(t, v1(t, ts, "PATCH", "/api/v1/cooling/outputs/pump", `{"rpm_sensor":"x"}`, nil), 400, "bad_request")
	wantError(t, v1(t, ts, "PATCH", "/api/v1/cooling/outputs/pump", `{`, nil), 400, "bad_request")
	wantError(t, v1(t, ts, "PATCH", "/api/v1/cooling/outputs/pump", `{"name":"`+strings.Repeat("x", 2<<20)+`"}`, nil), 413, "payload_too_large")
	// The pump is still on its floor.
	if o := coolingDoc(t, ts).Config.Outputs[1]; o.MinDuty < 30 {
		t.Fatalf("pump minimum %d", o.MinDuty)
	}
}

func TestV1CoolingCurves(t *testing.T) {
	_, ts := testServer(t)
	configureCooling(t, ts, demoCoolingAt("60"))
	curve := `{"name":"Leise","sources":["Board/Temperatures/Coolant"],"points":[[20,30],[40,60]],"rise_by":1,"fall_by":1,"rise_after":1,"fall_after":1}`
	wantStatus(t, v1(t, ts, "PUT", "/api/v1/cooling/curves/leise", curve, nil), 201)
	wantStatus(t, v1(t, ts, "PUT", "/api/v1/cooling/curves/leise", strings.Replace(curve, "[40,60]", "[40,70]", 1), nil), 200)
	if cs := coolingDoc(t, ts).Config.Curves; len(cs) != 2 || cs[1].ID != "leise" || cs[1].Points[1][1] != 70 {
		t.Fatalf("%+v", cs)
	}
	wantError(t, v1(t, ts, "PUT", "/api/v1/cooling/curves/x", `{"nope":1}`, nil), 400, "invalid_value")
	wantError(t, v1(t, ts, "PUT", "/api/v1/cooling/curves/x", strings.Replace(curve, `"name"`, `"id":"y","name"`, 1), nil), 400, "invalid_value")
	wantError(t, v1(t, ts, "PUT", "/api/v1/cooling/curves/x", `{"sources":["a"],"points":[[1]]}`, nil), 400, "invalid_value")
	wantError(t, v1h(t, ts, "PUT", "/api/v1/cooling/curves/x", curve, ifm(1), nil), 409, "revision_mismatch")

	wantError(t, v1(t, ts, "DELETE", "/api/v1/cooling/curves/c", "", nil), 409, "curve_in_use")
	wantError(t, v1(t, ts, "DELETE", "/api/v1/cooling/curves/zzz", "", nil), 404, "not_found")
	wantStatus(t, v1(t, ts, "DELETE", "/api/v1/cooling/curves/leise", "", nil), 200)
	if cs := coolingDoc(t, ts).Config.Curves; len(cs) != 1 {
		t.Fatalf("%+v", cs)
	}
}

func TestV1CoolingPresetsCRUD(t *testing.T) {
	_, ts := testServer(t)
	configureCooling(t, ts, demoCoolingAt("60"))
	var info app.CoolingPresetInfo
	wantStatus(t, v1(t, ts, "POST", "/api/v1/cooling/presets", `{"name":"Tag"}`, &info), 201)
	if info.ID == "" || info.Name != "Tag" {
		t.Fatalf("%+v", info)
	}
	var list app.CoolingPresetsView
	wantStatus(t, v1(t, ts, "GET", "/api/v1/cooling/presets", "", &list), 200)
	if len(list.Presets) != 1 || list.Active != info.ID {
		t.Fatalf("%+v", list)
	}
	wantStatus(t, v1(t, ts, "PUT", "/api/v1/cooling/presets/"+info.ID, `{"name":"Nacht"}`, nil), 204)
	configureCooling(t, ts, demoCoolingAt("30"))
	wantStatus(t, v1(t, ts, "POST", "/api/v1/cooling/presets/"+info.ID+"/overwrite", ``, nil), 204)
	wantError(t, v1(t, ts, "POST", "/api/v1/cooling/presets", `{"name":""}`, nil), 400, "bad_request")
	wantError(t, v1(t, ts, "POST", "/api/v1/cooling/presets", `{"name":"x","admin":1}`, nil), 400, "bad_request")
	wantError(t, v1(t, ts, "PUT", "/api/v1/cooling/presets/nope", `{"name":"x"}`, nil), 404, "not_found")
	wantError(t, v1(t, ts, "POST", "/api/v1/cooling/presets/nope/overwrite", ``, nil), 404, "not_found")
	wantStatus(t, v1(t, ts, "DELETE", "/api/v1/cooling/presets/"+info.ID, "", nil), 204)
	wantError(t, v1(t, ts, "DELETE", "/api/v1/cooling/presets/"+info.ID, "", nil), 404, "not_found")
}

func TestV1CoolingMode(t *testing.T) {
	_, ts := testServer(t)
	configureCooling(t, ts, demoCoolingAt("60"))
	wantError(t, v1(t, ts, "PUT", "/api/v1/cooling/mode", `{"mode":"fast"}`, nil), 400, "invalid_value")
	wantError(t, v1(t, ts, "PUT", "/api/v1/cooling/mode", `{}`, nil), 400, "invalid_value")
	var m v1ModeChanged
	wantStatus(t, v1(t, ts, "PUT", "/api/v1/cooling/mode", `{"mode":"preview"}`, &m), 200)
	if m.Mode != "preview" || m.Revision == 0 {
		t.Fatalf("%+v", m)
	}
	// The conflict catalog of the test machine lists fan programs as running:
	// live mode would send nothing and is refused.
	wantError(t, v1(t, ts, "PUT", "/api/v1/cooling/mode", `{"mode":"live"}`, nil), 409, "blocked")
	if v := getCooling(t, ts); v.Mode != "preview" {
		t.Fatalf("mode %q after a refused switch", v.Mode)
	}
}

func TestV1CoolingMeasure(t *testing.T) {
	_, ts := testServer(t)
	configureCooling(t, ts, demoCoolingAt("60"))
	wantError(t, v1(t, ts, "POST", "/api/v1/cooling/measure", `{"id":"port1-fan1"}`, nil), 400, "confirmation_required")
	wantError(t, v1(t, ts, "POST", "/api/v1/cooling/measure", `{"id":"port1-fan1","confirm":false}`, nil), 400, "confirmation_required")
	wantError(t, v1(t, ts, "POST", "/api/v1/cooling/measure", `{"confirm":true}`, nil), 400, "bad_request")
	wantError(t, v1(t, ts, "POST", "/api/v1/cooling/measure", `{"id":"port1-fan1","all":true,"confirm":true}`, nil), 400, "bad_request")
	wantError(t, v1(t, ts, "POST", "/api/v1/cooling/measure-pump", `{"id":"pump"}`, nil), 400, "confirmation_required")
	wantError(t, v1(t, ts, "POST", "/api/v1/cooling/measure-pump", `{"confirm":true}`, nil), 400, "bad_request")
	wantError(t, v1(t, ts, "POST", "/api/v1/cooling/measure", `{"id":"nope","confirm":true}`, nil), 409, "conflict")
	wantError(t, v1(t, ts, "POST", "/api/v1/cooling/recheck-pump", `{"id":"pump"}`, nil), 409, "conflict")
	wantError(t, v1(t, ts, "POST", "/api/v1/cooling/recheck-pump", `{}`, nil), 400, "bad_request")
	wantError(t, v1(t, ts, "DELETE", "/api/v1/cooling/measure?keep=all", "", nil), 400, "bad_request")

	// The demo measures in live mode only. The API refuses live while the
	// conflict catalog lists a running fan program, so the window's file is used.
	configureCooling(t, ts, `{"mode":"live","curves":[
 {"id":"c","name":"C","sources":["Board/Temperatures/Coolant"],"points":[[20,30],[40,80]],"rise_by":1,"fall_by":1,"rise_after":1,"fall_after":1}
],"outputs":[{"id":"port1-fan1","name":"Radiator","curve":"c","step_up":100,"step_down":100,"safe_duty":70}]}`)
	var mv app.CoolingMeasureView
	wantStatus(t, v1(t, ts, "GET", "/api/v1/cooling/measure", "", &mv), 200)
	if mv.Running {
		t.Fatalf("%+v", mv)
	}
	wantStatus(t, v1(t, ts, "POST", "/api/v1/cooling/measure", `{"id":"port1-fan1","confirm":true}`, &mv), 202)
	if !mv.Running || mv.ID != "port1-fan1" {
		t.Fatalf("%+v", mv)
	}
	// While it runs, the settings cannot be changed.
	d := coolingDoc(t, ts)
	wantError(t, v1h(t, ts, "PUT", "/api/v1/cooling/config", demoCoolingAt("25"), ifm(d.Revision), nil), 409, "busy")
	wantError(t, v1(t, ts, "PATCH", "/api/v1/cooling/outputs/port1-fan1", `{"min_duty":5}`, nil), 409, "busy")
	wantError(t, v1(t, ts, "PUT", "/api/v1/cooling/mode", `{"mode":"preview"}`, nil), 409, "busy")
	wantError(t, v1(t, ts, "POST", "/api/v1/cooling/measure", `{"id":"port1-fan1","confirm":true}`, nil), 409, "busy")
	wantStatus(t, v1(t, ts, "DELETE", "/api/v1/cooling/measure", "", nil), 204)
}

func TestV1CoolingEditsWhenCoolingIsOff(t *testing.T) {
	s, ts := testServer(t)
	configureCooling(t, ts, demoCoolingAt("60"))
	if err := s.Engine.SetModules(app.Modules{Display: true, Lighting: true}); err != nil {
		t.Fatal(err)
	}
	wantError(t, v1h(t, ts, "PUT", "/api/v1/cooling/config", demoCoolingAt("25"), ifm(1), nil), 409, "cooling_off")
	wantError(t, v1(t, ts, "PATCH", "/api/v1/cooling/outputs/pump", `{"min_duty":40}`, nil), 409, "cooling_off")
	wantError(t, v1(t, ts, "PUT", "/api/v1/cooling/curves/x", `{"sources":["a"],"points":[[1,2],[3,4]]}`, nil), 409, "cooling_off")
	wantError(t, v1(t, ts, "PUT", "/api/v1/cooling/mode", `{"mode":"preview"}`, nil), 409, "cooling_off")
	wantError(t, v1(t, ts, "POST", "/api/v1/cooling/measure", `{"all":true,"confirm":true}`, nil), 409, "cooling_off")
	wantStatus(t, v1(t, ts, "GET", "/api/v1/cooling/config", "", nil), 200) // reading stays
}

// The pump can only be released in the window: nothing in /api/v1 does it.
func TestV1HasNoPumpRelease(t *testing.T) {
	_, ts := testServer(t)
	for _, p := range []string{"/api/v1/cooling/release-pump", "/api/v1/cooling/measure-all"} {
		wantError(t, v1(t, ts, "POST", p, `{"id":"pump"}`, nil), 404, "not_found")
	}
}

func TestV1CoolingViewTellsWhatToDo(t *testing.T) {
	_, ts := testServer(t)
	var raw map[string]json.RawMessage
	configureCooling(t, ts, demoCoolingAt("60"))
	wantStatus(t, v1(t, ts, "GET", "/api/v1/cooling", "", &raw), 200)
	var outs []map[string]json.RawMessage
	json.Unmarshal(raw["outputs"], &outs)
	for _, k := range []string{"mode", "unregulated", "calibrating", "unproven", "notReleased", "notMatched", "noSource", "problem"} {
		if _, ok := outs[0][k]; !ok {
			t.Errorf("output field %s missing", k)
		}
	}
	for _, k := range []string{"mode", "revision"} {
		if _, ok := raw[k]; !ok {
			t.Errorf("field %s missing", k)
		}
	}
}

func readAll(resp *http.Response) []byte {
	b, _ := io.ReadAll(resp.Body)
	return b
}

func nopBody(b []byte) io.ReadCloser { return io.NopCloser(strings.NewReader(string(b))) }
