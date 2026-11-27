package ui

import (
	"image/color"
	"time"

	"github.com/ujjwal313/boombrick/internal/ui/gfx"
	"github.com/ujjwal313/boombrick/internal/ui/text"
)

// How the focused item's text too long for its room scrolls: it waits
// ScrollDelay with its end cut off, then slides left at scrollSpeed until a
// copy scrollGap behind it comes round, rests scrollRest, and slides again.
const (
	ScrollDelay = 500 * time.Millisecond
	scrollRest  = 2500 * time.Millisecond
	scrollSpeed = 40.0 // design pixels a second
	scrollGap   = 56
)

// scrollKey is a line scrolling: its text and style.
type scrollKey struct {
	s  string
	st text.Style
}

// scrolling is when a line began to show focused, the last frame it did,
// and its whole width, measured once.
type scrolling struct {
	since time.Time
	frame uint64
	width float32
}

// ScrollText draws s at x, y as Text does, in room w. Unfocused, or when it
// fits, it is cut to fit with "…". Focused and too long, it scrolls: from
// ScrollDelay after it first showed focused, sliding left through the room,
// resting at its start between runs. A line not drawn focused in the frame
// before starts over. The frames it needs are asked for: every frame while
// it slides, and while it waits, one when the wait is over. It returns the
// width drawn, as Text does, at most w. Whether a line fits is told from
// Fit's cached cut, so a frame drawn again shapes nothing.
func (u *Context) ScrollText(s string, st text.Style, x, y, w float32, col color.NRGBA, focused bool) float32 {
	f := u.Fit(s, st, w)
	if !focused || f == s {
		return u.Text(f, st, x, y, col)
	}
	if u.scrolls == nil {
		u.scrolls = map[scrollKey]*scrolling{}
	}
	k := scrollKey{s, st}
	sc := u.scrolls[k]
	if sc == nil || sc.frame+1 < u.frame {
		sc = &scrolling{since: u.Now, width: u.TextWidth(s, st)}
		u.scrolls[k] = sc
	}
	sc.frame = u.frame
	off, moving, next := scrollOffset(sc.width, u.Now.Sub(sc.since))
	if !moving {
		u.WakeAt(sc.since.Add(next))
		return u.Text(f, st, x, y, col)
	}
	tw := sc.width
	u.KeepDrawing()
	lh := u.LineHeight(st)
	u.PushClip(gfx.Rect{X: x, Y: y - lh/4, W: w, H: lh * 1.5})
	u.Text(s, st, x-off, y, col)
	u.Text(s, st, x-off+tw+scrollGap, y, col)
	u.PopClip()
	return w
}

// scrollOffset is how far a line tw wide is slid t after it began to show
// focused, whether it is sliding, and, when it isn't, how long after it
// began the next slide starts.
func scrollOffset(tw float32, t time.Duration) (off float32, moving bool, next time.Duration) {
	if t < ScrollDelay {
		return 0, false, ScrollDelay
	}
	slide := time.Duration(float64(tw+scrollGap) / scrollSpeed * float64(time.Second))
	cycle := slide + scrollRest
	n := (t - ScrollDelay) / cycle
	in := (t - ScrollDelay) % cycle
	if in < slide {
		return float32(in.Seconds() * scrollSpeed), true, 0
	}
	return 0, false, ScrollDelay + (n+1)*cycle
}

// WakeAt asks for a frame at t though nothing moves before it; the
// earliest asked for in a frame is Wake's.
func (u *Context) WakeAt(t time.Time) {
	if u.wakeAt.IsZero() || t.Before(u.wakeAt) {
		u.wakeAt = t
	}
}

// Wake is when the last frame asked to be drawn again, if it did.
func (u *Context) Wake() (time.Time, bool) { return u.wakeAt, !u.wakeAt.IsZero() }
