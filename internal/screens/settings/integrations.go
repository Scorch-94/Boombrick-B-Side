package settings

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ujjwal313/boombrick/internal/platform/input"
	"github.com/ujjwal313/boombrick/internal/screens"
	keys "github.com/ujjwal313/boombrick/internal/settings"
	"github.com/ujjwal313/boombrick/internal/theme"
	"github.com/ujjwal313/boombrick/internal/ui"
	"github.com/ujjwal313/boombrick/internal/ui/anim"
	"github.com/ujjwal313/boombrick/internal/ui/gfx"
	"github.com/ujjwal313/boombrick/internal/ui/widgets"
)

// Integration is a switch on the Integrations page: the settings key that
// holds it, and the row that shows it.
type Integration struct {
	Key, Name, Icon, About string
	// Attached is a switch that belongs to the row above, as Podcast
	// episodes in Continue listening: on the Home sections page it moves
	// with that row and can't be moved by itself.
	Attached bool
}

// integrationsPage is a page of switches, Integrations or Home sections: a
// row for each, its name over a line about it, and A switching it on or off.
type integrationsPage struct {
	title    string
	items    []Integration
	switched func(key string, on bool) // after Prefs has it; may be nil
	// order, when set, lets X pick a row up to move it, and keeps the
	// order of the rows not attached under this settings key.
	order  string
	moving bool
	before []Integration // the order before the move, for B
	from   int           // the focus before the move
	focus  int
	scroll anim.Value // how far the rows are scrolled, to keep focus in view
	k      []anim.Value
	rows   []widgets.Setting
	hints  []widgets.Hint
}

func newIntegrationsPage(s *Screen) *integrationsPage {
	return newSwitchPage(s, "Integrations", s.opts.Integrations, s.opts.Switched)
}

// newHomePage is the Home sections page, in the order Home shows them.
func newHomePage(s *Screen) *integrationsPage {
	p := newSwitchPage(s, "Home sections", ordered(s.opts.HomeSections, s.opts.Prefs.String(keys.HomeOrder)),
		func(string, bool) {
			if s.opts.HomeSwitched != nil {
				s.opts.HomeSwitched()
			}
		})
	p.order = keys.HomeOrder
	return p
}

// ordered is items in the order saved, each attached switch kept under the
// row it belongs to.
func ordered(items []Integration, saved string) []Integration {
	order := keys.Order(saved)
	out := make([]Integration, 0, len(items))
	seen := map[int]bool{}
	for _, k := range order {
		for i, in := range items {
			if in.Key != k || in.Attached {
				continue
			}
			for j := i; j < len(items) && (j == i || items[j].Attached); j++ {
				out, seen[j] = append(out, items[j]), true
			}
		}
	}
	for i, in := range items { // any the order doesn't know, last
		if !seen[i] {
			out = append(out, in)
		}
	}
	return out
}

func newSwitchPage(s *Screen, title string, items []Integration, switched func(string, bool)) *integrationsPage {
	p := &integrationsPage{title: title, items: items, switched: switched, k: make([]anim.Value, len(items))}
	p.k[0].Set(1)
	p.read(s)
	return p
}

// read writes the rows from the switches. A row being moved says so.
func (p *integrationsPage) read(s *Screen) {
	p.rows = p.rows[:0]
	start, end := p.group(p.focus)
	for i, in := range p.items {
		r := widgets.Setting{Icon: in.Icon, Label: in.Name, Detail: in.About, Value: onOff(s.on(in.Key))}
		if p.moving && i >= start && i < end {
			r.Value, r.Active = "Moving", i == start
		}
		p.rows = append(p.rows, r)
	}
	switch {
	case p.moving:
		p.hints = moveHints
	case p.order != "" && !p.items[p.focus].Attached:
		p.hints = orderHints
	default:
		p.hints = switchHints
	}
}

var (
	switchHints = []widgets.Hint{{Key: "A", Label: "Switch"}, {Key: "B", Label: "Back"}}
	orderHints  = []widgets.Hint{{Key: "A", Label: "Switch"}, {Key: "X", Label: "Move"}, {Key: "B", Label: "Back"}}
	moveHints   = []widgets.Hint{{Key: "A", Label: "Drop here"}, {Key: "▲ ▼", Label: "Move"}, {Key: "B", Label: "Cancel"}}
)

// group is the rows that move together with row i: from the row not
// attached at or above it, through those attached under that one.
func (p *integrationsPage) group(i int) (start, end int) {
	start = i
	for start > 0 && p.items[start].Attached {
		start--
	}
	end = start + 1
	for end < len(p.items) && p.items[end].Attached {
		end++
	}
	return start, end
}

