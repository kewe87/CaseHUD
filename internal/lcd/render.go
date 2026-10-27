package lcd

import (
	"image"
	"image/color"
	"image/draw"
	"math"
	"strings"
	"sync"
	"time"

	_ "embed"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"

	"github.com/kewe87/casehud/internal/metrics"
)

// Rajdhani (SIL OFL 1.1, see fonts/OFL.txt): a condensed technical face that
// keeps big numbers readable on small round screens.
var (
	//go:embed fonts/Rajdhani-Bold.ttf
	fontBold []byte
	//go:embed fonts/Rajdhani-SemiBold.ttf
	fontSemi []byte
)

var (
	fontsOnce          sync.Once
	fontsBold, fontsMd *opentype.Font
	fontsErr           error
)

// parsedFonts parses the embedded fonts once per process.
func parsedFonts() (*opentype.Font, *opentype.Font, error) {
	fontsOnce.Do(func() {
		fontsBold, fontsErr = opentype.Parse(fontBold)
		if fontsErr == nil {
			fontsMd, fontsErr = opentype.Parse(fontSemi)
		}
	})
	return fontsBold, fontsMd, fontsErr
}

// supersample is the internal render scale; frames are drawn larger and
// downscaled, which smooths arcs, curves and text edges.
const supersample = 2

// Renderer draws a Layout from live metrics.
type Renderer struct {
	L       Layout  // drawing layout (supersampled)
	draftL  Layout  // the layout at output size, for animation in-between frames
	ss      float64 // scale of the layout being drawn (supersample or 1)
	out     image.Point
	hist    *metrics.History
	bold    *opentype.Font
	medium  *opentype.Font
	faces   map[faceKey]font.Face
	bg, pnl color.RGBA
	fg, mut color.RGBA
	acc     color.RGBA
	warn    color.RGBA
	crit    color.RGBA
	sub     color.RGBA // second values
	Now     func() time.Time

	// Fahrenheit shows temperatures (sensors reporting °C) in °F; Clock12h
	// shows clocks with AM/PM.
	Fahrenheit bool
	Clock12h   bool
	celsius    map[string]tempKind // sources whose sensor reports °C

	// Transparent leaves the background clear so the frame can be laid
	// over an animated Background.
	Transparent bool

	// Animation: shown values glide towards their readings.
	animTau   time.Duration // 0 = off
	anim      map[string]*animValue
	frame     int
	lastFrame time.Time
	settled   bool
	alpha     float64 // smoothing step of the current frame

	// SplitIcons leaves icons out of Frame/Render; the caller adds them per
	// display frame with ComposeIcons/DrawIcons (see icon.go).
	SplitIcons bool
	icons      map[int]*iconState
	// inline icons (Widget.Inline): the host widget of each icon and where the
	// last frame put them (draw pixels, output pixels).
	inlHost   map[int][]int
	inlDraw   map[int]image.Rectangle
	inlOut    map[int]image.Rectangle
	inlTxt    map[int]image.Rectangle
	inlAxis   map[int]float64
	cur       int // widget being drawn
	vstats    []ValueStat
	curW      []Widget
	testClock *time.Time // tests: the clock behind Now
}

// animValue is one source's displayed value while it glides.
type animValue struct {
	shown, target float64
	frame         int // last frame the value was advanced in
}

type faceKey struct {
	bold bool
	size float64
}

// NewRenderer prepares fonts and colors for l.
func NewRenderer(l Layout) (*Renderer, error) {
	b, m, err := parsedFonts()
	if err != nil {
		return nil, err
	}
	out := image.Pt(l.Width, l.Height)
	r := &Renderer{
		L:       l.ScaledTo(l.Width*supersample, l.Height*supersample),
		draftL:  l,
		ss:      supersample,
		anim:    map[string]*animValue{},
		celsius: map[string]tempKind{},
		out:     out,
		hist:    metrics.NewHistory(l.HistorySize),
		bold:    b,
		medium:  m,
		faces:   map[faceKey]font.Face{},
		bg:      parseHex(l.Background, color.RGBA{0, 0, 0, 255}),
		pnl:     parseHex(l.Panel, color.RGBA{}),
		fg:      parseHex(l.Foreground, color.RGBA{255, 255, 255, 255}),
		mut:     parseHex(l.Muted, color.RGBA{138, 148, 168, 255}),
		acc:     parseHex(l.Accent, color.RGBA{34, 211, 238, 255}),
		warn:    warnFor(parseHex(l.Background, color.RGBA{0, 0, 0, 255})),
		sub:     subFor(parseHex(l.Background, color.RGBA{0, 0, 0, 255})),
		crit:    color.RGBA{244, 63, 94, 255},
		Now:     time.Now,
	}
	// Coloured themes (e.g. amber retro) get second values in a dimmed
	// foreground instead of the neutral grey.
	if lo, hi := minMax3(r.fg.R, r.fg.G, r.fg.B); hi-lo > 60 {
		r.sub = mixOKLab(r.fg, r.bg, 0.25)
	}
	return r, nil
}

func minMax3(a, b, c uint8) (lo, hi int) {
	lo, hi = int(a), int(a)
	for _, v := range []uint8{b, c} {
		lo, hi = min(lo, int(v)), max(hi, int(v))
	}
	return lo, hi
}

func (r *Renderer) face(bold bool, size float64) font.Face {
	k := faceKey{bold, math.Round(size)}
	if f, ok := r.faces[k]; ok {
		return f
	}
	src := r.medium
	if bold {
		src = r.bold
	}
	f, _ := opentype.NewFace(src, &opentype.FaceOptions{Size: k.size, DPI: 72, Hinting: font.HintingFull})
	r.faces[k] = f
	return f
}

