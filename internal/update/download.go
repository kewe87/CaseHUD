package update

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Size limits for the release files.
const (
	maxSums  = 64 << 10
	maxSig   = 4 << 10
	maxSetup = 400 << 20
)

// Hosts a release download may go to: GitHub itself and the hosts its
// release links redirect to. Only https, default port.
var releaseHosts = map[string]bool{
	"github.com":                            true,
	"objects.githubusercontent.com":         true,
	"release-assets.githubusercontent.com":  true,
	"github-releases.githubusercontent.com": true,
}

// Overridable in tests only.
var (
	downloadBase = "https://github.com/" + Repo + "/releases/download/"
	hostAllowed  = func(u *url.URL) bool {
		return u.Scheme == "https" && u.Port() == "" && u.User == nil && releaseHosts[strings.ToLower(u.Hostname())]
	}
	downloadClient = &http.Client{
		CheckRedirect: checkRedirect,
		// A stalled server must not keep the update busy until the
		// overall timeout.
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			TLSHandshakeTimeout:   15 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second,
			IdleConnTimeout:       30 * time.Second,
		},
	}
)

func checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 5 {
		return errors.New("too many redirects")
	}
	if !hostAllowed(req.URL) {
		return fmt.Errorf("redirect to %s refused", req.URL.Host)
	}
	return nil
}

// Only plain release versions are installed (no pre-releases), and the
// version also ends up in a URL and a file name.
var releaseVersion = regexp.MustCompile(`^[0-9]{1,4}\.[0-9]{1,4}\.[0-9]{1,4}$`)

// ValidVersion reports whether v can be downloaded and installed.
func ValidVersion(v string) bool { return releaseVersion.MatchString(v) }

// SetupName is the setup's file name in a release.
func SetupName(version string) string { return "CaseHUD-Setup-" + version + ".exe" }

// Progress reports downloaded bytes; total is -1 while unknown.
type Progress func(done, total int64)

// Download fetches the setup of version into dir (which must exist and be
// empty). SHA256SUMS.txt and its signature come first: nothing else is
// downloaded unless the signature verifies. The setup is hashed while it
// is written and must match its signed line. It returns the file and the
// signed digest; a failed download leaves nothing behind.
func Download(ctx context.Context, version, dir string, progress Progress) (string, [32]byte, error) {
	var want [32]byte
	if !ValidVersion(version) {
		return "", want, fmt.Errorf("invalid version %q", version)
	}
	base := downloadBase + "v" + version + "/"
	sums, err := fetchSmall(ctx, base+"SHA256SUMS.txt", maxSums)
	if err != nil {
		return "", want, err
	}
	sig, err := fetchSmall(ctx, base+"SHA256SUMS.txt.sig", maxSig)
	if err != nil {
		return "", want, err
	}
	if err := VerifySums(sums, sig); err != nil {
		return "", want, err
	}
	name := SetupName(version)
	if want, err = SumFor(sums, name); err != nil {
		return "", want, err
	}
	path := filepath.Join(dir, name)
	got, err := fetchFile(ctx, base+name, path, progress)
	if err == nil && got != want {
		err = errors.New("the setup does not match its signed checksum")
	}
	if err != nil {
		os.Remove(path)
		return "", want, err
	}
	return path, want, nil
}

func get(ctx context.Context, u string) (*http.Response, error) {
	pu, err := url.Parse(u)
	if err != nil || !hostAllowed(pu) {
		return nil, fmt.Errorf("download from %s refused", u)
	}
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "CaseHUD-updater")
	resp, err := downloadClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("%s: HTTP %d", filepath.Base(pu.Path), resp.StatusCode)
	}
	return resp, nil
}

func fetchSmall(ctx context.Context, u string, limit int64) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	resp, err := get(ctx, u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > limit {
		return nil, fmt.Errorf("%s is too large", filepath.Base(u))
	}
	return b, nil
}

func fetchFile(ctx context.Context, u, path string, progress Progress) ([32]byte, error) {
	var sum [32]byte
	resp, err := get(ctx, u)
	if err != nil {
		return sum, err
	}
	defer resp.Body.Close()
	total := resp.ContentLength
	if total > maxSetup {
		return sum, errors.New("the setup is too large")
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return sum, err
	}
	h := sha256.New()
	w := &countWriter{w: io.MultiWriter(f, h), total: total, progress: progress}
	n, err := io.Copy(w, io.LimitReader(resp.Body, maxSetup+1))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return sum, err
	}
	if n > maxSetup {
		return sum, errors.New("the setup is too large")
	}
	if total >= 0 && n != total {
		return sum, errors.New("the download was cut short")
	}
	copy(sum[:], h.Sum(nil))
	return sum, nil
}

type countWriter struct {
	w        io.Writer
	done     int64
	total    int64
	progress Progress
}

func (c *countWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.done += int64(n)
	if c.progress != nil {
		c.progress(c.done, c.total)
	}
	return n, err
}
