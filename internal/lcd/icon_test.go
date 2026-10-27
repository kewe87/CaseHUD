package lcd

import (
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kewe87/casehud/internal/metrics"
)

type styleSpec struct{ set, bg, fg, col, panel string }

var iconStyles = []styleSpec{
	{"clean", "#000000", "#f1f4fa", "#22d3ee", "#10141c"},
	{"neon", "#04050c", "#eaf6ff", "#22e5ff", "#0a0d1c"},
	{"retro", "#0b0700", "#ffb000", "#ffb000", ""},
	{"cyberpunk", "#050508", "#fcee0a", "#fcee0a", ""},
}

var t0 = time.Date(2026, 10, 20, 14, 8, 30, 0, time.UTC)

func iconRenderer(t testing.TB, st styleSpec, icon string, size int, anim bool) *Renderer {
	t.Helper()
	l := Layout{Width: size, Height: size, IntervalMS: 1000, HistorySize: 2, Set: st.set,
		Background: st.bg, Foreground: st.fg, Muted: "#8a94a8", Accent: st.col, Panel: st.panel,
		Widgets: []Widget{{Type: "icon", Icon: icon, X: 0, Y: 0, W: size, H: size, Source: "x", Max: 100, Color: st.col, Warn: 85, Crit: 95}}}
	r, err := NewRenderer(l)
	if err != nil {
		t.Fatal(err)
	}
	now := t0
	r.Now = func() time.Time { return now }
	if anim {
		r.SetAnimation(100 * time.Millisecond)
	}
	r.testClock = &now
	return r
}

func iconFrames(t testing.TB, st styleSpec, icon string, size int, val float64, n int, step time.Duration) []*image.RGBA {
	r := iconRenderer(t, st, icon, size, true)
	p := &fixedProvider{v: val}
	var out []*image.RGBA
	for i := 0; i < n; i++ {
		out = append(out, r.Render(p))
		*r.testClock = r.testClock.Add(step)
	}
	return out
}

func TestIconGeometryAndNonEmpty(t *testing.T) {
	for _, st := range iconStyles {
		for _, name := range IconNames {
			const size, pad = 96, 40
			// the icon widget sits in a padded image: nothing may leak outside its rect
			l := Layout{Width: size + 2*pad, Height: size + 2*pad, HistorySize: 2, Set: st.set, Background: "#000000", Accent: st.col,
				Widgets: []Widget{{Type: "icon", Icon: name, X: pad, Y: pad, W: size, H: size, Source: "x", Max: 100, Color: st.col}}}
			r, _ := NewRenderer(l)
			r.SetAnimation(time.Second)
			now := t0
			r.Now = func() time.Time { return now }
			p := &fixedProvider{v: 80}
			for f := 0; f < 20; f++ {
				img := r.Render(p)
				now = now.Add(40 * time.Millisecond)
				inside := 0
				for y := 0; y < img.Bounds().Dy(); y++ {
					for x := 0; x < img.Bounds().Dx(); x++ {
						c := img.RGBAAt(x, y)
						if c.R|c.G|c.B == 0 {
							continue
						}
						if x < pad || x >= pad+size || y < pad || y >= pad+size {
							t.Fatalf("%s/%s frame %d: pixel at %d,%d outside icon rect", st.set, name, f, x, y)
						}
						inside++
					}
				}
				if inside < size*size/40 {
					t.Fatalf("%s/%s frame %d: icon nearly empty (%d px)", st.set, name, f, inside)
				}
			}
		}
	}
}

func TestIconDeterministicAndMoving(t *testing.T) {
	for _, name := range IconNames {
		size := 80
		if name == "temp" {
			size = 120 // bubbles and heat shimmer only appear on larger icons
		}
		a := iconFrames(t, iconStyles[0], name, size, 70, 6, 40*time.Millisecond)
		b := iconFrames(t, iconStyles[0], name, size, 70, 6, 40*time.Millisecond)
		moved := false
		for i := range a {
			if string(a[i].Pix) != string(b[i].Pix) {
				t.Fatalf("%s frame %d not deterministic", name, i)
			}
			if i > 0 && string(a[i].Pix) != string(a[i-1].Pix) {
				moved = true
			}
		}
		if !moved {
			t.Errorf("%s does not animate", name)
		}
	}
}

func TestIconStaticWithoutAnimation(t *testing.T) {
	r := iconRenderer(t, iconStyles[0], "fan", 80, false)
	p := &fixedProvider{v: 70}
	a := r.Render(p)
	*r.testClock = r.testClock.Add(time.Second)
	b := r.Render(p)
	if string(a.Pix) != string(b.Pix) {
		t.Error("icon moves although animations are off")
	}
}