// Sample reads every widget's sources from p and records them for graphs.
// Call once per frame before Render.
func (r *Renderer) Sample(p metrics.Provider) {
	p = clockSource{p, r.Now}
	for _, w := range r.L.Widgets {
		for _, src := range []string{w.Source, w.SubSource} {
			if src == "" {
				continue
			}
			if rd, ok := metrics.Eval(p, src); ok {
				r.hist.Push(src, rd.Value)
			}
		}
	}
}

// SetAnimation makes values glide to new readings with time constant tau
// (0 turns it off). Drive it with Frame.
func (r *Renderer) SetAnimation(tau time.Duration) { r.animTau = tau }

// Frame draws the next animation frame and reports whether every value has
// reached its reading. In-between frames are drawn without supersampling,
// which is about four times cheaper; the settled frame is full quality.
func (r *Renderer) Frame(p metrics.Provider) (*image.RGBA, bool) {
	p = clockSource{p, r.Now}
	now := time.Now()
	r.frame++
	r.settled = true
	if r.lastFrame.IsZero() {
		r.lastFrame = now
	}
	alpha := 1.0
	if r.animTau > 0 {
		alpha = 1 - math.Exp(-float64(now.Sub(r.lastFrame))/float64(r.animTau))
	}
	r.lastFrame = now
	r.alpha = alpha
	// A dry pass advances the values so the frame quality can be chosen.
	r.advance(p)
	if r.settled {
		return r.draw(p, r.L, supersample), true
	}
	return r.draw(p, r.draftL, 1), false
}

// Render draws one frame at full quality, showing readings directly.
func (r *Renderer) Render(p metrics.Provider) *image.RGBA {
	p = clockSource{p, r.Now}
	r.frame++
	r.alpha = 1
	return r.draw(p, r.L, supersample)
}

// advance moves every widget's shown values one step towards the readings.
func (r *Renderer) advance(p metrics.Provider) {
	for _, w := range r.L.Widgets {
		for _, src := range []string{w.Source, w.SubSource} {
			if src != "" && w.Type != "graph" {
				r.reading(p, src, "")
			}
		}
	}
}

func (r *Renderer) draw(p metrics.Provider, l Layout, ss float64) *image.RGBA {
	r.ss = ss
	img := image.NewRGBA(image.Rect(0, 0, l.Width, l.Height))
	if !r.Transparent {
		draw.Draw(img, img.Bounds(), image.NewUniform(r.bg), image.Point{}, draw.Src)
	}
	var tiles []image.Rectangle
	r.vstats = r.vstats[:0]
	r.bindInline(l.Widgets)
	for idx, w := range l.Widgets {
		r.cur = idx
		rect := image.Rect(w.X, w.Y, w.X+w.W, w.Y+w.H)
		if w.Type == "icon" && w.Inline != "" {
			sq, ok := r.inlDraw[idx]
			if !ok {
				continue
			}
			rect = sq
		}
		if r.pnl.A != 0 && w.Type != "text" && w.Type != "clock" && w.Type != "icon" && !insideAny(rect, tiles) {
			// Widgets laid over another one (a number inside a ring) share
			// its tile instead of covering it with their own.
			roundRect(img, rect, 14*r.ss, r.pnl)
			tiles = append(tiles, rect)
		}
		switch w.Type {
		case "icon":
			if !r.SplitIcons {
				r.drawIcon(img, idx, w, rect, p, r.Now(), false)
			}
		case "gauge":
			r.gauge(img, w, rect, p)
		case "value":
			r.value(img, w, rect, p)
		case "bar":
			r.bar(img, w, rect, p)
		case "graph":
			r.graph(img, w, rect, p)
		case "text":
			r.text(img, w, rect, w.Text)
		case "clock":
			layout := w.Text
			if layout == "" {
				layout = "15:04"
			}
			r.text(img, w, rect, localizeDate(r.Now().Format(clockLayout(layout, r.Clock12h, r.L.Lang)), r.L.Lang))
		}
	}
	if l.Scanlines {
		scanlines(img, int(2*r.ss))
	}
	if r.out == img.Bounds().Size() {
		return img
	}
	if img.Bounds().Dx() == r.out.X*2 && img.Bounds().Dy() == r.out.Y*2 {
		return downsample2x(img)
	}
	dst := image.NewRGBA(image.Rectangle{Max: r.out})
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Src, nil)
	return dst
}

// downsample2x averages 2x2 blocks: the exact inverse of 2x supersampling
// and far cheaper than a general resampling kernel.
func downsample2x(src *image.RGBA) *image.RGBA {
	w, h := src.Bounds().Dx()/2, src.Bounds().Dy()/2
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	sp, dp, ss := src.Pix, dst.Pix, src.Stride
	for y := 0; y < h; y++ {
		r0 := 2 * y * ss
		r1 := r0 + ss
		d := y * dst.Stride
		for x := 0; x < w; x++ {
			i := 8 * x
			for k := 0; k < 4; k++ {
				dp[d+4*x+k] = uint8((uint16(sp[r0+i+k]) + uint16(sp[r0+i+4+k]) + uint16(sp[r1+i+k]) + uint16(sp[r1+i+4+k]) + 2) / 4)
			}
		}
	}
	return dst
}

var (
	enDays   = []string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}
	deDays   = []string{"Montag", "Dienstag", "Mittwoch", "Donnerstag", "Freitag", "Samstag", "Sonntag", "Mo", "Di", "Mi", "Do", "Fr", "Sa", "So"}
	enMonths = []string{"January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December", "Mar", "May", "Oct", "Dec"}
	deMonths = []string{"Januar", "Februar", "März", "April", "Mai", "Juni", "Juli", "August", "September", "Oktober", "November", "Dezember", "Mär", "Mai", "Okt", "Dez"}
)

