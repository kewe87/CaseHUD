package lcd

import (
	"image"
	"image/color"
	"math"
	"strings"

	"golang.org/x/image/font"
)

// Inline icons (Widget.Inline): an icon that stands in front of a text of
// the value or gauge widget before it, centred with that text as one group.
//
//	[icon] gap [text]      gap = 0.2 x icon size between the squares; the margin
//	                         of the drawing in its square makes it look like about 0.3
//
// The group is centred on the widget's axis, the icon's centre sits on the
// middle of the digit height of the text. The group is measured per frame
// with tabular digits, so it only moves when the number of digits changes.

const inlineGap = 0.2

// inlineMin is the smallest automatic icon size (layout pixels).
const inlineMin = 44

// bindInline finds, for every inline icon, the value or gauge widget before it.
func (r *Renderer) bindInline(ws []Widget) {
	r.curW = ws
	r.inlHost = nil
	host := -1
	for i, w := range ws {
		switch {
		case w.Type == "value" || w.Type == "gauge":
			host = i
		case w.Type == "icon" && w.Inline != "" && host >= 0:
			if r.inlHost == nil {
				r.inlHost = map[int][]int{}
			}
			r.inlHost[host] = append(r.inlHost[host], i)
		}
	}
}

// inlineIcon returns the inline icon of the widget being drawn for text target.
func (r *Renderer) inlineIcon(target string) (int, Widget, bool) {
	for _, i := range r.inlHost[r.cur] {
		if i < len(r.curW) && r.curW[i].Inline == target {
			return i, r.curW[i], true
		}
	}
	return 0, Widget{}, false
}

// inlineSize is the icon's edge for text with digit height cap.
func (r *Renderer) inlineSize(iw Widget, cap float64) float64 {
	if s := float64(min(iw.W, iw.H)); s > 0 {
		return s
	}
	f := 1.2
	if iw.Inline == "main" {
		f = 1 // next to a big number the icon must not rise above its digits
	}
	return math.Max(f*cap, inlineMin*r.ss)
}

// inlineExtra is the width the icon and its gap add to a text at size fs.
func (r *Renderer) inlineExtra(target string, fs float64) float64 {
	_, iw, ok := r.inlineIcon(target)
	if !ok {
		return 0
	}
	return r.inlineSize(iw, digitHeight(r.face(true, fs))) * (1 + inlineGap)
}

// placeInline records the icon square of widget i for a text of width tw
// and digit height cap on baseline, centred on cx as a group, and returns
// the centre of the text.
func (r *Renderer) placeInline(i int, iw Widget, cx, baseline, cap, tw float64) float64 {
	size := r.inlineSize(iw, cap)
	gap := size * inlineGap
	left := cx - (size+gap+tw)/2
	top := baseline - cap/2 - size/2
	rd := func(v float64) int { return int(math.Round(v)) }
	if r.inlDraw == nil {
		r.inlDraw, r.inlOut = map[int]image.Rectangle{}, map[int]image.Rectangle{}
		r.inlAxis = map[int]float64{}
	}
	r.inlDraw[i] = image.Rect(rd(left), rd(top), rd(left)+rd(size), rd(top)+rd(size))
	r.inlOut[i] = image.Rect(rd(left/r.ss), rd(top/r.ss), rd(left/r.ss)+rd(size/r.ss), rd(top/r.ss)+rd(size/r.ss))
	if r.inlTxt == nil {
		r.inlTxt = map[int]image.Rectangle{}
	}
	x0 := left + size + gap
	r.inlTxt[i] = image.Rect(rd(x0/r.ss), rd((baseline-cap)/r.ss), rd((x0+tw)/r.ss), rd(baseline/r.ss))
	r.inlAxis[i] = cx / r.ss
	return left + size + gap + tw/2
}

func (r *Renderer) drawLabelAt(img *image.RGBA, f font.Face, cx, baseline float64, s string, c color.Color) {
	if i, iw, ok := r.inlineIcon("label"); ok {
		cx = r.placeInline(i, iw, cx, baseline, digitHeight(f), measureTracked(f, s))
	}
	r.drawLabel(img, f, cx, baseline, s, c)
}

func (r *Renderer) drawValueAt(img *image.RGBA, cx, baseline, size float64, val, unit string, c, uc color.Color) {
	if i, iw, ok := r.inlineIcon("main"); ok {
		cx = r.placeInline(i, iw, cx, baseline, digitHeight(r.face(true, size)), r.valueWidth(size, val, unit))
	}
	r.drawValue(img, cx, baseline, size, val, unit, c, uc)
}

func (r *Renderer) drawSubAt(img *image.RGBA, f font.Face, cx, baseline float64, s string, c color.Color) {
	i, iw, ok := r.inlineIcon("sub")
	if !ok {
		r.drawCentered(img, f, cx, baseline, s, c)
		return
	}
	s = strings.TrimSpace(s)
	tw := measureNum(f, s)
	cx = r.placeInline(i, iw, cx, baseline, digitHeight(f), tw)
	r.drawNum(img, f, cx-tw/2, baseline, s, c)
}

// InlineGroup is one laid-out inline icon with the text it stands before, in
// output pixels, as of the last drawn frame.
type InlineGroup struct {
	Widget int             // index of the icon widget
	Icon   image.Rectangle // the icon square
	Text   image.Rectangle // advance box of the text: digit height tall
	Axis   float64         // the centre line the group is centred on
}

// InlineGroups lists the inline icons of the last drawn frame.
func (r *Renderer) InlineGroups() []InlineGroup {
	var out []InlineGroup
	for i, w := range r.draftL.Widgets {
		if w.Type == "icon" && w.Inline != "" {
			if sq, ok := r.inlOut[i]; ok {
				out = append(out, InlineGroup{i, sq, r.inlTxt[i], r.inlAxis[i]})
			}
		}
	}
	return out
}
