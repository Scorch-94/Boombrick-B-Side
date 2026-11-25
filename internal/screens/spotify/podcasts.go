package spotify

import (
	"errors"
	"slices"
	"strconv"
	"time"

	"github.com/ujjwal313/boombrick/internal/platform/input"
	"github.com/ujjwal313/boombrick/internal/player"
	"github.com/ujjwal313/boombrick/internal/screens"
	"github.com/ujjwal313/boombrick/internal/spotify"
	"github.com/ujjwal313/boombrick/internal/theme"
	"github.com/ujjwal313/boombrick/internal/ui"
	"github.com/ujjwal313/boombrick/internal/ui/gfx"
	"github.com/ujjwal313/boombrick/internal/ui/views"
	"github.com/ujjwal313/boombrick/internal/ui/widgets"
)

// podcastsTab is the Podcasts tab's ID.
const podcastsTab = widgets.TabPodcasts

// The Podcasts tab's filters, chosen with L2 and R2.
const (
	continueFilter = iota
	showsFilter
	savedFilter
	podcastFilters
)

var podcastFilterNames = []string{"Continue", "Shows", "Your Episodes"}

// The episode lists the filters show, by filter; the shows have none.
var filterURIs = [podcastFilters]string{continueFilter: spotify.ContinueURI, savedFilter: spotify.YourEpisodesURI}

// startOver is how near an episode's end a saved place is taken as having
// finished it: it plays again from the start.
const startOver = 30 * time.Second

// EpisodeTrack is episode e as a queue entry, streamed: its title, its show
// as the artist, its publisher as the album, and its cover.
func EpisodeTrack(e spotify.Episode) player.Track {
	album := e.Publisher
	if album == "" {
		album = e.Show
	}
	return player.Track{SpotifyID: spotify.EpisodeStreamID(e.ID), Title: e.Title, Artist: e.Show, AlbumArtist: e.Publisher,
		Album: album, Duration: time.Duration(e.Duration) * time.Millisecond, Cover: e.Cover}
}

// IsEpisode reports whether queue entry t is a podcast episode.
func IsEpisode(t player.Track) bool {
	_, ok := spotify.IsEpisode(t.SpotifyID)
	return ok && t.Path == ""
}

// externalToast says why an episode Spotify doesn't host won't play.
const externalToast = "Hosted outside Spotify, so it can’t play here yet"

// playEpisode plays ep on its own from where the user got to in it, or from
// the start once they finished it or got near its end.
func playEpisode(e *Env, ep spotify.Episode) {
	if ep.External {
		e.Nav.Toast(externalToast)
		return
	}
	ms := ep.Position
	if ep.Finished || ms >= ep.Duration-startOver.Milliseconds() {
		ms = 0
	}
	e.Player.SetShuffle(false)
	e.Player.PlayFrom([]player.Track{EpisodeTrack(ep)}, 0, ms*spotify.Rate/1000)
}

// queueEpisode adds ep to the end of the queue.
func queueEpisode(e *Env, ep spotify.Episode) {
	if ep.External {
		e.Nav.Toast(externalToast)
		return
	}
	e.Player.Enqueue([]player.Track{EpisodeTrack(ep)})
	e.Nav.Toast(screens.AddedToQueue)
}

// episodeRow is ep's row: its title over first what the row is listed
// under says it should start with (its show, or its date), then how much
// is left of it.
func episodeRow(ep spotify.Episode, withShow bool) widgets.Row {
	var lead string
	if withShow {
		lead = ep.Show
	} else if ep.Published > 0 {
		lead = dateLine(time.Unix(ep.Published, 0).UTC())
	}
	state := minutes(ep.Duration)
	switch {
	case ep.External:
		state = "Not playable yet"
	case ep.Finished:
		state = "Played"
	case ep.Position > 0:
		state = minutes(ep.Duration-ep.Position) + " left"
	}
	sub := state
	if lead != "" {
		sub = lead + " · " + state
	}
	return widgets.Row{Title: ep.Title, Subtitle: sub}
}

// dateLine is a date as an episode row gives it: "Nov 20", with the year
// when it isn't this one.
func dateLine(d time.Time) string {
	if d.Year() != time.Now().Year() {
		return d.Format("Jan 2, 2006")
	}
	return d.Format("Jan 2")
}

// minutes is a length of ms in whole minutes, at least one: "45 min",
// "1 hr 5 min".
func minutes(ms int64) string {
	m := max(int((ms+30_000)/60_000), 1)
	if m < 60 {
		return strconv.Itoa(m) + " min"
	}
	h, m := m/60, m%60
	if m == 0 {
		return strconv.Itoa(h) + " hr"
	}
	return strconv.Itoa(h) + " hr " + strconv.Itoa(m) + " min"
}

// The Podcasts tab's layout: the filters under the tabs, the list below.
const podcastListTop = screens.ContentTop + widgets.FiltersHeight

