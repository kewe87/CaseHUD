package cooling

import (
	"context"
	"testing"
)

// Speeds measured while starting from standstill are another thing than those
// measured while going down: they never enter the table.
func TestCalibrateTableHasNoRestartValues(t *testing.T) {
	f := &calFan{perDuty: 20, stopBelow: 8, startAt: 14, twitchAt: 10}
	steps, cal, err := newCal(f).Calibrate(context.Background())
	if err != nil || !cal.Starts {
		t.Fatalf("%+v %v", cal, err)
	}
	for i := 1; i < len(steps); i++ {
		if steps[i].Duty >= steps[i-1].Duty {
			t.Fatalf("steps are not the way down: %+v", steps)
		}
	}
	if got := steps[len(steps)-1].Duty; got != 7 { // it stood there
		t.Fatalf("last step %d", got)
	}
	// The restart tried 1..14; none of it is among the steps (they stop at 7 and rise nowhere).
	if len(steps) != 3+13 {
		t.Fatalf("%d steps", len(steps))
	}
}
