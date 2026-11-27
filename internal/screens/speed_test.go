package screens

import "testing"

func TestNextSpeed(t *testing.T) {
	for _, c := range []struct {
		x    float64
		step int
		want float64
	}{{1, 1, 1.1}, {1, -1, 0.9}, {2, 1, 2}, {0.75, -1, 0.75}, {1.3, 1, 1.5}, {5, 1, 2}} {
		if got := NextSpeed(c.x, c.step); got != c.want {
			t.Errorf("NextSpeed(%v, %d) = %v, want %v", c.x, c.step, got, c.want)
		}
	}
	if SpeedLabel(1.25) != "1.25×" || SpeedLabel(2) != "2×" {
		t.Error(SpeedLabel(1.25), SpeedLabel(2))
	}
}
