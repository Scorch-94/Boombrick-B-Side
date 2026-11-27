// Package stretch changes the speed of stereo audio without changing its
// pitch, for spoken word played faster or slower. It uses WSOLA: windows of
// the input are overlapped and added at a steady output hop, each taken from
// near where the speed says it should start, nudged to where it best lines
// up with what was played before it, so the voice stays smooth.
package stretch

import "math"

// Sizes in frames, for 48 kHz audio: 21 ms windows, overlapping by half,
// each placed within 5 ms of its nominal start.
const (
	window = 1024
	hop    = window / 2
	search = 256
	// decimate is the step through the samples the search compares, and
	// through the offsets it tries: a quarter of the work, and as good for a
	// voice.
	decimate = 2
)

// Stretcher plays stereo audio at a speed. Its buffers grow to fit and are
// then reused; Process allocates nothing once warm.
type Stretcher struct {
	in    []float32 // interleaved input not yet used up
	base  int64     // the input frame in[0] is
	next  float64   // where the next window nominally starts, in input frames
	prev  int64     // where the last window started, -1 before the first
	ola   []float32 // the overlap: the last window's second half, to add the next to
	out   []float32
	win   [window]float32
	ready bool
}

// New makes a Stretcher.
func New() *Stretcher {
	s := &Stretcher{prev: -1}
	for i := range s.win { // a periodic Hann window: halves overlapped sum to one
		s.win[i] = float32(0.5 - 0.5*math.Cos(2*math.Pi*float64(i)/window))
	}
	s.ola = make([]float32, 2*hop)
	return s
}

// Reset drops what's held, as after a seek or a change of speed.
func (s *Stretcher) Reset() {
	s.in, s.base, s.next, s.prev, s.ready = s.in[:0], 0, 0, -1, false
	clear(s.ola)
}

// Process takes interleaved stereo frames and returns them played at speed,
// as interleaved stereo; the slice is reused by the next call. It holds back
// a window or so of input, which comes out with what follows.
func (s *Stretcher) Process(in []float32, speed float64) []float32 {
	s.in = append(s.in, in...)
	s.out = s.out[:0]
	step := hop * speed
	for {
		nominal := int64(s.next)
		// The window may start up to search frames either side of nominal,
		// and the first half of the window after prev is compared with it.
		need := nominal + search + window
		if s.prev >= 0 {
			need = max(need, s.prev+hop+hop)
		}
		if need-s.base > int64(len(s.in)/2) {
			break
		}
		at := nominal
		if s.prev >= 0 {
			at = s.best(nominal)
		}
		s.add(at)
		s.prev = at
		s.next += step
		// Keep only what a later window can still reach.
		keep := min(int64(s.next)-search, s.prev+hop)
		if drop := keep - s.base; drop > 0 {
			n := copy(s.in, s.in[2*drop:])
			s.in = s.in[:n]
			s.base = keep
		}
	}
	return s.out
}

// best is the start near nominal whose window lines up best with the
// natural continuation of the last window: the input just after it.
func (s *Stretcher) best(nominal int64) int64 {
	ref := s.frame(s.prev + hop)
	lo := max(nominal-search, s.base)
	bestAt, bestScore := nominal, math.Inf(-1)
	for at := lo; at <= nominal+search; at += decimate {
		c := s.frame(at)
		var dot, energy float64
		for i := 0; i < hop; i += decimate {
			a := ref[2*i] + ref[2*i+1]
			b := c[2*i] + c[2*i+1]
			dot += float64(a * b)
			energy += float64(b * b)
		}
		if energy <= 0 {
			continue
		}
		if score := dot / math.Sqrt(energy); score > bestScore {
			bestAt, bestScore = at, score
		}
	}
	return bestAt
}

// frame is the input from frame at on.
func (s *Stretcher) frame(at int64) []float32 { return s.in[2*(at-s.base):] }

// add overlaps the window at at onto what's held, puts out the first hop of
// frames, now complete, and holds the second half for the next.
func (s *Stretcher) add(at int64) {
	w := s.frame(at)
	start := len(s.out)
	s.out = append(s.out, s.ola...)
	o := s.out[start:]
	for i := range hop {
		g := s.win[i]
		o[2*i] += g * w[2*i]
		o[2*i+1] += g * w[2*i+1]
	}
	if !s.ready { // the first window has nothing before it to overlap
		for i := range hop {
			o[2*i], o[2*i+1] = w[2*i], w[2*i+1]
		}
		s.ready = true
	}
	for i := range hop {
		g := s.win[hop+i]
		s.ola[2*i] = g * w[2*(hop+i)]
		s.ola[2*i+1] = g * w[2*(hop+i)+1]
	}
}
