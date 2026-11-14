package fileacl

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFileReplacesAtomically(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "secret.json")
	for _, content := range []string{"first", "second, longer"} {
		if err := WriteFile(p, []byte(content)); err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(p)
		if err != nil || string(b) != content {
			t.Fatalf("%v %q", err, b)
		}
	}
	left, _ := filepath.Glob(filepath.Join(dir, "*.tmp"))
	if len(left) != 0 {
		t.Fatalf("temporary files left: %v", left)
	}
	if fi, err := os.Stat(p); err != nil || fi.Mode().Perm()&0o077 != 0 && os.PathSeparator == '/' {
		t.Fatalf("mode %v %v", fi, err)
	}
}

func TestWriteFileIntoAMissingFolderFails(t *testing.T) {
	if err := WriteFile(filepath.Join(t.TempDir(), "nope", "x"), []byte("x")); err == nil {
		t.Fatal("no error")
	}
}
