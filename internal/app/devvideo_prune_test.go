package app

import (
	"fmt"
	"os"
	"testing"
	"time"
)

func TestDeviceVideoJobsAreBounded(t *testing.T) {
	var d devVideoState
	d.jobs = map[string]*devVideoJob{}
	done := func() chan struct{} { c := make(chan struct{}); close(c); return c }
	file := t.TempDir() + "/ok.mp4"
	os.WriteFile(file, []byte("x"), 0o600)
	old := time.Now().Add(-2 * devVideoRetry)
	d.jobs["failed-old"] = &devVideoJob{path: "gone", started: old, done: done(), err: fmt.Errorf("x")}
	d.jobs["failed-new"] = &devVideoJob{path: "gone", started: time.Now(), done: done(), err: fmt.Errorf("x")}
	d.jobs["pending"] = &devVideoJob{path: file, started: old, done: make(chan struct{})}
	for i := 0; i < 3*devVideoJobsMax; i++ {
		k := fmt.Sprintf("ready-%d", i)
		d.jobs[k] = &devVideoJob{path: file, started: time.Now().Add(time.Duration(i) * time.Second), done: done()}
		d.pruneLocked("")
	}
	if len(d.jobs) > devVideoJobsMax {
		t.Fatalf("%d jobs", len(d.jobs))
	}
	if _, ok := d.jobs["failed-old"]; ok {
		t.Fatal("old failed job kept")
	}
	if _, ok := d.jobs["pending"]; !ok {
		t.Fatal("running job dropped")
	}
	if _, ok := d.jobs[fmt.Sprintf("ready-%d", 3*devVideoJobsMax-1)]; !ok {
		t.Fatal("newest job dropped")
	}
}
