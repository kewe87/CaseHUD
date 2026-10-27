package lcd

import (
	"slices"
)

// Cell is one field of a grid, in panel pixels.
type Cell struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

// GridPreset divides a bar display into fields, each of which behaves like a
// small display of its own.
type GridPreset struct {
	ID    string `json:"id"`
	Cells []Cell `json:"cells"`
	// Small marks presets whose fields are only half as high as the bar
	// (readable from close by).
	Small bool `json:"small,omitempty"`
}

// GridIDs lists the presets in the order they are offered. "4" is the main
// variant: four 480x480 squares side by side.
var GridIDs = []string{"4", "3", "2", "2+4", "1+6", "8"}

// GridPresets returns the grid layouts for a bar display of w x h: landscape
// (about 4:1) or portrait (1:4). The base is a unit of 480 px: a field is a
// square, or a half-height strip of a square (480x240); fields touch, the
// designs bring their own margins. In portrait the presets are the landscape
// ones turned upright: squares and the 3/2 splits are stacked, and half
// fields stay 480 wide (stacked) so they remain readable. Other sizes get
// nil.
func GridPresets(w, h int) []GridPreset {
	switch {
	case w == 4*h && h > 0:
		return buildGrids(h, false)
	case h == 4*w && w > 0:
		return buildGrids(w, true)
	}
	return nil
}

func buildGrids(u int, portrait bool) []GridPreset {
	half := u / 2
	third := u * 4 / 3
	var out []GridPreset
	add := func(id string, small bool, cells ...Cell) {
		if portrait {
			cells = portraitCells(u, cells)
		}
		out = append(out, GridPreset{ID: id, Cells: cells, Small: small})
	}
	row := func(n, w int) []Cell {
		cs := make([]Cell, n)
		for i := range cs {
			cs[i] = Cell{i * w, 0, w, u}
		}
		return cs
	}
	// halves fills columns col0.. with two stacked half fields each, row by row.
	halves := func(col0, cols int) []Cell {
		var cs []Cell
		for r := 0; r < 2; r++ {
			for c := 0; c < cols; c++ {
				cs = append(cs, Cell{(col0 + c) * u, r * half, u, half})
			}
		}
		return cs
	}
	add("4", false, row(4, u)...)
	add("3", false, row(3, third)...)
	add("2", false, row(2, 2*u)...)
	add("2+4", false, append(row(2, u), halves(2, 2)...)...)
	add("1+6", false, append(row(1, u), halves(1, 3)...)...)
	add("8", true, halves(0, 4)...)
	return out
}

// portraitCells turns landscape cells upright. Full-height cells swap axes
// (a row becomes a stack); half fields stay u wide and are stacked below the
// full ones, so the whole bar is still covered: the landscape strip of u/2
// high fields has 4 u of width, i.e. in portrait they run down in a single
// lane of u wide, u/2 high fields.
func portraitCells(u int, cells []Cell) []Cell {
	var out []Cell
	y := 0
	for _, c := range cells {
		if c.H == u { // full height: swap axes
			out = append(out, Cell{0, c.X, c.H, c.W})
		}
	}
	for _, c := range cells {
		if c.H == u {
			if c.X+c.W > y {
				y = c.X + c.W
			}
		}
	}
	// Halves: keep landscape reading order, stack below the full fields.
	var hs []Cell
	for _, c := range cells {
		if c.H != u {
			hs = append(hs, c)
		}
	}
	// sort by row, then column
	slices.SortStableFunc(hs, func(a, b Cell) int {
		if a.Y != b.Y {
			return a.Y - b.Y
		}
		return a.X - b.X
	})
	for _, c := range hs {
		out = append(out, Cell{0, y, c.W, c.H})
		y += c.H
	}
	return out
}

// GridPreset looks up one preset by id.
func GridPresetByID(w, h int, id string) (GridPreset, bool) {
	for _, g := range GridPresets(w, h) {
		if g.ID == id {
			return g, true
		}
	}
	return GridPreset{}, false
}

// gridCells is how many fields each preset has.
var gridCells = map[string]int{"4": 4, "3": 3, "2": 2, "2+4": 6, "1+6": 7, "8": 8}

// GridCellCount is the number of fields of preset id (0 = unknown preset).
// It is the same for the landscape and the upright bar.
func GridCellCount(id string) int { return gridCells[id] }
