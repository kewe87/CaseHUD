package lcd

import (
	"math"
	"testing"
)

func TestSweepAngle(t *testing.T) {
	if got := sweepAngle(1, 0, 0); got != 0 {
		t.Errorf("east from 0: %v", got)
	}
	if got := sweepAngle(0, 1, 0); math.Abs(got-90) > 1e-9 {
		t.Errorf("south from 0: %v", got)
	}
	if got := sweepAngle(1, 0, 90); math.Abs(got-270) > 1e-9 {
		t.Errorf("east from 90: %v", got)
	}
	// Extreme or invalid start angles must neither hang nor leave [0, 360).
	for _, s := range []float64{math.Inf(1), math.Inf(-1), math.NaN(), 1e18, -1e18, 1e-20, -1e-20, 720, -720} {
		got := sweepAngle(1, 1, s)
		if math.IsNaN(got) || got < 0 || got >= 360 {
			t.Errorf("start %v: got %v", s, got)
		}
	}
	if got := sweepAngle(math.NaN(), 1, 0); got != 0 {
		t.Errorf("NaN direction: %v", got)
	}
}
