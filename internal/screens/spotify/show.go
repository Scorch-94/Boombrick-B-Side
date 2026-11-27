package spotify

import (
	"slices"
	"time"

	"github.com/ujjwal313/boombrick/internal/platform/input"
	"github.com/ujjwal313/boombrick/internal/screens"
	"github.com/ujjwal313/boombrick/internal/spotify"
	"github.com/ujjwal313/boombrick/internal/theme"
	"github.com/ujjwal313/boombrick/internal/ui"
	"github.com/ujjwal313/boombrick/internal/ui/gfx"
	"github.com/ujjwal313/boombrick/internal/ui/text"
	"github.com/ujjwal313/boombrick/internal/ui/views"
	"github.com/ujjwal313/boombrick/internal/ui/widgets"
)

var (
	showEpisodeHints = []widgets.Hint{{Key: "A", Label: "Play"}, {Key: "Y", Label: "Add to queue"}, backHint, tabsHint}
	showWaitHints    = []widgets.Hint{backHint, tabsHint}
	showRetryHints   = []widgets.Hint{{Key: "X", Label: "Retry"}, backHint, tabsHint}
)

// ShowPage is a podcast: its art, name and publisher on the left, its
// episodes on the right, newest first, the first focused.
type ShowPage struct {
	e     *Env
	show  spotify.Show
	art   widgets.ArtSpec
	eps   []spotify.Episode
	rows  []widgets.Row
	list  views.Grid
	count string // the episodes, once known
	have  bool
	err   error
	msg   widgets.Wrapped
	ver   uint64
	asked time.Time
}

// NewShowPage opens show s.
func NewShowPage(e *Env, s spotify.Show) *ShowPage {
	pg := &ShowPage{e: e, show: s, art: art(s.Name, s.Cover)}
	pg.list = views.Grid{Cols: 1, CellW: theme.ScreenW - screens.Side - screens.PageListX, CellH: widgets.RowHeight,
		Pad: screens.Pad, ViewH: screens.ContentBot - screens.ContentTop}
	return pg
}

func (pg *ShowPage) refresh(now time.Time) {
	if v := pg.e.Src.Version(); v == pg.ver && now.Sub(pg.asked) < time.Second {
		return
	}
	pg.ver, pg.asked = pg.e.Src.Version(), now
	es, have, err := pg.e.Src.Episodes(pg.show.URI)
	pg.have, pg.err = have, err
	if slices.Equal(es, pg.eps) && len(pg.rows) == len(es) {
		return
	}
	pg.eps = es
	pg.rows = make([]widgets.Row, len(es))
	for i, ep := range es {
		pg.rows[i] = episodeRow(ep, false)
	}
	pg.count = ""
	if have {
		pg.count = screens.Count(len(es)) + " episodes"
		if len(es) >= spotify.MaxShowEpisodes {
			pg.count = "Latest " + pg.count // the show has more
		}
		if len(es) == 1 {
			pg.count = "1 episode"
		}
	}
	pg.list.SetCountFocus(len(es), min(pg.list.Focus(), max(len(es)-1, 0)))
}

// Handle plays the focused episode from where the user got to with A, and
// queues it with Y.
func (pg *ShowPage) Handle(ev input.Event, now time.Time) bool {
	if ev.Kind == input.Release {
		return false
	}
	if ev.Kind == input.Press {
		switch ev.Button {
		case input.A:
			if len(pg.eps) == 0 {
				return false
			}
			PlayEpisode(pg.e, pg.eps[pg.list.Focus()])
			return true
		case input.Y:
			if len(pg.eps) == 0 {
				return false
			}
			QueueEpisode(pg.e, pg.eps[pg.list.Focus()])
			return true
		case input.X:
			if pg.err == nil {
				return false
			}
			pg.e.Src.Retry()
			pg.asked = time.Time{}
			return true
		}
	}
	return pg.list.Move(ev.Button, now)
}

// Animating reports whether the list is moving.
func (pg *ShowPage) Animating(now time.Time) bool { return pg.list.Animating(now) }

// Draw renders the frame.
func (pg *ShowPage) Draw(u *ui.Context) {
	pg.refresh(u.Now)
	widgets.DrawBackground(u, widgets.TabGround)
	side, w := float32(screens.Side), float32(screens.PageArt)
	widgets.Art(u, pg.art, screens.PageArt, widgets.CardRadius, gfx.Rect{X: side, Y: screens.PageTop, W: w, H: w})
	y := float32(screens.PageTop + screens.PageArt + screens.Pad)
	u.Text(u.Fit(pg.show.Name, text.Heading, w), text.Heading, side, y, theme.Paper)
	y += u.LineHeight(text.Heading)
	u.Text(u.Fit(pg.show.Publisher, text.Meta, w), text.Meta, side, y, theme.Ink2)
	y += u.LineHeight(text.Meta)
	u.Text(pg.count, text.Meta, side, y, theme.Ink3)

	hints := showEpisodeHints
	if len(pg.rows) == 0 {
		s := "Loading episodes…"
		hints = showWaitHints
		switch {
		case pg.err != nil && !pg.have:
			s, hints = podcastFailText(pg.err), showRetryHints
		case pg.have:
			s = "No episodes."
		}
		screens.Message(u, &pg.msg, s, gfx.Rect{X: screens.PageListX, Y: screens.ContentTop, W: pg.list.CellW,
			H: screens.ContentBot - screens.ContentTop})
	} else {
		playing := pg.e.Player.State().Track.SpotifyID
		u.PushClip(screens.GlowArea)
		pg.list.Draw(u.Now, screens.PageListX, screens.ContentTop, func(i int, x, y, k float32) {
			r := pg.rows[i]
			r.Playing = spotify.EpisodeStreamID(pg.eps[i].ID) == playing
			box := gfx.Rect{X: x, Y: y, W: pg.list.CellW, H: widgets.RowHeight}
			widgets.RowGlow(u, box, widgets.RowRadius, k)
			u.PushClip(screens.Below(screens.ContentTop))
			widgets.ListRow(u, r, box, k)
			u.PopClip()
		})
		u.PopClip()
	}
	widgets.StatusBar(u)
	widgets.Tabs(u, podcastsTab)
	widgets.HintBar(u, hints)
}
