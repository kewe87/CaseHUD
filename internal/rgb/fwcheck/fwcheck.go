// Package fwcheck looks up the newest firmware the manufacturer offers for
// the Lian Li devices this module drives, so a host can tell the user that an
// update (done with the manufacturer's software) is available.
//
// It reads one public web page and only looks at the file names it links to;
// nothing is downloaded and nothing is sent to a device. Call it once at
// start, and only when the user has not switched it off.
package fwcheck

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// PageURL is the manufacturer's page with the firmware files (also the page
// to send the user to).
const PageURL = "https://lian-li.com/l-connect3/l3_update_manually/"

const (
	maxPage = 4 << 20 // bytes read from the page at most
	timeout = 15 * time.Second
)

// Release is the newest firmware file found for one product.
type Release struct {
	// Product is the USB vendor:product ID ("0416:7372"), lower case.
	Product string `json:"product"`
	Name    string `json:"name"`
	// Version is in the form the device reports it (see Firmware of the
	// backends and the screen's version string), "" if the file name has none.
	Version string `json:"version,omitempty"`
	// Date is the build date in the file name (YYYY-MM-DD), "" if none.
	Date string `json:"date,omitempty"`
	File string `json:"file"` // file name on the page
}

type product struct {
	id, name string
	re       *regexp.Regexp
	parse    func(m []string) (version, date string)
}

var products = []product{
	{"0416:7372", "UNI FAN TL controller", regexp.MustCompile(`TL12028-Controller_V(\d)(\d{2})(\d{2})`),
		func(m []string) (string, string) { return atoiStr(m[2]) + "." + atoiStr(m[3]), "" }},
	{"1cbe:a088", `Universal Screen 8.8"`, regexp.MustCompile(`lianli88_(\d{4})_(\d{4})`),
		func(m []string) (string, string) {
			n, _ := strconv.Atoi(m[2])
			if n < 18 {
				return "", ""
			}
			return fmt.Sprintf("%d.%d", (n-8)/10, (n-8)%10), ""
		}},
	{"04fc:7393", "TL LCD fan", regexp.MustCompile(`JZ5081_(\d{4})(\d{2})(\d{2})_\d+`),
		func(m []string) (string, string) { return "", m[1] + "-" + m[2] + "-" + m[3] }},
}

func atoiStr(s string) string {
	n, _ := strconv.Atoi(s)
	return strconv.Itoa(n)
}

var href = regexp.MustCompile(`href="([^"]{1,512})"`)

// Parse extracts the newest release per product from the page's links.
func Parse(page []byte) []Release {
	best := map[string]Release{}
	for _, h := range href.FindAllSubmatch(page, -1) {
		link := string(h[1])
		file := link[strings.LastIndexByte(link, '/')+1:]
		for _, p := range products {
			m := p.re.FindStringSubmatch(file)
			if m == nil {
				continue
			}
			v, d := p.parse(m)
			r := Release{Product: p.id, Name: p.name, Version: v, Date: d, File: file}
			if old, ok := best[p.id]; !ok || newer(r, old) {
				best[p.id] = r
			}
		}
	}
	var out []Release
	for _, p := range products {
		if r, ok := best[p.id]; ok {
			out = append(out, r)
		}
	}
	return out
}

func newer(a, b Release) bool {
	if a.Version != "" || b.Version != "" {
		return Newer(a.Version, b.Version)
	}
	return a.Date > b.Date
}

// Newer reports whether version a ("0.62", "1.5") is newer than b. A missing
// or unreadable version is never newer.
func Newer(a, b string) bool {
	pa, oka := parts(a)
	pb, okb := parts(b)
	if !oka {
		return false
	}
	if !okb {
		return true
	}
	for i := 0; i < max(len(pa), len(pb)); i++ {
		x, y := at(pa, i), at(pb, i)
		if x != y {
			return x > y
		}
	}
	return false
}

func parts(v string) ([]int, bool) {
	if v == "" {
		return nil, false
	}
	var out []int
	for _, s := range strings.Split(v, ".") {
		n, err := strconv.Atoi(s)
		if err != nil {
			return nil, false
		}
		out = append(out, n)
	}
	return out, true
}

func at(p []int, i int) int {
	if i < len(p) {
		return p[i]
	}
	return 0
}

// Latest fetches PageURL once and returns the newest release per product.
// client nil uses a client with a 15 s timeout.
func Latest(ctx context.Context, client *http.Client) ([]Release, error) {
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, PageURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fwcheck: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fwcheck: %s", resp.Status)
	}
	page, err := io.ReadAll(io.LimitReader(resp.Body, maxPage))
	if err != nil {
		return nil, fmt.Errorf("fwcheck: %w", err)
	}
	out := Parse(page)
	if len(out) == 0 {
		return nil, errors.New("fwcheck: no known firmware on the page (layout changed?)")
	}
	return out, nil
}
