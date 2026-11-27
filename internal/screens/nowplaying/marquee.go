package nowplaying

import (
	"image/color"
	"time"

	"github.com/ujjwal313/boombrick/internal/ui"
	"github.com/ujjwal313/boombrick/internal/ui/gfx"
	"github.com/ujjwal313/boombrick/internal/ui/text"
)

// How a line too long for its room scrolls: it rests marqueeRest at its
// start, then slides left at marqueeSpeed until a copy of it, marqueeGap
// behind, has come round to the start, and rests again.
const (
	marqueeRest  = 2500 * time.Millisecond
	marqueeSpeed = 40.0 // design pixels a second
	marqueeGap   = 56
)

// marquee is one line's scroll: the text it is for, and when it was first
// drawn, which the scroll counts from.
type marquee struct {
	text  string
	since time.Time
	// moving is whether the last frame drew it sliding, rather than resting
	// or fitting.
	moving bool
}

// offset is how far left the line is slid at now, for a line w wider than
// nothing, a run of rest then slide repeating; moving says whether it slides.
func (m *marquee) offset(w float32, now time.Time) (float32, bool) {
	run := float64(w + marqueeGap)
	slide := time.Duration(run / marqueeSpeed * float64(time.Second))
	t := now.Sub(m.since) % (marqueeRest + slide)
	if t < marqueeRest {
		return 0, false
	}
	return float32((t - marqueeRest).Seconds() * marqueeSpeed), true
}

// draw draws s at x, y in room w: whole when it fits, else sliding through
// the room and clipped to it. The scroll starts again when s changes.
func (m *marquee) draw(u *ui.Context, s string, style text.Style, x, y, w float32, col color.NRGBA) {
	if s != m.text {
		m.text, m.since = s, u.Now
	}
	tw := u.TextWidth(s, style)
	if tw <= w {
		m.moving = false
		u.Text(s, style, x, y, col)
		return
	}
	off, moving := m.offset(tw, u.Now)
	m.moving = moving
	lh := u.LineHeight(style)
	u.PushClip(gfx.Rect{X: x, Y: y - lh/4, W: w, H: lh * 1.5})
	u.Text(s, style, x-off, y, col)
	u.Text(s, style, x-off+tw+marqueeGap, y, col)
	u.PopClip()
}