// The Podcasts tab's hints.
var (
	episodeHints    = []widgets.Hint{{Key: "A", Label: "Play"}, {Key: "Y", Label: "Add to queue"}, {Key: "L2/R2", Label: "Filter"}, tabsHint}
	showHints       = []widgets.Hint{{Key: "A", Label: "Open"}, {Key: "L2/R2", Label: "Filter"}, tabsHint}
	emptyListHints  = []widgets.Hint{{Key: "L2/R2", Label: "Filter"}, tabsHint}
	retryPodHints   = []widgets.Hint{{Key: "X", Label: "Retry"}, {Key: "L2/R2", Label: "Filter"}, tabsHint}
	podSignInHints  = []widgets.Hint{{Key: "A", Label: "Sign in"}, tabsHint}
	podFailedHints  = []widgets.Hint{retryHint, tabsHint}
	podLoadingHints = []widgets.Hint{tabsHint}
)

// Podcasts is the Podcasts tab: the episodes the user is part way through,
// the shows they follow and the episodes they saved, from their Spotify
// account. Signed out, it leads to the sign-in.
type Podcasts struct {
	e      *Env
	filter int
	lists  [podcastFilters]views.Grid
	eps    [podcastFilters][]spotify.Episode // the Continue and Your Episodes lists
	shows  []spotify.Show
	rows   [podcastFilters][]widgets.Row
	arts   [podcastFilters][]widgets.ArtSpec
	have   [podcastFilters]bool
	errs   [podcastFilters]error
	msg    widgets.Wrapped

	version   uint64
	asked     time.Time
	st        spotify.Status
	err       error
	signedOut bool
	failed    bool // the status can't be had
}

// NewPodcasts makes the tab.
func NewPodcasts(e *Env) *Podcasts {
	p := &Podcasts{e: e}
	for i := range p.lists {
		p.lists[i] = views.Grid{Cols: 1, CellW: theme.ScreenW - 2*screens.Side, CellH: widgets.RowHeight, Pad: screens.Pad,
			ViewH: screens.ContentBot - podcastListTop}
	}
	return p
}

// refresh asks the library again when it has changed, or a second on.
func (p *Podcasts) refresh(now time.Time) {
	if v := p.e.Src.Version(); v == p.version && now.Sub(p.asked) < time.Second {
		return
	}
	p.version, p.asked = p.e.Src.Version(), now
	st, err := p.e.Src.Status()
	p.st, p.err = st, err
	p.signedOut = err == nil && (st.State == spotify.SignedOut || st.State == spotify.SigningIn)
	p.failed = err != nil && st.State == ""
	if p.signedOut || p.failed || st.State == "" {
		return
	}
	// Only the filter shown is asked for, so opening the tab doesn't ask for
	// all three at once.
	switch p.filter {
	case showsFilter:
		ss, have, err := p.e.Src.Shows()
		p.have[showsFilter], p.errs[showsFilter] = have, err
		if !slices.Equal(ss, p.shows) || len(p.rows[showsFilter]) != len(ss) {
			p.setShows(ss)
		}
	default:
		es, have, err := p.e.Src.Episodes(filterURIs[p.filter])
		p.have[p.filter], p.errs[p.filter] = have, err
		if !slices.Equal(es, p.eps[p.filter]) || len(p.rows[p.filter]) != len(es) {
			p.setEpisodes(p.filter, es)
		}
	}
}

func (p *Podcasts) setShows(ss []spotify.Show) {
	p.shows = ss
	p.arts[showsFilter] = make([]widgets.ArtSpec, len(ss))
	p.rows[showsFilter] = make([]widgets.Row, len(ss))
	for i, s := range ss {
		p.arts[showsFilter][i] = art(s.Name, s.Cover)
		p.rows[showsFilter][i] = widgets.Row{Art: &p.arts[showsFilter][i], Title: s.Name, Subtitle: s.Publisher}
	}
	g := &p.lists[showsFilter]
	g.SetCountFocus(len(ss), min(g.Focus(), max(len(ss)-1, 0)))
}

func (p *Podcasts) setEpisodes(f int, es []spotify.Episode) {
	p.eps[f] = es
	p.arts[f] = make([]widgets.ArtSpec, len(es))
	p.rows[f] = make([]widgets.Row, len(es))
	for i, ep := range es {
		p.arts[f][i] = art(ep.Show, ep.Cover)
		p.rows[f][i] = episodeRow(ep, true)
		p.rows[f][i].Art = &p.arts[f][i]
	}
	g := &p.lists[f]
	g.SetCountFocus(len(es), min(g.Focus(), max(len(es)-1, 0)))
}

