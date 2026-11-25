package settings

import (
	"strconv"
	"time"

	"github.com/ujjwal313/boombrick/internal/platform/input"
	"github.com/ujjwal313/boombrick/internal/screens"
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
}

// integrationsPage is a page of switches, Integrations or Home sections: a
// row for each, its name over a line about it, and A switching it on or off.
type integrationsPage struct {
	title    string
	items    []Integration
	switched func(key string, on bool) // after Prefs has it; may be nil
	focus    int
	scroll   anim.Value // how far the rows are scrolled, to keep focus in view
	k        []anim.Value
	rows     []widgets.Setting
	hints    []widgets.Hint
}

func newIntegrationsPage(s *Screen) *integrationsPage {
	return newSwitchPage(s, "Integrations", s.opts.Integrations, s.opts.Switched)
}

// newHomePage is the Home sections page.
func newHomePage(s *Screen) *integrationsPage {
	return newSwitchPage(s, "Home sections", s.opts.HomeSections, func(string, bool) {
		if s.opts.HomeSwitched != nil {
			s.opts.HomeSwitched()
		}
	})
}

func newSwitchPage(s *Screen, title string, items []Integration, switched func(string, bool)) *integrationsPage {
	p := &integrationsPage{title: title, items: items, switched: switched, k: make([]anim.Value, len(items)),
		hints: []widgets.Hint{{Key: "A", Label: "Switch"}, {Key: "B", Label: "Back"}}}
	p.k[0].Set(1)
	p.read(s)
	return p
}

// read writes the rows from the switches.
func (p *integrationsPage) read(s *Screen) {
	p.rows = p.rows[:0]
	for _, in := range p.items {
		p.rows = append(p.rows, widgets.Setting{Icon: in.Icon, Label: in.Name, Detail: in.About, Value: onOff(s.on(in.Key))})
	}
}

// handle moves between the rows and switches the focused one with A.
func (p *integrationsPage) handle(s *Screen, ev input.Event, now time.Time) bool {
	if ev.Kind == input.Release {
		return false
	}
	switch ev.Button {
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