// clockLayout adapts a clock's Go time layout to the user's settings:
// 12-hour time with AM/PM, and month/day order for English dates.
func clockLayout(layout string, h12 bool, lang string) string {
	if h12 && strings.Contains(layout, "15") {
		layout = strings.Replace(layout, "15", "3", 1)
		if !strings.Contains(layout, "PM") && !strings.Contains(layout, "pm") {
			layout += " PM"
		}
	}
	if lang != "de" {
		layout = strings.Replace(layout, "02.01.2006", "Jan 2, 2006", 1)
		layout = strings.Replace(layout, "02.01.", "Jan 2", 1)
	}
	return layout
}

// localizeDate translates English day/month names produced by Go's time
// formatting.
func localizeDate(s, lang string) string {
	if lang != "de" {
		return s
	}
	// Full names come first in the tables; stop at the first hit so
	// "Monday" -> "Montag" isn't then mangled by the "Mon" rule.
	for i, d := range enDays {
		if strings.Contains(s, d) {
			s = strings.Replace(s, d, deDays[i], 1)
			break
		}
	}
	for i, m := range enMonths {
		if strings.Contains(s, m) {
			s = strings.Replace(s, m, deMonths[i], 1)
			break
		}
	}
	return s
}

// reading resolves a source for display; ok=false renders as "--". With
// animation on, the value glides towards the reading.
func (r *Renderer) reading(p metrics.Provider, src, unitOverride string) (float64, string, bool) {
	v, unit, ok := r.rawReading(p, src, unitOverride)
	if !ok || r.animTau <= 0 {
		return v, unit, ok
	}
	a := r.anim[src]
	if a == nil {
		a = &animValue{shown: v}
		r.anim[src] = a
	}
	a.target = v
	if a.frame != r.frame {
		a.frame = r.frame
		a.shown += (v - a.shown) * r.alpha
		// Snap once the difference is below what the display can show.
		if math.Abs(v-a.shown) <= math.Max(0.02, math.Abs(v)*0.002) {
			a.shown = v
		}
	}
	if a.shown != v {
		r.settled = false
	}
	return a.shown, unit, ok
}

// rawReading is reading plus the unit setting: it remembers which sources
// are in °C, so numbers can be shown in °F while thresholds stay in °C.
func (r *Renderer) rawReading(p metrics.Provider, src, unitOverride string) (float64, string, bool) {
	v, unit, ok := reading(p, src, unitOverride)
	if src == "" {
		return v, unit, ok
	}
	if rd, rok := metrics.Eval(p, src); rok {
		switch {
		case rd.Unit != "°C":
			r.celsius[src] = notTemp
		case metrics.IsDifference(src):
			r.celsius[src] = tempDelta // e.g. coolant minus ambient
		default:
			r.celsius[src] = tempAbs
		}
	}
	if r.Fahrenheit && r.celsius[src] != notTemp && (unit == "°C" || unit == "°F") {
		unit = "°F"
	}
	return v, unit, ok
}

type tempKind uint8

const (
	notTemp tempKind = iota
	tempAbs
	tempDelta
)

// num formats a value of src for display, converting °C to °F if set. A
// temperature difference only scales (5 °C apart = 9 °F apart).
func (r *Renderer) num(src string, v float64, decimals int) string {
	if r.Fahrenheit {
		switch r.celsius[src] {
		case tempAbs:
			v = v*9/5 + 32
		case tempDelta:
			v = v * 9 / 5
		}
	}
	return formatValue(v, decimals, r.L.Lang == "de")
}

// reading resolves the widget's main value; ok=false renders as "--".
func reading(p metrics.Provider, src, unitOverride string) (float64, string, bool) {
	if src == "" {
		return 0, unitOverride, false
	}
	rd, ok := metrics.Eval(p, src)
	unit := rd.Unit
	if unitOverride != "" {
		unit = unitOverride
	}
	if unit == "none" {
		unit = ""
	}
	return rd.Value, unit, ok
}

func (r *Renderer) colorFor(w Widget, v float64, ok bool) color.RGBA {
	if !ok {
		return r.mut
	}
	if w.Invert {
		// "Lower is worse" (FPS, pump RPM, flow): mirror values and
		// thresholds so the logic below applies unchanged.
		v, w.Warn, w.Crit = -v, -w.Warn, -w.Crit
		if w.Warn == 0 {
			w.Warn = math.Inf(-1)
		}
	}
	if w.Smooth && w.Warn != 0 && w.Crit > w.Warn {
		// Fade base -> warn -> crit instead of switching at thresholds.
		base := parseHex(w.Color, r.acc)
		lo := w.Warn - (w.Crit-w.Warn)*0.5
		switch {
		case v <= lo:
			return base
		case v <= w.Warn:
			return mixOKLab(base, r.warn, (v-lo)/(w.Warn-lo))
		case v <= w.Crit:
			return mixOKLab(r.warn, r.crit, (v-w.Warn)/(w.Crit-w.Warn))
		}
		return r.crit
	}
	if w.Crit != 0 && v >= w.Crit {
		return r.crit
	}
	if w.Warn != 0 && !math.IsInf(w.Warn, -1) && v >= w.Warn {
		return r.warn
	}
	return parseHex(w.Color, r.acc)
}

func (r *Renderer) fraction(w Widget, v float64) float64 {
	max := w.Max
	if max == 0 {
		max = 100
	}
	if max == w.Min {
		return 0
	}
	return clamp01((v - w.Min) / (max - w.Min))
}

