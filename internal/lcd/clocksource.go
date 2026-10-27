package lcd

import (
	"strings"
	"time"

	"github.com/kewe87/casehud/internal/metrics"
)

// clockSource adds the time of day as readings, so a gauge can show the
// seconds or the hour hand: "Clock/Seconds" (0-60), "Clock/Minutes" (0-60)
// and "Clock/Hours" (0-12).
type clockSource struct {
	metrics.Provider
	now func() time.Time
}

func (c clockSource) Get(source string) (metrics.Reading, bool) {
	if !strings.HasPrefix(source, "Clock/") {
		return c.Provider.Get(source)
	}
	t := time.Now()
	if c.now != nil {
		t = c.now()
	}
	var v float64
	switch source {
	case "Clock/Seconds":
		v = float64(t.Second())
	case "Clock/Minutes":
		v = float64(t.Minute()) + float64(t.Second())/60
	case "Clock/Hours":
		v = float64(t.Hour()%12) + float64(t.Minute())/60
	default:
		return c.Provider.Get(source)
	}
	return metrics.Reading{Key: source, Path: source, Value: v, Unit: "none"}, true
}