// Handle plays an episode with A or opens a show, queues an episode with
// Y, and changes filter with L2 and R2. Signed out, A leads to the sign-in.
func (p *Podcasts) Handle(ev input.Event, now time.Time) bool {
	if ev.Kind == input.Release {
		return false
	}
	press := ev.Kind == input.Press
	switch {
	case p.failed:
		if press && ev.Button == input.A {
			p.retry()
			return true
		}
		return false
	case p.signedOut:
		if press && ev.Button == input.A {
			p.e.Nav.Push(NewSignIn(p.e))
			return true
		}
		return false
	}
	g := &p.lists[p.filter]
	if press {
		switch ev.Button {
		case input.L2, input.R2:
			f := p.filter - 1
			if ev.Button == input.R2 {
				f = p.filter + 1
			}
			if f < 0 || f >= podcastFilters {
				return false
			}
			p.filter = f
			p.asked = time.Time{} // ask for the new filter's list at once
			return true
		case input.A:
			if g.Count() == 0 {
				return false
			}
			if p.filter == showsFilter {
				p.e.Nav.Push(NewShowPage(p.e, p.shows[g.Focus()]))
			} else {
				playEpisode(p.e, p.eps[p.filter][g.Focus()])
			}
			return true
		case input.Y:
			if p.filter == showsFilter || g.Count() == 0 {
				return false
			}
			queueEpisode(p.e, p.eps[p.filter][g.Focus()])
			return true
		case input.X:
			if p.errs[p.filter] == nil {
				return false
			}
			p.retry()
			return true
		}
	}
	return g.Move(ev.Button, now)
}

func (p *Podcasts) retry() {
	p.e.Src.Retry()
	p.asked = time.Time{}
}

// Animating reports whether the list is moving.
func (p *Podcasts) Animating(now time.Time) bool { return p.lists[p.filter].Animating(now) }

// Draw renders the frame.
func (p *Podcasts) Draw(u *ui.Context) {
	p.refresh(u.Now)
	widgets.DrawBackground(u, widgets.TabGround)
	content := gfx.Rect{X: screens.Side, Y: screens.ContentTop, W: theme.ScreenW - 2*screens.Side,
		H: screens.ContentBot - screens.ContentTop}
	var hints []widgets.Hint
	switch {
	case p.failed:
		screens.Message(u, &p.msg, failText(p.err), content)
		hints = podFailedHints
	case p.signedOut:
		screens.Message(u, &p.msg, "Sign in to Spotify to see the podcasts you follow and pick up where you left off.", content)
		hints = podSignInHints
	case p.st.State == "":
		screens.Message(u, &p.msg, "Loading…", content)
		hints = podLoadingHints
	default:
		hints = p.drawLists(u)
	}
	widgets.StatusBar(u)
	widgets.Tabs(u, podcastsTab)
	widgets.HintBar(u, hints)
}

// drawLists draws the filters and the list chosen, or what it says while
// it has nothing, and returns the hints for it.
func (p *Podcasts) drawLists(u *ui.Context) []widgets.Hint {
	widgets.Filters(u, podcastFilterNames, nil, p.filter, screens.Side, screens.ContentTop)
	g := &p.lists[p.filter]
	rows := p.rows[p.filter]
	if len(rows) == 0 {
		box := gfx.Rect{X: screens.Side, Y: podcastListTop, W: g.CellW, H: screens.ContentBot - podcastListTop}
		s, hints := p.emptyText(), emptyListHints
		if err := p.errs[p.filter]; err != nil && !p.have[p.filter] {
			s, hints = podcastFailText(err), retryPodHints
		}
		screens.Message(u, &p.msg, s, box)
		return hints
	}
	playing := p.e.Player.State().Track.SpotifyID
	u.PushClip(screens.GlowArea)
	g.Draw(u.Now, screens.Side, podcastListTop, func(i int, x, y, k float32) {
		box := gfx.Rect{X: x, Y: y, W: g.CellW, H: widgets.RowHeight}
		r := rows[i]
		if p.filter != showsFilter {
			r.Playing = spotify.EpisodeStreamID(p.eps[p.filter][i].ID) == playing
		}
		widgets.RowGlow(u, box, widgets.RowRadius, k)
		u.PushClip(screens.Below(podcastListTop))
		widgets.ListRow(u, r, box, k)
		u.PopClip()
	})
	u.PopClip()
	if p.filter == showsFilter {
		return showHints
	}
	return episodeHints
}

// emptyText says why the filter shown has nothing.
func (p *Podcasts) emptyText() string {
	if !p.have[p.filter] {
		return "Loading…"
	}
	switch p.filter {
	case continueFilter:
		return "Nothing in progress. Episodes you start, here or in Spotify, show up here."
	case showsFilter:
		return "You don’t follow any podcasts yet. Follow some in Spotify and they’ll show up here."
	}
	return "No saved episodes. Episodes you save in Spotify show up here."
}

// podcastFailText says why a podcast list couldn't be had.
func podcastFailText(err error) string {
	if errors.Is(err, spotify.ErrOffline) || errors.Is(err, spotify.ErrNoHelper) || errors.Is(err, spotify.ErrSignedOut) {
		return failText(err)
	}
	return "Couldn’t load your podcasts from Spotify. Press X to try again."
}