func (r *Renderer) gauge(img *image.RGBA, w Widget, rect image.Rectangle, p metrics.Provider) {
	v, unit, ok := r.reading(p, w.Source, w.Unit)
	col := r.colorFor(w, v, ok)
	size := float64(min(rect.Dx(), rect.Dy()))
	cx := float64(rect.Min.X) + float64(rect.Dx())/2
	start, total := gaugeArc(w)
	cy := float64(rect.Min.Y) + float64(rect.Dy())/2
	if total < 360 {
		cy += size * 0.03 // the open bottom makes a centred arc look high
	}
	thick := size * 0.075
	if w.Thickness > 0 {
		thick = size * w.Thickness
	}
	radius := size/2 - thick/2 - size*0.07
	if w.Glow {
		radius = size/2 - thick*1.6 // leave room for the halo
	}
	if w.Ticks > 0 {
		r.ticks(img, cx, cy, radius+thick*1.4, size*0.025, w.Ticks, start, total)
	}
	sweep := 0.0
	if ok {
		sweep = total * r.fraction(w, v)
	}
	if w.Segments > 0 {
		r.segmentedArc(img, cx, cy, radius, thick, start, total, sweep, w.Segments, col, w.Glow)
	} else {
		arc(img, cx, cy, radius, thick, start, total, withAlpha(r.mut, 0.30))
		if ok {
			if w.Glow {
				// Stacked translucent arcs approximate a soft blur.
				var widths [6]float64
				for i := range widths {
					widths[i] = thick * (2.6 - float64(i)*0.27)
				}
				arcGlow(img, cx, cy, radius, widths[:], start, sweep, col, 0.045)
			}
			arc(img, cx, cy, radius, thick, start, sweep, col)
		}
	}

	if w.RingOnly {
		return
	}
	val := "--"
	if ok {
		val = r.num(w.Source, v, w.Decimals)
	}
	numCol := r.fg
	if w.Tint {
		numCol = col
	}
	r.drawValueAt(img, cx, cy+size*0.1, size*0.29, val, unit, numCol, col)
	if w.Label != "" {
		ls := size * 0.085
		for ls > 8 && measureTracked(r.face(true, ls), w.Label)+r.inlineExtra("label", ls) > size*0.5 {
			ls *= 0.95
		}
		r.drawLabelAt(img, r.face(true, ls), cx, cy-size*0.16, w.Label, r.mut)
	}
	if w.SubSource != "" {
		sv, sunit, sok := r.reading(p, w.SubSource, w.SubUnit)
		s := "--"
		if sok {
			s = r.num(w.SubSource, sv, w.SubDecimals)
		}
		subSize, subY := size*0.1, size*0.31
		if _, _, ok := r.inlineIcon("sub"); ok {
			// icon and text are wider: sit in the ring's chord, above its ends
			subSize, subY = size*0.09, size*0.235
		}
		r.drawSubAt(img, r.face(true, subSize), cx, cy+subY, s+sunit, r.sub)
	}
}

func (r *Renderer) value(img *image.RGBA, w Widget, rect image.Rectangle, p metrics.Provider) {
	if w.Align == "center" {
		r.valueCentered(img, w, rect, p)
		return
	}
	v, unit, ok := r.reading(p, w.Source, w.Unit)
	col := r.colorFor(w, v, ok)
	h := float64(rect.Dy())
	pad := h * 0.14
	lf := r.face(true, h*0.2)
	r.drawTracked(img, lf, float64(rect.Min.X)+pad, float64(rect.Min.Y)+pad+h*0.16, w.Label, r.mut)

	val := "--"
	if ok {
		val = r.num(w.Source, v, w.Decimals)
	}
	size := h * 0.42
	// Shrink to fit the tile width.
	for size > 8 && r.valueWidth(size, val, unit)+pad*2.3 > float64(rect.Dx()) {
		size -= 2
	}
	base := float64(rect.Max.Y) - pad
	x := float64(rect.Min.X) + pad
	r.drawValue(img, x+r.valueWidth(size, val, unit)/2, base, size, val, unit, col, col)
}

// ValueStat is how one centred value widget was laid out in the last drawn
// frame, in output pixels: for tests and design tools that compare the type
// of several cells.
type ValueStat struct {
	Widget    int
	Label     string
	LabelSize float64         // font size of the label
	LabelBase float64         // baseline of the label
	LabelW    float64         // label width including an inline icon
	ValueSize float64         // font size of the number
	ValueBase float64         // baseline of the number
	DigitH    float64         // digit height of the number
	ValueW    float64         // number and unit width
	SubSize   float64         // font size of the second value, 0 = none
	SubBase   float64         // its baseline
	Rect      image.Rectangle // the widget
}

// ValueStats lists the centred values of the last drawn frame.
func (r *Renderer) ValueStats() []ValueStat { return append([]ValueStat(nil), r.vstats...) }

