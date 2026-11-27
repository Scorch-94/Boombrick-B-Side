package nowplaying

import (
	"image"
	"testing"
	"time"

	"github.com/ujjwal313/boombrick/internal/fixture"
	"github.com/ujjwal313/boombrick/internal/theme"
	"github.com/ujjwal313/boombrick/internal/ui/uitest"
)

// A long line rests at its start, then slides left at marqueeSpeed until
// its copy has come round, and rests again.
func TestMarqueeOffset(t *testing.T) {
	t0 := time.Date(2026, 11, 26, 23, 0, 0, 0, time.UTC)
	m := marquee{since: t0}
	const w = 200
	if off, moving := m.offset(w, t0.Add(time.Second)); off != 0 || moving {
		t.Fatalf("resting: %v %v", off, moving)
	}
	off, moving := m.offset(w, t0.Add(marqueeRest+time.Second))
	if !moving || off < marqueeSpeed-1 || off > marqueeSpeed+1 {
		t.Fatalf("a second sliding: %v %v", off, moving)
	}
	slide := time.Duration(float64(w+marqueeGap) / marqueeSpeed * float64(time.Second))
	if off, moving := m.offset(w, t0.Add(marqueeRest+slide+time.Second)); off != 0 || moving {
		t.Fatalf("round again, resting: %v %v", off, moving)
	}
}

// A title too long for its room slides through it once it has rested, and
// the screen asks for frames while it does; short lines stay put.
func TestLongLinesScroll(t *testing.T) {
	f := playing()
	f.st.Track.Title = "Sacred Symbols, Episode 200: Ranking Every PlayStation Mascot"
	f.st.Track.Artist = "Sacred Symbols: A PlayStation Podcast"
	n := New(f)
	draw := func(at time.Time) *image.NRGBA {
		u, off := uitest.NewContext(t, theme.DeviceW, theme.DeviceH)
		u.Accent = theme.Derive(fixture.PlayingHue, fixture.PlayingChroma)
		u.Status = fixture.Status()
		u.Now = at
		n.Draw(u)
		return off.Image()
	}
	draw(t0)
	if n.Animating(t0) || n.lines[0].moving {
		t.Fatal("resting at first")
	}
	img := draw(t0.Add(marqueeRest + 3*time.Second))
	if !n.lines[0].moving || !n.lines[1].moving || n.lines[2].moving || !n.Animating(t0) {
		t.Fatalf("sliding: %v %v %v", n.lines[0].moving, n.lines[1].moving, n.lines[2].moving)
	}
	uitest.Golden(t, "nowplaying-scrolling", img)
}
