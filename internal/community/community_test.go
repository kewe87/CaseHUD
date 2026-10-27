package community

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGallery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/g/index.json":
			w.Write([]byte(`{"version":1,"designs":[
				{"id":"ring","name":"Ring","author":"kewe87","round":true,"file":"designs/ring.json"},
				{"id":"BAD ID","name":"x","file":"x.json"},
				{"id":"evil","name":"Evil","file":"https://example.com/x.json"}]}`))
		case "/g/designs/ring.json":
			w.Write([]byte(`{"width":480,"height":480,"lhm_url":"http://attacker/","widgets":[{"type":"gauge","x":0,"y":0,"w":480,"h":480,"source":"Load/CPU Total"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	g := New(srv.URL + "/g/index.json")
	ctx := context.Background()
	es, err := g.Entries(ctx)
	if err != nil || len(es) != 2 {
		t.Fatalf("Entries = %+v, %v (invalid ids must be dropped)", es, err)
	}
	l, e, err := g.Design(ctx, "ring")
	if err != nil || e.Author != "kewe87" || l.Name != "Ring" || l.LHMURL != "" || len(l.Widgets) != 1 {
		t.Fatalf("Design = %+v %+v %v", l, e, err)
	}
	if _, _, err := g.Design(ctx, "evil"); err == nil {
		t.Fatal("absolute design URLs must be refused")
	}
	if _, err := Parse([]byte(`{"width":480,"height":480,"widgets":[]}`)); err == nil {
		t.Fatal("empty design accepted")
	}
}