// valueCentered stacks label, big value and an optional second value,
// centred in the widget: made for the middle of round screens.
func (r *Renderer) valueCentered(img *image.RGBA, w Widget, rect image.Rectangle, p metrics.Provider) {
	v, unit, ok := r.reading(p, w.Source, w.Unit)
	col := r.colorFor(w, v, ok)
	h := float64(rect.Dy())
	cx := float64(rect.Min.X) + float64(rect.Dx())/2
	top := float64(rect.Min.Y)
	sub := w.SubSource != ""

	labelSize, valSize := h*0.22, h*0.55
	if sub {
		labelSize, valSize = h*0.18, h*0.46
	}
	if w.Label == "" {
		labelSize = 0
		valSize = h * 0.55
	}
	if w.Uniform && w.ValueSize > 0 {
		valSize = w.ValueSize
	}
	if w.Size > 0 && (w.Size < labelSize || w.Uniform) {
		labelSize = w.Size // designs may ask for a smaller label (long label, narrow ring); uniform ones for an exact one
	}
	// A uniform cell centres label, number and the second value (which
	// follows the number at a fixed gap) as one block in its widget, so
	// cells of one class line up whatever their height.
	shift, subCap, uniGap := 0.0, 0.0, 0.0
	if w.Uniform {
		if w.Size <= 0 {
			w.Size = labelSize
		}
		if w.Label != "" {
			uniGap = labelSize * 0.95 // room for an icon taller than the label
		}
		vc := digitHeight(r.face(true, valSize))
		mb := top + labelSize*0.95 + uniGap + vc
		blockTop, blockBot := mb-vc, mb
		if w.Label != "" {
			blockTop = math.Min(blockTop, top+labelSize*0.95-digitHeight(r.face(true, labelSize)))
		}
		if sub {
			subCap = digitHeight(r.face(true, w.Size))
			blockBot = mb + 0.32*valSize + subCap
		}
		shift = (h-(blockBot-blockTop))/2 - (blockTop - top)
	}
	labelBase, labelBelow := top+labelSize*0.95+shift, 0.0 // labelBelow: how far an inline icon hangs below the label
	st := ValueStat{Widget: r.cur, Label: w.Label}
	if w.Label != "" {
		ls := labelSize
		for !w.Uniform && ls > 8 && measureTracked(r.face(true, ls), w.Label)+r.inlineExtra("label", ls) > float64(rect.Dx())*0.92 {
			ls *= 0.95 // long labels shrink instead of clipping
		}
		r.drawLabelAt(img, r.face(true, ls), cx, labelBase, w.Label, col)
		if _, iw, ok := r.inlineIcon("label"); ok {
			cap := digitHeight(r.face(true, ls))
			labelBelow = math.Max(0, (r.inlineSize(iw, cap)-cap)/2)
		}
		st.LabelSize, st.LabelBase = ls/r.ss, labelBase/r.ss
		st.LabelW = (measureTracked(r.face(true, ls), w.Label) + r.inlineExtra("label", ls)) / r.ss
	}
	val := "--"
	if ok {
		dec := w.Decimals
		if w.AutoDecimals {
			dec = 0
			if math.Abs(v) < 9.95 {
				dec = 1
			}
		}
		val = r.num(w.Source, v, dec)
	}
	numCol := r.fg
	if w.Tint {
		numCol = col
	}
	// Shrink long values (e.g. "1599 rpm") to the widget width, keeping the
	// baseline where the full-size number would sit.
	fit := valSize
	for !w.Uniform && fit > 8 && r.valueWidth(fit, val, unit)+r.inlineExtra("main", fit) > float64(rect.Dx())*0.94 {
		fit *= 0.95
	}
	mainBase := top + labelSize*1.05 + valSize*0.82 + shift
	if w.Uniform {
		mainBase = top + labelSize*0.95 + uniGap + digitHeight(r.face(true, valSize)) + shift
	}
	// an icon in front of the label keeps 16 px from the digits below it
	if labelBelow > 0 && !w.Uniform {
		mainBase += math.Max(0, labelBase+labelBelow+18*r.ss-(mainBase-digitHeight(r.face(true, fit))))
	}
	st.ValueSize, st.ValueBase, st.DigitH = fit/r.ss, mainBase/r.ss, digitHeight(r.face(true, fit))/r.ss
	st.ValueW = (r.valueWidth(fit, val, unit) + r.inlineExtra("main", fit)) / r.ss
	st.Rect = image.Rect(int(float64(rect.Min.X)/r.ss), int(float64(rect.Min.Y)/r.ss), int(float64(rect.Max.X)/r.ss), int(float64(rect.Max.Y)/r.ss))
	r.vstats = append(r.vstats, st)
	vi := len(r.vstats) - 1
	r.drawValueAt(img, cx, mainBase, fit, val, unit, numCol, col)
	if sub {
		sv, sunit, sok := r.reading(p, w.SubSource, w.SubUnit)
		s := "--"
		if sok {
			s = r.num(w.SubSource, sv, w.SubDecimals)
		}
		subSize := h * 0.17
		if _, _, ok := r.inlineIcon("sub"); ok {
			subSize = h * 0.15 // the icon takes room: a shorter line keeps clear of the edges
		}
		subBase := float64(rect.Max.Y) - h*0.03
		if subSize != h*0.17 {
			subBase = float64(rect.Max.Y) - h*0.075 // icon and text sit higher, off the card edge
		}
		if w.Uniform && w.Size > 0 {
			subSize, subBase = w.Size, mainBase+0.32*valSize+subCap // as big as the label, always
		}
		r.drawSubAt(img, r.face(true, subSize), cx, subBase, s+sunit, r.sub)
		r.vstats[vi].SubSize, r.vstats[vi].SubBase = subSize/r.ss, subBase/r.ss
	}
}

func (r *Renderer) bar(img *image.RGBA, w Widget, rect image.Rectangle, p metrics.Provider) {
	v, unit, ok := r.reading(p, w.Source, w.Unit)
	col := r.colorFor(w, v, ok)
	h := float64(rect.Dy())
	pad := h * 0.18
	bh := int(h * 0.2)
	track := image.Rect(rect.Min.X+int(pad), rect.Max.Y-int(pad)-bh, rect.Max.X-int(pad), rect.Max.Y-int(pad))
	if w.Label != "" {
		f := r.face(true, h*0.28)
		top := float64(rect.Min.Y) + pad + h*0.22
		r.drawTracked(img, f, float64(rect.Min.X)+pad, top, w.Label, r.mut)
		val := "--"
		if ok {
			val = r.num(w.Source, v, w.Decimals) + spacedUnit(unit)
		}
		r.drawString(img, f, float64(rect.Max.X)-pad-float64(measure(f, val)), top, val, r.fg)
	} else {
		// Without a label the bar is just the meter, filling the height.
		track = rect.Inset(int(pad * 0.5))
		bh = track.Dy()
	}
	if w.Segments > 0 {
		// LED-meter style: discrete blocks, lit up to the value.
		n := w.Segments
		gap := float64(track.Dx()) * 0.012
		bw := (float64(track.Dx()) - gap*float64(n-1)) / float64(n)
		lit := 0
		if ok {
			lit = int(math.Round(r.fraction(w, v) * float64(n)))
		}
		for i := 0; i < n; i++ {
			x0 := float64(track.Min.X) + float64(i)*(bw+gap)
			b := image.Rect(int(x0), track.Min.Y, int(x0+bw), track.Max.Y)
			c := withAlpha(r.mut, 0.25)
			if i < lit {
				c = col
			}
			draw.Draw(img, b, image.NewUniform(c), image.Point{}, draw.Over)
		}
		return
	}
	roundRect(img, track, float64(bh)/2, withAlpha(r.mut, 0.35))
	if ok {
		fill := track
		fill.Max.X = fill.Min.X + int(float64(track.Dx())*r.fraction(w, v))
		if fill.Dx() >= bh {
			roundRect(img, fill, float64(bh)/2, col)
		}
	}
}

