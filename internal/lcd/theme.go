package lcd

import (
	"image/color"
	"math"
)

// Themes: designs are authored dark; ApplyTheme derives the light variant
// (for white builds) at render time, so every design, including user ones,
// works in both.

// ApplyTheme returns l restyled for theme ("light" or anything else = as
// designed).
func ApplyTheme(l Layout, theme string) Layout {
	if theme != "light" {
		return l
	}
	out := l
	out.Background = "#f3f5f9"
	out.Foreground = "#0f141c"
	out.Muted = "#5b6474"
	if l.Panel != "" {
		out.Panel = "#e3e8ef"
	}
	out.Accent = forLight(l.Accent)
	if l.Set == "retro" {
		// Warm paper instead of cold grey, like an amber terminal printout.
		out.Background, out.Foreground, out.Muted = "#efe6d0", "#5a3a00", "#8a6a30"
	}
	if l.Set == "cyberpunk" {
		out.Foreground = "#111111"
	}
	out.Widgets = make([]Widget, len(l.Widgets))
	for i, w := range l.Widgets {
		if w.Color != "" {
			w.Color = forLight(w.Color)
		}
		w.Glow = false // halos turn into smudges on white
		if l.Set == "cyberpunk" {
			w.Tint = false // olive numbers are weak on white: black numbers, coloured rings
		}
		out.Widgets[i] = w
	}
	return out
}

// forLight keeps a colour's hue but lowers its lightness until it has
// enough contrast on a near-white background; near-white colours (e.g. the
// Minimal ring) become near-black.
func forLight(hex string) string {
	if hex == "" {
		return hex
	}
	c := parseHex(hex, color.RGBA{0x22, 0xd3, 0xee, 0xff})
	L, a, b := toOKLab(c)
	chroma := math.Hypot(a, b)
	switch {
	case chroma < 0.04 && L > 0.6: // greys/whites
		L = 0.25
	case L > 0.62:
		L = 0.55
	}
	o := fromOKLab(L, a, b)
	return "#" + hexByte(o.R) + hexByte(o.G) + hexByte(o.B)
}

func hexByte(v uint8) string {
	const digits = "0123456789abcdef"
	return string([]byte{digits[v>>4], digits[v&15]})
}

// isLight reports whether a background colour is light.
func isLight(c color.RGBA) bool {
	L, _, _ := toOKLab(c)
	return L > 0.7
}