// shift moves the focused row's group past the group above it, or below.
func (p *integrationsPage) shift(s *Screen, down bool, now time.Time) bool {
	start, end := p.group(p.focus)
	var items []Integration
	focus := 0
	if down {
		if end >= len(p.items) {
			return false
		}
		_, nend := p.group(end)
		items = append(append(append(append([]Integration{}, p.items[:start]...), p.items[end:nend]...), p.items[start:end]...), p.items[nend:]...)
		focus = start + nend - end
	} else {
		if start == 0 {
			return false
		}
		pstart, _ := p.group(start - 1)
		items = append(append(append(append([]Integration{}, p.items[:pstart]...), p.items[start:end]...), p.items[pstart:start]...), p.items[end:]...)
		focus = pstart
	}
	p.items = items
	p.refocus(focus, now)
	p.read(s)
	return true
}

// refocus moves focus to row i at once, as a moved row goes with it.
func (p *integrationsPage) refocus(i int, now time.Time) {
	for j := range p.k {
		p.k[j].Set(0)
	}
	p.focus = i
	p.k[i].Set(1)
}

// drop puts the row being moved down where it is, keeping the order.
func (p *integrationsPage) drop(s *Screen) {
	p.moving = false
	var order []string
	for _, in := range p.items {
		if !in.Attached {
			order = append(order, in.Key)
		}
	}
	s.opts.Prefs.Set(p.order, strings.Join(order, ","))
	p.read(s)
	if p.switched != nil {
		p.switched(p.order, true)
	}
}

// cancel puts the row being moved back where it was.
func (p *integrationsPage) cancel(s *Screen, now time.Time) {
	p.moving, p.items = false, p.before
	p.refocus(p.from, now)
	p.read(s)
}

// handle moves between the rows and switches the focused one with A.
func (p *integrationsPage) handle(s *Screen, ev input.Event, now time.Time) bool {
	if ev.Kind == input.Release {
		return false
	}
	if p.moving {
		switch ev.Button {
		case input.Up, input.Down:
			return p.shift(s, ev.Button == input.Down, now)
		case input.A, input.X:
			if ev.Kind == input.Press {
				p.drop(s)
				return true
			}
		}
		return false
	}
	switch ev.Button {
	case input.X:
		if ev.Kind != input.Press || p.order == "" || p.items[p.focus].Attached {
			return false
		}
		p.moving, p.before, p.from = true, slices.Clone(p.items), p.focus
		p.read(s)
		return true
	case input.Up, input.Down:
		next := p.focus + 1
		if ev.Button == input.Up {
			next = p.focus - 1
		}
		if next < 0 || next >= len(p.rows) {
			return false
		}
		p.k[p.focus].To(0, now, anim.Focus, anim.EaseOutCubic)
		p.focus = next
		p.k[next].To(1, now, anim.Focus, anim.EaseOutCubic)
		p.read(s) // the hints: X moves only a row not attached
		return true
	case input.A:
		if ev.Kind != input.Press {
			return false
		}
		in := p.items[p.focus]
		on := !s.on(in.Key)
		s.opts.Prefs.SetBool(in.Key, on)
		s.readPrefs() // first: the rows read the switches it keeps
		p.read(s)
		if p.switched != nil {
			p.switched(in.Key, on)
		}
		return true
	}
	return false
}

func (p *integrationsPage) animating(now time.Time) bool {
	if !p.scroll.Done(now) {
		return true
	}
	for i := range p.k {
		if !p.k[i].Done(now) {
			return true
		}
	}
	return false
}

func (p *integrationsPage) draw(s *Screen, u *ui.Context) {
	widgets.StatusBar(u)
	header(u, p.title)
	top := listTop(u)
	view := gfx.Rect{X: 0, Y: top, W: theme.ScreenW, H: screens.ContentBot - top}
	// Scroll as little as keeps the focused row wholly in view.
	h := float64(widgets.SettingDetailHeight)
	to := max(p.scroll.Target(), float64(p.focus+1)*h-float64(view.H))
	to = min(to, float64(p.focus)*h)
	p.scroll.To(to, u.Now, anim.Focus, anim.EaseOutCubic)
	y := top - float32(p.scroll.At(u.Now))
	u.PushClip(view)
	for i, r := range p.rows {
		box := gfx.Rect{X: side - rowInset, Y: y, W: theme.ScreenW - 2*(side-rowInset), H: widgets.SettingDetailHeight}
		if box.Y+box.H > view.Y && box.Y < view.Y+view.H {
			widgets.DrawSetting(u, r, box, float32(p.k[i].At(u.Now)), i == len(p.rows)-1)
		}
		y += box.H
	}
	u.PopClip()
	widgets.HintBar(u, p.hints)
}

// on reports whether the integration whose switch is key is on, as the
// switches were last read.
func (s *Screen) on(key string) bool {
	if v, ok := s.switches[key]; ok {
		return v
	}
	return s.opts.Prefs.Bool(key)
}

// switchedOn is a switch page's row's value: "2 of 2 on".
func (s *Screen) switchedOn(items []Integration) string {
	n := 0
	for _, in := range items {
		if s.on(in.Key) {
			n++
		}
	}
	return strconv.Itoa(n) + " of " + strconv.Itoa(len(items)) + " on"
}