func (r *Renderer) graph(img *image.RGBA, w Widget, rect image.Rectangle, p metrics.Provider) {
	v, unit, ok := r.rawReading(p, w.Source, w.Unit)
	col := r.colorFor(w, v, ok)
	h := float64(rect.Dy())
	pad := 12.0 * r.ss
	lf := r.face(true, h*0.2)
	vf := r.face(true, h*0.26)
	// label and value share one baseline
	gbase := float64(rect.Min.Y) + pad + float64(max(lf.Metrics().Ascent.Ceil(), vf.Metrics().Ascent.Ceil()))
	r.drawTracked(img, lf, float64(rect.Min.X)+pad, gbase, w.Label, r.mut)
	if ok {
		val := r.num(w.Source, v, w.Decimals) + spacedUnit(unit)
		r.drawString(img, vf, float64(rect.Max.X)-pad-float64(measure(vf, val)), gbase, val, col)
	}

	samples := r.hist.Get(w.Source)
	area := image.Rect(rect.Min.X+int(pad), rect.Min.Y+int(h*0.4), rect.Max.X-int(pad), rect.Max.Y-int(pad))
	if len(samples) == 1 {
		samples = []float64{samples[0], samples[0]}
	}
	if len(samples) < 2 || area.Dy() < 4 {
		return
	}
	// until the history is full the samples are spread over the whole width
	// (the curve is there from the first values on); then it scrolls
	n := min(r.L.HistorySize, max(len(samples), 2))
	if len(samples) >= r.L.HistorySize {
		n = r.L.HistorySize
	}
	step := float64(area.Dx()) / float64(n-1)
	off := n - len(samples)
	pts := make([]image.Point, len(samples))
	for i, s := range samples {
		x := float64(area.Min.X) + float64(off+i)*step
		y := float64(area.Max.Y) - r.fraction(w, s)*float64(area.Dy())
		pts[i] = image.Pt(int(math.Round(x)), int(math.Round(y)))
	}
	// Gradient fill under the curve: 35% alpha at the line, fading to 0.
	for i := 1; i < len(pts); i++ {
		for x := pts[i-1].X; x < pts[i].X; x++ {
			t := float64(x-pts[i-1].X) / float64(pts[i].X-pts[i-1].X)
			top := float64(pts[i-1].Y) + t*float64(pts[i].Y-pts[i-1].Y)
			span := float64(area.Max.Y) - top
			for y := int(top); y < area.Max.Y; y++ {
				a := 0.35 * (1 - (float64(y)-top)/span)
				blendPixel(img, x, y, col, a)
			}
		}
	}
	polyline(img, pts, 3*r.ss, col)
}

// spacedUnit puts a space before a unit made of letters (W, ms, rpm), so
// "140 W" reads the same everywhere; % and the degree sign stay tight.
func spacedUnit(u string) string {
	if u == "" || u[0] == ' ' || u[0] == '%' || strings.HasPrefix(u, "°") {
		return u
	}
	return " " + u
}

func (r *Renderer) text(img *image.RGBA, w Widget, rect image.Rectangle, s string) {
	size := w.Size
	if size == 0 {
		size = float64(rect.Dy()) * 0.7
	}
	f := r.face(true, size)
	for size > 8 && float64(measure(f, s)) > float64(rect.Dx()) {
		size *= 0.94 // shrink to fit, e.g. "11:45 PM" where "23:45" fits
		f = r.face(true, size)
	}
	col := parseHex(w.Color, r.fg)
	cx := float64(rect.Min.X) + float64(rect.Dx())/2
	m := f.Metrics()
	base := float64(rect.Min.Y) + (float64(rect.Dy())+float64(m.Ascent.Ceil()-m.Descent.Ceil()))/2
	r.drawCentered(img, f, cx, base, s, col)
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}

func measure(f font.Face, s string) int {
	if !strings.Contains(s, "°") {
		return font.MeasureString(f, s).Ceil()
	}
	_, _, adv := degreeSize(f)
	parts := strings.Split(s, "°")
	w := adv * float64(len(parts)-1)
	for _, p := range parts {
		w += float64(font.MeasureString(f, p)) / 64
	}
	return int(math.Ceil(w))
}

func (r *Renderer) drawString(img *image.RGBA, f font.Face, x, baseline float64, s string, c color.Color) {
	if strings.Contains(s, "°") {
		_, _, adv := degreeSize(f)
		parts := strings.Split(s, "°")
		for i, p := range parts {
			if i > 0 {
				drawDegree(img, f, x, baseline, c)
				x += adv
			}
			r.drawString(img, f, x, baseline, p, c)
			x += float64(font.MeasureString(f, p)) / 64
		}
		return
	}
	d := font.Drawer{Dst: img, Src: image.NewUniform(c), Face: f,
		Dot: fixed.Point26_6{X: fixed.Int26_6(x * 64), Y: fixed.Int26_6(baseline * 64)}}
	d.DrawString(s)
}

