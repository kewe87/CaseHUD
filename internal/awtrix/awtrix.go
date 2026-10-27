// Package awtrix talks to AWTRIX pixel clocks over their HTTP API: CaseHUD
// shows its readings as custom apps in the clock's app loop. It speaks both
// AWTRIX 3 (Ulanzi TC001) and AWTRIX NG (TC001 and the 52×16 TC002).
package awtrix

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"
)

// Firmware names the API a clock speaks.
type Firmware string

const (
	Awtrix3 Firmware = "awtrix3"
	NG      Firmware = "ng"
)

// Fragment is a piece of text in one colour.
type Fragment struct {
	T string `json:"t"`
	C string `json:"c"` // RRGGBB without '#'
}

// Card is what CaseHUD wants to show, independent of the firmware.
type Card struct {
	Icon       string     // one of IconIDs, "" = none
	Label      string     // shown before the text when the icon is missing
	Text       []Fragment // the reading, coloured
	Progress   int        // 0-100, -1 = no bar
	ProgressC  string
	ProgressBC string
	Lifetime   time.Duration // the clock drops the app without updates
}

// Info describes a clock.
type Info struct {
	Firmware Firmware `json:"firmware"`
	Version  string   `json:"version"`
	Name     string   `json:"name,omitempty"` // host name it announces
	UID      string   `json:"uid,omitempty"`
	Model    string   `json:"model"` // "tc001", "tc002" or "" when unknown
	W        int      `json:"w"`
	H        int      `json:"h"`
}

// Client is one clock.
type Client struct {
	Host string // IP or host name, optionally with http:// and port
	HTTP *http.Client
}

// New returns a client with a short timeout: clocks sit on Wi-Fi and a
// missing one must not stall the others.
func New(host string) *Client {
	return newClient(host, 3*time.Second)
}

func newClient(host string, timeout time.Duration) *Client {
	return &Client{Host: host, HTTP: &http.Client{Timeout: timeout,
		Transport: lanTransport(),
		// A device on the LAN must not bounce requests elsewhere.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

// allowedIP reports whether ip may be a clock: loopback and private networks
// (RFC 1918, unique local) only. Link-local (169.254/16, fe80::/10, which
// include cloud metadata services) and public addresses are refused.
func allowedIP(ip net.IP) bool {
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate()
}

// lanTransport only connects to allowedIP addresses, checked on the address
// actually dialled (after name resolution), and never uses a proxy.
func lanTransport() *http.Transport {
	d := &net.Dialer{Timeout: 3 * time.Second, Control: func(_, address string, _ syscall.RawConn) error {
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return err
		}
		if ip := net.ParseIP(host); ip == nil || !allowedIP(ip) {
			return fmt.Errorf("address not allowed")
		}
		return nil
	}}
	return &http.Transport{Proxy: nil, DialContext: d.DialContext}
}

func (c *Client) url(path string, q url.Values) (string, error) {
	h := strings.TrimSpace(c.Host)
	if h == "" {
		return "", fmt.Errorf("no address")
	}
	if !strings.Contains(h, "://") {
		h = "http://" + h
	}
	u, err := url.Parse(h)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", fmt.Errorf("invalid address %q", c.Host)
	}
	if ip := net.ParseIP(strings.Trim(u.Hostname(), "[]")); ip != nil && !allowedIP(ip) {
		return "", fmt.Errorf("address not allowed")
	}
	u.Path = path
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func (c *Client) do(ctx context.Context, method, path string, q url.Values, body []byte, ctype string) ([]byte, error) {
	u, err := c.url(path, q)
	if err != nil {
		return nil, err
	}
	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, rd)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", ctype)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode/100 != 2 {
		return nil, &StatusError{Host: c.Host, Code: resp.StatusCode}
	}
	return b, nil
}

// StatusError is an HTTP error answer from a clock.
type StatusError struct {
	Host string
	Code int
}

// Error is deliberately generic; the status code stays in Code for logic.
func (e *StatusError) Error() string { return e.Host + ": request failed" }

// ValidHost reports whether host can be used as a clock address.
func ValidHost(host string) bool {
	_, err := New(host).url("/", nil)
	return err == nil
}

