package app

import "github.com/kewe87/casehud/internal/lcd"

// Cell typography of the grid fields: one place for the numbers every cell of
// a class shares, so neighbouring cells line up whatever they show.
//
// Half fields (480x240, a card of 456x216 at 12,12) hold one reading
// (halfSingle) or two side by side (halfCols). Every reading is a cell:
//
//	label    labelSize px, tracked capitals, at the cell's top
//	number   digits >= 60 px high on one baseline for all cells, unit half as big
//	bar      16 px high on the same line in every cell
//
// Rules for the combined cells (designs_combo.go):
//
//   - Two equal columns of 206 px at x=26 and x=248; the bar is as wide as
//     its column, so the number never reaches past it.
//   - Exactly one icon per card: in front of the label of the first reading,
//     it follows that reading (chip, temp, pump, power, memory, clock, fps).
//   - Labels are at most 9 characters (8 in front of the icon, width permitting): a subject and
//     a metric (CPU LOAD, GPU TEMP, PUMP RPM, GPU POWER), the same in every
//     style. Nothing shrinks to fit: cells are drawn Uniform.
//   - Decimals per kind: % and W and MHz and rpm and degrees 0, coolant
//     degrees 1, GB and ms one decimal below 10, none from 10 on.
const (
	cellLabelSize = 30.0 // label font size
	cellValueY    = 24   // the label/number block is centred in this widget
	cellValueH    = 140
	cellValueSize = 94.0 // number font size: digits ~60.5 px
	cellBarY      = 182
	cellBarH      = 16
	cellColW      = 206 // width of a column of a two-reading cell: text and bar alike
)

// cellIconSizes are the icon edges: the drawings fill their square to a
// different extent, so each kind gets the edge at which they look equally big.
var cellIconSizes = map[string]int{"chip": 44, "temp": 44, "pump": 44, "power": 44, "memory": 44, "clock": 44, "fps": 44}

// halfCol is where a reading sits inside a half field.
type halfCol struct {
	dy         int // offset of the cell from the top of a half field (a half field in the lower half of a card)
	x, w       int // text block
	barX, barW int
}

var (
	halfSingle = halfCol{x: 28, w: 424, barX: 40, barW: 400}
	halfCols   = [2]halfCol{{x: 26, w: cellColW, barX: 26, barW: cellColW}, {x: 248, w: cellColW, barX: 248, barW: cellColW}}
)

// halfCell is one reading of a half field: label (with the cell's icon when
// icon is set and the reading has one), number and bar.
func (s tileStyle) halfCell(r tileReading, c halfCol, icon bool) []lcd.Widget {
	v := s.thresholds(r, lcd.Widget{Type: "value", Align: "center", X: c.x, Y: c.dy + cellValueY, W: c.w, H: cellValueH,
		Label: r.label, Size: cellLabelSize, ValueSize: cellValueSize, Uniform: true, AutoDecimals: r.auto, Tint: true, Smooth: true})
	out := []lcd.Widget{v}
	if icon {
		if sp, ok := iconSpecFor(r); ok {
			out = append(out, s.inlineIcon(sp, "label", cellIconSizes[sp.icon]))
		}
	}
	b := r
	if r.barSrc != "" {
		b.src, b.min, b.max, b.warn, b.crit, b.unit, b.dec = r.barSrc, 0, 100, 90, 97, "%", 0
	}
	return append(out, s.bar(b, c.barX, c.dy+cellBarY, c.barW, cellBarH))
}

// Square cards (480x480 and the squares of the wide fields) and the value
// cards of the wide fields share one scale, bigger than the half fields:
//
//	label          cardLabelSize px, the second value as big
//	number         cardRingValue px (inside a ring, beside a bar) or
//	               cardFlatValue px (a card without ring), digits ~0.72 x that
//	icon           in front of the label, or of the second value when the card
//	               shows rpm there (the pump)
//
// Label, number and second value are one block, centred in the cell.
const (
	cardLabelSize = 44.0
	cardRingValue = 112.0
	cardFlatValue = 140.0
	cardSubValue  = 56.0 // the unlabelled readings under a ring
)

// cardValue is the label/number/second-value block of a square card or wide
// field: ring is true for the number inside a ring.
func (s tileStyle) cardValue(r tileReading, x, y, w, h int, ring bool) lcd.Widget {
	size := cardFlatValue
	if ring {
		size = cardRingValue
	}
	v := s.thresholds(r, lcd.Widget{Type: "value", Align: "center", X: x, Y: y, W: w, H: h, Label: r.label,
		Size: cardLabelSize, ValueSize: size, Uniform: true, Tint: true, Smooth: true,
		SubSource: r.sub, SubUnit: r.subUnit, SubDecimals: r.subDec})
	return v
}

// cardIcon is the icon of reading r on a card, sized for the card scale.
func (s tileStyle) cardIcon(r tileReading, target string) []lcd.Widget {
	if sp, ok := iconSpecFor(r); ok {
		return []lcd.Widget{s.inlineIcon(sp, target, cellIconSizes[sp.icon]*6/5)}
	}
	return nil
}