func (r *Renderer) drawCentered(img *image.RGBA, f font.Face, cx, baseline float64, s string, c color.Color) {
	r.drawString(img, f, cx-float64(measure(f, s))/2, baseline, strings.TrimSpace(s), c)
}

func mix(a, b color.RGBA, t float64) color.RGBA {
	t = clamp01(t)
	l := func(x, y uint8) uint8 { return uint8(float64(x) + (float64(y)-float64(x))*t) }
	return color.RGBA{l(a.R, b.R), l(a.G, b.G), l(a.B, b.B), 255}
}

func blendPixel(img *image.RGBA, x, y int, c color.RGBA, a float64) {
	if !(image.Point{x, y}.In(img.Rect)) || a <= 0 {
		return
	}
	i := img.PixOffset(x, y)
	for k, v := range []uint8{c.R, c.G, c.B} {
		img.Pix[i+k] = uint8(float64(img.Pix[i+k])*(1-a) + float64(v)*a)
	}
}

// drawValue draws a big number with its unit, centred on cx. Digits sit on
// fixed-width cells (the font has no tabular figures) so values don't jitter
// sideways as they change; the unit is bold at 42% of the number size.
// unitScale is the unit size relative to the number it follows.
const unitScale = 0.5

// valueWidth is how wide drawValue draws val+unit at size.
func (r *Renderer) valueWidth(size float64, val, unit string) float64 {
	w := measureNum(r.face(true, size), val)
	if unit == "" {
		return w
	}
	if strings.HasPrefix(unit, "°") {
		rest := strings.TrimPrefix(unit, "°")
		return w + r.degreeWidth(size) + float64(measure(r.face(true, size*unitScale), rest))
	}
	return w + float64(measure(r.face(true, size*unitScale), unit)) + size*0.07
}

// digitHeight is the height of the digits of f in pixels.
func digitHeight(f font.Face) float64 {
	b, _, ok := f.GlyphBounds('0')
	if !ok {
		return float64(f.Metrics().Ascent) / 64 * 0.7
	}
	return float64(-b.Min.Y) / 64
}

// degreeSize is the diameter, ring thickness and advance of a drawn degree
// sign: about 40 % of the digit height, a proper ring instead of the font's
// tiny raised glyph.
func degreeSize(f font.Face) (d, thick, adv float64) {
	d = digitHeight(f) * 0.4
	return d, math.Max(d*0.26, 1.5), d + math.Max(d*0.3, 2)
}

func (r *Renderer) degreeWidth(size float64) float64 {
	_, _, adv := degreeSize(r.face(true, size))
	return adv
}

// drawDegree draws the ring at the top of the digits.
func drawDegree(img *image.RGBA, f font.Face, x, baseline float64, c color.Color) {
	d, thick, _ := degreeSize(f)
	cy := baseline - digitHeight(f) + d/2
	arc(img, x+d/2, cy, d/2-thick/2, thick, 0, 360, c)
}

func (r *Renderer) drawValue(img *image.RGBA, cx, baseline, size float64, val, unit string, c, uc color.Color) {
	big := r.face(true, size)
	small := r.face(true, size*unitScale)
	gap := size * 0.07
	deg := strings.HasPrefix(unit, "°")
	rest := unit
	uw := 0.0
	if deg {
		rest = strings.TrimPrefix(unit, "°")
		uw = r.degreeWidth(size)
		if rest != "" {
			uw += float64(measure(small, rest))
		}
	} else if unit != "" {
		uw = float64(measure(small, unit)) + gap
	}
	vw := measureNum(big, val)
	x := cx - (vw+uw)/2
	r.drawNum(img, big, x, baseline, val, c)
	if deg {
		drawDegree(img, big, x+vw, baseline, uc)
		if rest != "" {
			r.drawString(img, small, x+vw+r.degreeWidth(size), baseline, rest, uc)
		}
	} else if unit != "" {
		r.drawString(img, small, x+vw+gap, baseline, unit, uc)
	}
}

// digitCell is the advance of "0", used for every digit.
func digitCell(f font.Face) float64 {
	a, _ := f.GlyphAdvance('0')
	return float64(a) / 64
}

func measureNum(f font.Face, s string) float64 {
	cell, w := digitCell(f), 0.0
	for _, ch := range s {
		if ch >= '0' && ch <= '9' {
			w += cell
		} else {
			a, _ := f.GlyphAdvance(ch)
			w += float64(a) / 64
		}
	}
	return w
}

func (r *Renderer) drawNum(img *image.RGBA, f font.Face, x, baseline float64, s string, c color.Color) {
	cell := digitCell(f)
	for _, ch := range s {
		g := string(ch)
		gw := float64(measure(f, g))
		adv := gw
		off := 0.0
		if ch >= '0' && ch <= '9' {
			adv = cell
			off = (cell - gw) / 2
		}
		r.drawString(img, f, x+off, baseline, g, c)
		x += adv
	}
}

// labelTracking is the extra letter spacing for all-caps labels (em).
const labelTracking = 0.06

func measureTracked(f font.Face, s string) float64 {
	em := float64(f.Metrics().Height) / 64
	n := len([]rune(s))
	if n == 0 {
		return 0
	}
	return float64(measure(f, s)) + em*labelTracking*float64(n-1)
}

func (r *Renderer) drawTracked(img *image.RGBA, f font.Face, x, baseline float64, s string, c color.Color) {
	em := float64(f.Metrics().Height) / 64
	for _, ch := range s {
		g := string(ch)
		r.drawString(img, f, x, baseline, g, c)
		x += float64(measure(f, g)) + em*labelTracking
	}
}

// drawLabel draws a tracked label centred on cx.
func (r *Renderer) drawLabel(img *image.RGBA, f font.Face, cx, baseline float64, s string, c color.Color) {
	r.drawTracked(img, f, cx-measureTracked(f, s)/2, baseline, s, c)
}

