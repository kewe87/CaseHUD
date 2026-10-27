package metrics

import (
	"regexp"
	"strings"
)

// exprOp splits a source into terms joined by " + " or " - " (spaces
// required, so sensor names like "+12V" stay intact).
var exprOp = regexp.MustCompile(`\s+([+-])\s+`)

// Eval resolves a source that may combine sensors, e.g.
// "Powers/Package + Powers/GPU Package" for total power or
// "Temperatures/T Sensor - Temperatures/Ambient" for a coolant delta. Each
// term is a normal source (with "|" alternatives); all terms must resolve.
// The result carries the first term's unit.
func Eval(p Provider, source string) (Reading, bool) {
	idx := exprOp.FindAllStringSubmatchIndex(source, -1)
	if len(idx) == 0 {
		return p.Get(source)
	}
	first, ok := p.Get(strings.TrimSpace(source[:idx[0][0]]))
	if !ok {
		return Reading{}, false
	}
	out := Reading{Key: source, Path: source, Value: first.Value, Unit: first.Unit}
	for i, m := range idx {
		end := len(source)
		if i+1 < len(idx) {
			end = idx[i+1][0]
		}
		r, ok := p.Get(strings.TrimSpace(source[m[1]:end]))
		if !ok {
			return Reading{}, false
		}
		if source[m[2]:m[3]] == "-" {
			out.Value -= r.Value
		} else {
			out.Value += r.Value
		}
	}
	return out, true
}

// IsDifference reports whether source subtracts readings ("A - B").
func IsDifference(source string) bool {
	for _, m := range exprOp.FindAllStringSubmatch(source, -1) {
		if m[1] == "-" {
			return true
		}
	}
	return false
}
