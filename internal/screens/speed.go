package screens

import (
	"slices"
	"strconv"
)

// Speeds are the speeds spoken word can play at, as the quick menu offers
// them.
var Speeds = []float64{0.75, 0.9, 1, 1.1, 1.25, 1.5, 1.75, 2}

// SpeedLabel names speed x: "1×", "1.25×".
func SpeedLabel(x float64) string { return strconv.FormatFloat(x, 'f', -1, 64) + "×" }

// NextSpeed is the speed step places on from x, stopping at the ends. A
// speed not offered goes to the nearest one offered.
func NextSpeed(x float64, step int) float64 {
	i := slices.Index(Speeds, x)
	if i < 0 {
		i = slices.IndexFunc(Speeds, func(s float64) bool { return s >= x })
		if i < 0 {
			i = len(Speeds) - 1
		}
		return Speeds[i]
	}
	return Speeds[min(max(i+step, 0), len(Speeds)-1)]
}
