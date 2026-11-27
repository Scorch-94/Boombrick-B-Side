package stretch

import (
	"math"
	"testing"
)

const rate = 48000

func tone(freq float64, frames int) []float32 {
	out := make([]float32, 2*frames)
	for i := range frames {
		v := float32(0.5 * math.Sin(2*math.Pi*freq*float64(i)/rate))
		out[2*i], out[2*i+1] = v, v
	}
	return out
}

// crossings is how many times the left channel rises through zero per second.
func crossings(pcm []float32) float64 {
	n := 0
	for i := 2; i < len(pcm); i += 2 {
		if pcm[i-2] < 0 && pcm[i] >= 0 {
			n++
		}
	}
	return float64(n) / (float64(len(pcm)/2) / rate)
}

func run(t *testing.T, speed float64, in []float32) []float32 {
	t.Helper()
	s := New()
	var out []float32
	for len(in) > 0 { // in chunks, as the engine feeds it
		n := min(len(in), 2*1024)
		out = append(out, s.Process(in[:n], speed)...)
		in = in[n:]
	}
	return out
}

func TestSpeedKeepsPitch(t *testing.T) {
	in := tone(220, 4*rate)
	for _, speed := range []float64{0.75, 1.25, 1.5, 2} {
		out := run(t, speed, in)
		got := float64(len(out)/2) / rate
		want := 4 / speed
		if math.Abs(got-want) > 0.05 {
			t.Errorf("%v×: %.3f s out, want about %.3f", speed, got, want)
		}
		if f := crossings(out); math.Abs(f-220) > 3 {
			t.Errorf("%v×: the tone is %.1f Hz, want 220", speed, f)
		}
		// Overlapping windows that line up keep the level: no dips or clicks.
		var peak, low float32 = 0, 1
		for i := rate / 10; i < len(out)/2-rate/10; i += 48 {
			var m float32
			for j := i; j < i+240 && j < len(out)/2; j++ {
				m = max(m, float32(math.Abs(float64(out[2*j]))))
			}
			peak, low = max(peak, m), min(low, m)
		}
		if peak > 0.55 || low < 0.4 {
			t.Errorf("%v×: the level runs %.2f to %.2f, want about 0.5", speed, low, peak)
		}
	}
}

func TestResetStartsAfresh(t *testing.T) {
	s := New()
	s.Process(tone(220, rate/2), 1.5)
	s.Reset()
	if out := s.Process(make([]float32, 2*100), 1.5); len(out) != 0 {
		t.Errorf("%d samples out of 100 frames after Reset, want none yet", len(out))
	}
}

func TestProcessDoesNotAllocateOnceWarm(t *testing.T) {
	s := New()
	in := tone(220, 1024)
	for range 50 {
		s.Process(in, 1.5)
	}
	if a := testing.AllocsPerRun(50, func() { s.Process(in, 1.5) }); a != 0 {
		t.Errorf("Process allocates %v times", a)
	}
}

func BenchmarkProcess(b *testing.B) {
	s := New()
	in := tone(220, 1024)
	for b.Loop() {
		s.Process(in, 1.5)
	}
}
