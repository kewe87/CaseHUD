package app

import (
	"testing"

	"github.com/kewe87/casehud/internal/lcd"
	"github.com/kewe87/casehud/internal/metrics"
)

// BenchmarkDesigns renders a few heavy built-in designs at TL LCD size.
func BenchmarkDesigns(b *testing.B) {
	for _, id := range []string{"cpu-gpu", "gaming", "neon-cpu-gpu", "neon-cpu", "retro-gaming", "water-cooling"} {
		b.Run(id, func(b *testing.B) {
			r, _ := lcd.NewRenderer(builtinDesigns()[id].ScaledTo(400, 400))
			p := metrics.NewMock()
			for i := 0; i < b.N; i++ {
				r.Sample(p)
				r.Render(p)
			}
		})
	}
}