func TestIconSpeedMapping(t *testing.T) {
	for _, n := range []string{"fan", "pump", "flow"} {
		if iconRate(n, 0) != 0 {
			t.Errorf("%s should stand still at 0", n)
		}
		if !(iconRate(n, 0.2) < iconRate(n, 0.6) && iconRate(n, 0.6) < iconRate(n, 1)) {
			t.Errorf("%s speed not monotonic", n)
		}
	}
	// no wagon-wheel: at 25 fps and full speed a rotor turns less than half a pitch per frame
	if deg := iconRate("fan", 1) * 360 / 25; deg > 36 {
		t.Errorf("fan turns %.1f deg per frame", deg)
	}
	// speed follows the reading: after one second 100 % turns further than 20 %
	turn := func(v float64) float64 {
		r := iconRenderer(t, iconStyles[0], "fan", 64, true)
		p := &fixedProvider{v: v}
		for i := 0; i < 25; i++ {
			r.Render(p)
			*r.testClock = r.testClock.Add(40 * time.Millisecond)
		}
		return r.icons[0].phase
	}
	if a, b := turn(20), turn(100); !(b > a) {
		t.Errorf("phase 20%%=%.2f 100%%=%.2f", a, b)
	}
}

func TestIconSplitAndOverlay(t *testing.T) {
	r := iconRenderer(t, iconStyles[0], "chip", 96, true)
	r.SplitIcons = true
	p := &fixedProvider{v: 90}
	hud := r.Render(p)
	for i := 0; i < len(hud.Pix); i += 4 {
		if hud.Pix[i]|hud.Pix[i+1]|hud.Pix[i+2] != 0 {
			t.Fatal("split HUD must not contain the icon")
		}
	}
	if !r.AnimatedIcons() || len(r.IconRects()) != 1 {
		t.Fatal("icons not reported")
	}
	img := r.ComposeIcons(hud, p)
	if string(img.Pix) == string(hud.Pix) {
		t.Fatal("overlay drew nothing")
	}
	for i := 0; i < len(hud.Pix); i += 4 {
		if hud.Pix[i]|hud.Pix[i+1]|hud.Pix[i+2] != 0 {
			t.Fatal("ComposeIcons modified its input")
		}
	}
}

func TestIconAllocs(t *testing.T) {
	r := iconRenderer(t, iconStyles[1], "fan", 96, true)
	r.SplitIcons = true
	p := &fixedProvider{v: 70}
	hud := r.Render(p)
	dst := image.NewRGBA(hud.Bounds())
	r.DrawIcons(dst, p)
	a := testing.AllocsPerRun(20, func() {
		*r.testClock = r.testClock.Add(40 * time.Millisecond)
		r.DrawIcons(dst, p)
	})
	if a > 400 {
		t.Errorf("%v allocations per icon frame", a)
	}
}

// grid layout with 4 icons on a 1920x480 bar
func gridIconRenderer(t testing.TB) (*Renderer, *image.RGBA) {
	l := Layout{Width: 1920, Height: 480, HistorySize: 2, Set: "neon", Background: "#04050c", Accent: "#22e5ff", Panel: "#0a0d1c"}
	for i, ic := range []string{"fan", "pump", "chip", "power"} {
		l.Widgets = append(l.Widgets, Widget{Type: "icon", Icon: ic, X: i*480 + 176, Y: 40, W: 128, H: 128, Source: "x", Max: 100, Color: "#22e5ff"})
	}
	r, err := NewRenderer(l)
	if err != nil {
		t.Fatal(err)
	}
	r.SetAnimation(100 * time.Millisecond)
	r.SplitIcons = true
	now := t0
	r.Now = func() time.Time { return now }
	r.testClock = &now
	hud := r.Render(&fixedProvider{v: 70})
	return r, hud
}

func BenchmarkIcon480(b *testing.B) {
	r := iconRenderer(b, iconStyles[1], "fan", 480, true)
	r.L.Widgets[0].X, r.draftL.Widgets[0].X = 0, 0
	r.draftL.Widgets[0].W, r.draftL.Widgets[0].H = 80, 80
	r.SplitIcons = true
	p := &fixedProvider{v: 70}
	dst := r.Render(p)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		*r.testClock = r.testClock.Add(40 * time.Millisecond)
		r.DrawIcons(dst, p)
	}
}

func BenchmarkIconGrid4(b *testing.B) {
	r, hud := gridIconRenderer(b)
	p := &fixedProvider{v: 70}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		*r.testClock = r.testClock.Add(40 * time.Millisecond)
		r.ComposeIcons(hud, p)
	}
}

func BenchmarkIconGrid4InPlace(b *testing.B) {
	r, hud := gridIconRenderer(b)
	p := &fixedProvider{v: 70}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		*r.testClock = r.testClock.Add(40 * time.Millisecond)
		r.DrawIcons(hud, p)
	}
}

