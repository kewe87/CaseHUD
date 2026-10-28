package update

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEmbeddedKeyIsTheDocumentedOne(t *testing.T) {
	docs, err := os.ReadFile(filepath.Join("..", "..", "docs", "release-signing.pub.pem"))
	if err != nil {
		t.Fatal(err)
	}
	norm := func(b []byte) []byte { return bytes.TrimSpace(bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n"))) }
	if !bytes.Equal(norm(docs), norm(publicKeyPEM)) {
		t.Fatal("internal/update/release-signing.pub.pem differs from docs/release-signing.pub.pem")
	}
	if _, err := releaseKey(); err != nil {
		t.Fatal(err)
	}
}

// The checksums of the real 1.0.1 release, signed in CI.
func TestVerifyRealRelease(t *testing.T) {
	sums, _ := os.ReadFile("testdata/SHA256SUMS.txt")
	sig, _ := os.ReadFile("testdata/SHA256SUMS.txt.sig")
	if err := VerifySums(sums, sig); err != nil {
		t.Fatal(err)
	}
	got, err := SumFor(sums, "CaseHUD-Setup-1.0.1.exe")
	if err != nil || hex.EncodeToString(got[:]) != "a46b90db95c570f0b03488dea71f2242e542c0ea78252e14641c15b3435759b1" {
		t.Fatalf("SumFor: %x %v", got, err)
	}
	// One changed byte, or the file without the context, must fail.
	bad := bytes.Replace(sums, []byte("1.0.1.exe"), []byte("1.0.2.exe"), 1)
	if VerifySums(bad, sig) == nil {
		t.Fatal("changed checksums verified")
	}
	if VerifySums(sums, []byte("AAAA")) == nil || VerifySums(sums, []byte("not base64!")) == nil {
		t.Fatal("garbage signature verified")
	}
}

func TestSumFor(t *testing.T) {
	d := sha256.Sum256([]byte("x"))
	h := hex.EncodeToString(d[:])
	sums := []byte(h + "  a.exe\r\n" + h + " *b.exe\n" + h + "  c.exe\n" + h + "  c.exe\nzz  d.exe\n")
	for _, ok := range []string{"a.exe", "b.exe"} {
		if got, err := SumFor(sums, ok); err != nil || got != d {
			t.Errorf("%s: %v", ok, err)
		}
	}
	for _, bad := range []string{"c.exe", "d.exe", "e.exe", "a.ex", ""} {
		if _, err := SumFor(sums, bad); err == nil {
			t.Errorf("%s accepted", bad)
		}
	}
}

func TestValidVersion(t *testing.T) {
	for _, v := range []string{"1.0.2", "10.20.300"} {
		if !ValidVersion(v) {
			t.Errorf("%s rejected", v)
		}
	}
	for _, v := range []string{"", "1.0", "1.0.2-beta", "v1.0.2", "1.0.2/../x", "1.0.2 ", "1.0.12345"} {
		if ValidVersion(v) {
			t.Errorf("%q accepted", v)
		}
	}
}

func TestCleanNotes(t *testing.T) {
	in := "## Neu\r\n- a‮b\x1b[31m​c\tok"
	if got := cleanNotes(in); got != "## Neu\n- ab[31mc\tok" {
		t.Errorf("%q", got)
	}
	if got := cleanNotes("x󠁁⁠؜￹y"); got != "xy" {
		t.Errorf("invisible characters kept: %q", got)
	}
	if n := len([]rune(cleanNotes(strings.Repeat("ä", maxNotes+50)))); n != maxNotes+2 {
		t.Errorf("not capped: %d", n)
	}
}