// mixOKLab blends in the OKLab space, which keeps fades between saturated
// colours vivid instead of passing through muddy greys.
func mixOKLab(a, b color.RGBA, t float64) color.RGBA {
	t = clamp01(t)
	la, aa, ba := toOKLab(a)
	lb, ab, bb := toOKLab(b)
	return fromOKLab(la+(lb-la)*t, aa+(ab-aa)*t, ba+(bb-ba)*t)
}

func srgbToLinear(c uint8) float64 {
	v := float64(c) / 255
	if v <= 0.04045 {
		return v / 12.92
	}
	return math.Pow((v+0.055)/1.055, 2.4)
}

func linearToSRGB(v float64) uint8 {
	v = clamp01(v)
	if v <= 0.0031308 {
		v *= 12.92
	} else {
		v = 1.055*math.Pow(v, 1/2.4) - 0.055
	}
	return uint8(math.Round(v * 255))
}

func toOKLab(c color.RGBA) (float64, float64, float64) {
	r, g, b := srgbToLinear(c.R), srgbToLinear(c.G), srgbToLinear(c.B)
	l := math.Cbrt(0.4122214708*r + 0.5363325363*g + 0.0514459929*b)
	m := math.Cbrt(0.2119034982*r + 0.6806995451*g + 0.1073969566*b)
	s := math.Cbrt(0.0883024619*r + 0.2817188376*g + 0.6299787005*b)
	return 0.2104542553*l + 0.7936177850*m - 0.0040720468*s,
		1.9779984951*l - 2.4285922050*m + 0.4505937099*s,
		0.0259040371*l + 0.7827717662*m - 0.8086757660*s
}

func fromOKLab(L, a, b float64) color.RGBA {
	l := math.Pow(L+0.3963377774*a+0.2158037573*b, 3)
	m := math.Pow(L-0.1055613458*a-0.0638541728*b, 3)
	s := math.Pow(L-0.0894841775*a-1.2914855480*b, 3)
	return color.RGBA{
		linearToSRGB(4.0767416621*l - 3.3077115913*m + 0.2309699292*s),
		linearToSRGB(-1.2684380046*l + 2.6097574011*m - 0.3413193965*s),
		linearToSRGB(-0.0041960863*l - 0.7034186147*m + 1.7076147010*s),
		255,
	}
}

// subFor is the second-value colour: light grey on dark, dark grey on light.
func subFor(bg color.RGBA) color.RGBA {
	if isLight(bg) {
		return color.RGBA{0x3a, 0x42, 0x52, 255}
	}
	return color.RGBA{0xc3, 0xca, 0xd6, 255}
}

// warnFor is the warning colour: amber on dark, a deeper orange on light
// backgrounds where yellow would vanish.
func warnFor(bg color.RGBA) color.RGBA {
	if isLight(bg) {
		return color.RGBA{0xd9, 0x77, 0x06, 255}
	}
	return color.RGBA{250, 176, 5, 255}
}

// segmentedArc draws the 270° gauge as separate segments (HUD style); lit
// segments get the value colour, the rest a dim track.
// gaugeArc returns where a gauge's arc starts (degrees clockwise from 3
// o'clock) and how far it goes: 270° open at the bottom by default, a full
// circle starting at the top for Sweep 360.
func gaugeArc(w Widget) (start, total float64) {
	switch {
	case w.Sweep >= 360:
		return 270, 360
	case w.Sweep > 0:
		return 90 + float64(360-w.Sweep)/2, float64(w.Sweep)
	}
	return 135, 270
}

func (r *Renderer) segmentedArc(img *image.RGBA, cx, cy, radius, thick, start, total, sweep float64, n int, col color.RGBA, glow bool) {
	step := total / float64(n)
	gapDeg := step * 0.28
	for i := 0; i < n; i++ {
		a0 := start + float64(i)*step
		c := withAlpha(r.mut, 0.22)
		on := float64(i)*step < sweep-step*0.15
		if on {
			c = col
			if glow {
				arcFlat(img, cx, cy, radius, thick*1.9, a0, step-gapDeg, withAlpha(col, 0.12))
			}
		}
		arcFlat(img, cx, cy, radius, thick, a0, step-gapDeg, c)
	}
}

// ticks draws n+1 short radial scale marks along the gauge's arc.
func (r *Renderer) ticks(img *image.RGBA, cx, cy, radius, length float64, n int, start, total float64) {
	c := withAlpha(r.mut, 0.6)
	if total >= 360 {
		n-- // the last mark would sit on the first
	}
	for i := 0; i <= n; i++ {
		a := (start + total*float64(i)/float64(n+btoi(total >= 360))) * math.Pi / 180
		l := length
		if i%5 != 0 {
			l *= 0.55
		}
		p0 := image.Pt(int(cx+radius*math.Cos(a)), int(cy+radius*math.Sin(a)))
		p1 := image.Pt(int(cx+(radius+l)*math.Cos(a)), int(cy+(radius+l)*math.Sin(a)))
		polyline(img, []image.Point{p0, p1}, 1.6*r.ss, c)
	}
}

// scanlines darkens every other line like a CRT screen.
func scanlines(img *image.RGBA, period int) {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		if (y/(period/2))%2 == 0 {
			continue
		}
		row := img.Pix[(y-b.Min.Y)*img.Stride : (y-b.Min.Y)*img.Stride+b.Dx()*4]
		for i := 0; i < len(row); i += 4 {
			row[i] = uint8(uint16(row[i]) * 3 / 5)
			row[i+1] = uint8(uint16(row[i+1]) * 3 / 5)
			row[i+2] = uint8(uint16(row[i+2]) * 3 / 5)
		}
	}
}

func insideAny(r image.Rectangle, rs []image.Rectangle) bool {
	for _, o := range rs {
		if r.In(o) {
			return true
		}
	}
	return false
}
