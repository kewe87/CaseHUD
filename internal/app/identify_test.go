package app

import (
	"bytes"
	"context"
	"testing"
	"time"
)

func TestIdentifyShowsOverlay(t *testing.T) {
	e := &Engine{Store: &Store{Dir: t.TempDir()}, Demo: true}
	if err := e.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer e.Stop()
	id := e.Panels()[0].ID
	time.Sleep(300 * time.Millisecond)
	before := append([]byte(nil), e.Preview(id)...)
	if err := e.Identify(id); err != nil {
		t.Fatal(err)
	}
	time.Sleep(600 * time.Millisecond)
	if bytes.Equal(before, e.Preview(id)) {
		t.Fatal("preview did not change after identify")
	}
	w := e.workers[id]
	if _, ok := w.identifyFrame(w.p.Size(), ""); !ok {
		t.Fatal("identify frame not active")
	}
}