// TestIconGallery writes review strips and GIFs when ICON_OUT is set.
func TestIconGallery(t *testing.T) {
	out := os.Getenv("ICON_OUT")
	if out == "" {
		t.Skip("set ICON_OUT to a directory to render the gallery")
	}
	os.MkdirAll(out, 0o755)
	const size = 128
	for _, name := range IconNames {
		// strip: rows = styles, 8 frames each at 60 ms
		sheet := image.NewRGBA(image.Rect(0, 0, 8*size, len(iconStyles)*size))
		var gifs []*image.Paletted
		var delays []int
		gw := image.NewRGBA(image.Rect(0, 0, len(iconStyles)*size, size))
		for si, st := range iconStyles {
			fr := iconFrames(t, st, name, size, 75, 8, 60*time.Millisecond)
			for i, f := range fr {
				draw.Draw(sheet, image.Rect(i*size, si*size, (i+1)*size, (si+1)*size), f, image.Point{}, draw.Src)
			}
		}
		savePNG(t, filepath.Join(out, name+"-strip.png"), sheet)
		all := make([][]*image.RGBA, len(iconStyles))
		for si, st := range iconStyles {
			all[si] = iconFrames(t, st, name, size, 75, 40, 40*time.Millisecond)
		}
		for i := 0; i < 40; i++ {
			for si := range iconStyles {
				draw.Draw(gw, image.Rect(si*size, 0, (si+1)*size, size), all[si][i], image.Point{}, draw.Src)
			}
			pal := image.NewPaletted(gw.Bounds(), palette())
			draw.FloydSteinberg.Draw(pal, gw.Bounds(), gw, image.Point{})
			gifs = append(gifs, pal)
			delays = append(delays, 4)
		}
		f, _ := os.Create(filepath.Join(out, name+".gif"))
		gif.EncodeAll(f, &gif.GIF{Image: gifs, Delay: delays})
		f.Close()
		// levels: 5 reading levels, clean + neon
		lv := image.NewRGBA(image.Rect(0, 0, 5*size, len(iconStyles)*size))
		for si, st := range iconStyles {
			for li, v := range []float64{0, 25, 50, 75, 100} {
				fr := iconFrames(t, st, name, size, v, 12, 40*time.Millisecond)
				draw.Draw(lv, image.Rect(li*size, si*size, (li+1)*size, (si+1)*size), fr[11], image.Point{}, draw.Src)
			}
		}
		savePNG(t, filepath.Join(out, name+"-levels.png"), lv)
	}
}

func palette() color.Palette {
	var p color.Palette
	for r := 0; r < 6; r++ {
		for g := 0; g < 7; g++ {
			for b := 0; b < 6; b++ {
				p = append(p, color.RGBA{uint8(r * 51), uint8(g * 42), uint8(b * 51), 255})
			}
		}
	}
	return p
}

func savePNG(t testing.TB, path string, img image.Image) {
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	png.Encode(f, img)
}

var _ = metrics.NewMock

// TestIconRoundMockups renders 400x400 round fan-LCD designs with an
// animated icon instead of a background effect (ICON_OUT).
func TestIconRoundMockups(t *testing.T) {
	out := os.Getenv("ICON_OUT")
	if out == "" {
		t.Skip("set ICON_OUT")
	}
	type design struct {
		name, icon, label, unit string
		val, max                float64
		size                    int
		y                       int
	}
	designs := []design{
		{"coolant", "pump", "PUMP", "rpm", 2400, 3000, 190, 40},
		{"fps", "fps", "FPS", "fps", 144, 240, 210, 36},
		{"power", "power", "GPU POWER", "W", 280, 450, 170, 44},
		{"fan", "fan", "FAN", "rpm", 1500, 2000, 190, 40},
	}
	for _, d := range designs {
		sheet := image.NewRGBA(image.Rect(0, 0, 4*400, 400))
		for si, st := range iconStyles {
			l := Layout{Width: 400, Height: 400, HistorySize: 2, Round: true, Set: st.set, Background: st.bg, Foreground: st.fg,
				Muted: "#8a94a8", Accent: st.col,
				Widgets: []Widget{
					{Type: "icon", Icon: d.icon, X: (400 - d.size) / 2, Y: d.y, W: d.size, H: d.size, Source: "x", Max: d.max, Color: st.col},
					{Type: "value", Align: "center", X: 60, Y: d.y + d.size + 4, W: 280, H: 400 - d.y - d.size - 50, Label: d.label, Source: "x", Unit: d.unit, Max: d.max, Color: st.col},
				}}
			r, _ := NewRenderer(l)
			r.SetAnimation(100 * time.Millisecond)
			now := t0
			r.Now = func() time.Time { return now }
			p := &fixedProvider{v: d.val}
			var img *image.RGBA
			for f := 0; f < 14; f++ {
				img = r.Render(p)
				now = now.Add(40 * time.Millisecond)
			}
			for y := 0; y < 400; y++ {
				for x := 0; x < 400; x++ {
					dx, dy := float64(x)-199.5, float64(y)-199.5
					if dx*dx+dy*dy > 200*200 {
						img.SetRGBA(x, y, color.RGBA{40, 40, 44, 255})
					}
				}
			}
			draw.Draw(sheet, image.Rect(si*400, 0, si*400+400, 400), img, image.Point{}, draw.Src)
		}
		savePNG(t, filepath.Join(out, "round-"+d.name+".png"), sheet)
	}
}
