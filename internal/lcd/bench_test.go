package lcd

import (
	"testing"

	"github.com/kewe87/casehud/internal/metrics"
)

func BenchmarkRenderDefault400(b *testing.B) {
	r, _ := NewRenderer(DefaultLayout().ScaledTo(400, 400))
	p := metrics.NewMock()
	for i := 0; i < b.N; i++ {
		r.Sample(p)
		r.Render(p)
	}
}