// Probe finds out which firmware the clock runs and how big its display is.
func (c *Client) Probe(ctx context.Context) (Info, error) {
	if b, err := c.do(ctx, http.MethodGet, "/api/v1/device", nil, nil, ""); err == nil {
		var d struct {
			Version   string `json:"version"`
			UID       string `json:"uid"`
			BoardType string `json:"boardType"`
			Hostname  string `json:"hostname"`
		}
		if json.Unmarshal(b, &d) == nil && d.Version != "" {
			i := Info{Firmware: NG, Version: d.Version, Name: d.Hostname, UID: d.UID, Model: "tc001", W: 32, H: 8}
			if d.BoardType == "tc002" {
				i.Model, i.W, i.H = "tc002", 52, 16
			} else if b, err := c.do(ctx, http.MethodGet, "/api/v1/display/screen", nil, nil, ""); err == nil {
				// A DIY build can have another matrix size.
				var s struct{ Width, Height int }
				if json.Unmarshal(b, &s) == nil && s.Width > 0 && s.Height > 0 {
					i.W, i.H = s.Width, s.Height
					if s.Width != 32 || s.Height != 8 {
						i.Model = ""
					}
				}
			}
			return i, nil
		}
	} else if ctx.Err() != nil {
		return Info{}, err
	}
	b, err := c.do(ctx, http.MethodGet, "/api/stats", nil, nil, "")
	if err != nil {
		return Info{}, err
	}
	var s struct {
		Version string `json:"version"`
		UID     string `json:"uid"`
	}
	if err := json.Unmarshal(b, &s); err != nil || s.Version == "" {
		return Info{}, fmt.Errorf("%s does not answer like an AWTRIX clock", c.Host)
	}
	return Info{Firmware: Awtrix3, Version: s.Version, UID: s.UID, Name: s.UID, Model: "tc001", W: 32, H: 8}, nil
}

// iconName is the file name CaseHUD's icons get on AWTRIX 3 clocks.
func iconName(id string) string { return "casehud_" + id }

// UploadIcons stores CaseHUD's icons on an AWTRIX 3 clock (AWTRIX NG takes
// them inline with each app).
func (c *Client) UploadIcons(ctx context.Context) error {
	for _, id := range IconIDs {
		var body bytes.Buffer
		mw := multipart.NewWriter(&body)
		fw, err := mw.CreateFormFile("image", "ICONS/"+iconName(id)+".gif")
		if err != nil {
			return err
		}
		fw.Write(IconGIF(id, 8))
		mw.Close()
		if _, err := c.do(ctx, http.MethodPost, "/edit", nil, body.Bytes(), mw.FormDataContentType()); err != nil {
			return err
		}
	}
	return nil
}

// Push creates or updates the app name. icons says whether the clock has
// CaseHUD's icons (AWTRIX 3 needs UploadIcons first).
func (c *Client) Push(ctx context.Context, name string, card Card, info Info, icons bool) error {
	if info.Firmware == NG {
		b, err := json.Marshal(ngApp(card, info))
		if err != nil {
			return err
		}
		_, err = c.do(ctx, http.MethodPut, "/api/v1/apps/pushed/"+url.PathEscape(name), nil, b, "application/json")
		return err
	}
	if !icons {
		card.Icon = ""
	}
	b, err := json.Marshal(awtrix3App(card))
	if err != nil {
		return err
	}
	_, err = c.do(ctx, http.MethodPost, "/api/custom", url.Values{"name": {name}}, b, "application/json")
	return err
}

// Remove deletes the app name.
func (c *Client) Remove(ctx context.Context, name string, fw Firmware) error {
	if fw == NG {
		_, err := c.do(ctx, http.MethodDelete, "/api/v1/apps/"+url.PathEscape(name), nil, nil, "")
		return err
	}
	// AWTRIX 3 deletes a custom app when it gets an empty body.
	_, err := c.do(ctx, http.MethodPost, "/api/custom", url.Values{"name": {name}}, []byte{}, "application/json")
	return err
}

// withLabel puts the label in front of the text when there is no icon.
func withLabel(card Card) []Fragment {
	if card.Icon != "" || card.Label == "" {
		return card.Text
	}
	return append([]Fragment{{T: card.Label + " ", C: "FFFFFF"}}, card.Text...)
}

// awtrix3App is the AWTRIX 3 custom app JSON.
func awtrix3App(card Card) map[string]any {
	a := map[string]any{
		"text":     withLabel(card),
		"textCase": 2, // as sent
		"center":   true,
		"progress": card.Progress,
		"lifetime": int(card.Lifetime / time.Second),
	}
	if card.Icon != "" {
		a["icon"] = iconName(card.Icon)
	}
	if card.Progress >= 0 {
		a["progressC"], a["progressBC"] = card.ProgressC, card.ProgressBC
	}
	return a
}

// ngApp is the AWTRIX NG pushed app JSON. NG rejects unknown keys, so only
// documented ones are sent.
func ngApp(card Card, info Info) map[string]any {
	type frag struct {
		Text  string `json:"text"`
		Color string `json:"color"`
	}
	var text []frag
	for _, f := range withLabel(card) {
		text = append(text, frag{f.T, "#" + f.C})
	}
	a := map[string]any{
		"text":       text,
		"textCase":   "asTyped",
		"progress":   card.Progress,
		"lifetimeMs": card.Lifetime.Milliseconds(),
	}
	if card.Icon != "" {
		size := 8
		if info.Model == "tc002" {
			// A 16×16 icon keeps the app at native size, so the text needs a
			// full-height font to match.
			size = 16
			a["font"] = "matrix-chunky8"
		}
		a["icon"] = "data:image/gif;base64," + base64.StdEncoding.EncodeToString(IconGIF(card.Icon, size))
	}
	if card.Progress >= 0 {
		a["progressColor"], a["progressTrackColor"] = "#"+card.ProgressC, "#"+card.ProgressBC
	}
	return a
}
