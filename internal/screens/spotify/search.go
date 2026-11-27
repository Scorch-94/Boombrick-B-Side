package spotify

import (
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

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

// The search screen, laid out as the Library's: the keyboard on the left,
// the results on the right under their label.
const (
	searchKbX      = screens.Side
	searchKbY      = screens.ContentTop + screens.Pad
	searchResX     = searchKbX + widgets.KeyboardW + 28
	searchResW     = theme.ScreenW - screens.Side - searchResX
	searchListTop  = searchKbY + 30
	searchRowH     = 76
	searchMaxQuery = 80
)

var searchLabel = text.Style{Face: text.Figtree, Size: 20}

// Kinds of result, in the order they list.
const (
	songHit = iota
	albumHit
	artistHit
)

type searchHit struct {
	kind   int
	song   int // the index among the songs found
	album  spotify.Release
	artist spotify.Artist
}

// Search finds songs, albums and artists on Spotify. What's typed is asked
// for when Search (or Y) is pressed, not with every key, as each search
// goes to Spotify. A plays a song from the songs found or opens an album or
// an artist; Y queues a song.
type Search struct {
	e         *Env
	kb        widgets.Keyboard
	query     string // as typed
	asked     string // as last searched
	onResults bool
	res       spotify.Results
	have      bool
	err       error
	version   uint64
	hits      []searchHit
	rows      []widgets.Row
	arts      []widgets.ArtSpec
	list      views.Grid
	msg       widgets.Wrapped
}

// NewSearch opens the search, focus on the keyboard.
func NewSearch(e *Env) *Search {
	s := &Search{e: e, kb: widgets.NewKeyboard()}
	s.list = views.Grid{Cols: 1, CellW: searchResW, CellH: searchRowH, Pad: screens.Pad, ViewH: screens.ContentBot - searchListTop}
	s.list.SetActive(false, time.Time{})
	return s
}

// refresh takes the answer for what was searched, as it comes.
func (s *Search) refresh() {
	if s.asked == "" {
		return
	}
	v := s.e.Src.Version()
	if v == s.version && s.have {
		return
	}
	s.version = v
	res, have, err := s.e.Src.Search(s.asked)
	if have == s.have && err == nil && s.err == nil && sameResults(res, s.res) {
		return
	}
	s.res, s.have, s.err = res, have, err
	s.build()
}

func sameResults(a, b spotify.Results) bool {
	return len(a.Tracks) == len(b.Tracks) && len(a.Albums) == len(b.Albums) && len(a.Artists) == len(b.Artists) &&
		(len(a.Tracks) == 0 || a.Tracks[0].ID == b.Tracks[0].ID) &&
		(len(a.Albums) == 0 || a.Albums[0].URI == b.Albums[0].URI) &&
		(len(a.Artists) == 0 || a.Artists[0].URI == b.Artists[0].URI)
}

// build makes the rows: songs, then albums, then artists.
func (s *Search) build() {
	s.hits, s.rows, s.arts = s.hits[:0], s.rows[:0], s.arts[:0]
	for i, t := range s.res.Tracks {
		s.hits = append(s.hits, searchHit{kind: songHit, song: i})
		r := widgets.Row{Title: t.Title, Subtitle: "Song · " + strings.Join(t.Artists, ", ")}
		if t.Duration > 0 {
			r.Duration = screens.Clock(time.Duration(t.Duration) * time.Millisecond)
		}
		s.rows, s.arts = append(s.rows, r), append(s.arts, art(t.Album, t.Cover))
	}
	for _, a := range s.res.Albums {
		s.hits = append(s.hits, searchHit{kind: albumHit, album: a})
		s.rows = append(s.rows, widgets.Row{Title: a.Name, Subtitle: a.Type + " · " + a.Artist})
		s.arts = append(s.arts, art(a.Name, a.Cover))
	}
	for _, a := range s.res.Artists {
		s.hits = append(s.hits, searchHit{kind: artistHit, artist: a})
		s.rows = append(s.rows, widgets.Row{Title: a.Name, Subtitle: "Artist", Round: true})
		s.arts = append(s.arts, art(a.Name, a.Cover))
	}
	for i := range s.rows {
		s.rows[i].Art = &s.arts[i]
	}
	s.list.SetCount(len(s.hits))
	if len(s.hits) == 0 && s.onResults {
		s.toKeyboard(time.Time{})
	}
}

// Handle types on the keyboard, searches, and plays, opens or queues a
// result.
func (s *Search) Handle(ev input.Event, now time.Time) bool {
	if ev.Kind == input.Release {
		return false
	}
	if s.onResults {
		return s.handleResults(ev, now)
	}
	if ev.Kind == input.Press {
		switch ev.Button {
		case input.A:
			switch act, r := s.kb.Press(); act {
			case widgets.TypeKey:
				s.typeRune(r)
			case widgets.SpaceKey:
				s.space()
			case widgets.BackspaceKey:
				s.backspace()
			case widgets.SearchKey:
				s.search(now)
			}
			return true
		case input.X:
			return s.space()
		case input.Y:
			return s.search(now)
		}
	}
	if s.kb.Move(ev.Button, now) {
		return true
	}
	return ev.Button == input.Right && s.toResults(now)
}

// search asks Spotify for what's typed, or goes to the results it already
// has for it.
func (s *Search) search(now time.Time) bool {
	q := strings.TrimSpace(s.query)
	if q == "" {
		return false
	}
	if q != s.asked {
		s.asked, s.have, s.err, s.version = q, false, nil, 0
		s.res = spotify.Results{}
		s.build()
		s.list.SetCountFocus(len(s.hits), 0)
		s.refresh()
	}
	s.toResults(now)
	return true
}

func (s *Search) handleResults(ev input.Event, now time.Time) bool {
	if ev.Kind == input.Press {
		h := s.hits[s.list.Focus()]
		switch ev.Button {
		case input.A:
			switch h.kind {
			case songHit:
				s.e.Player.SetShuffle(false)
				s.e.Player.Play(Tracks(s.res.Tracks), h.song)
			case albumHit:
				s.e.Nav.Push(NewPlaylistPage(s.e, spotify.Playlist{URI: h.album.URI, Name: h.album.Name,
					Owner: h.album.Artist, Count: h.album.Tracks, Cover: h.album.Cover}))
			case artistHit:
				s.e.Nav.Push(NewPlaylistPage(s.e, spotify.Playlist{URI: h.artist.URI, Name: h.artist.Name,
					Owner: "Artist", Cover: h.artist.Cover}))
			}
			return true
		case input.Y:
			if h.kind != songHit {
				return false
			}
			s.e.Player.Enqueue(Tracks(s.res.Tracks[h.song : h.song+1]))
			s.e.Nav.Toast(screens.AddedToQueue)
			return true
		}
	}
	if s.list.Move(ev.Button, now) {
		return true
	}
	if ev.Button == input.Left {
		s.toKeyboard(now)
		return true
	}
	return false
}

// Back goes from the results to the keyboard, and on the keyboard deletes.
// With nothing typed it closes the search.
func (s *Search) Back() bool {
	if s.onResults {
		s.toKeyboard(time.Now())
		return true
	}
	return s.backspace()
}

func (s *Search) typeRune(r rune) {
	if utf8.RuneCountInString(s.query) < searchMaxQuery {
		s.query += string(r)
	}
}

func (s *Search) space() bool {
	if s.query == "" || strings.HasSuffix(s.query, " ") {
		return false
	}
	s.typeRune(' ')
	return true
}

func (s *Search) backspace() bool {
	if s.query == "" {
		return false
	}
	_, n := utf8.DecodeLastRuneInString(s.query)
	s.query = s.query[:len(s.query)-n]
	return true
}

func (s *Search) toResults(now time.Time) bool {
	if len(s.hits) == 0 {
		return false
	}
	s.onResults = true
	s.kb.SetActive(false, now)
	s.list.SetActive(true, now)
	return true
}

func (s *Search) toKeyboard(now time.Time) {
	s.onResults = false
	s.list.SetActive(false, now)
	s.kb.SetActive(true, now)
}

// Animating reports whether focus is moving.
func (s *Search) Animating(now time.Time) bool { return s.kb.Animating(now) || s.list.Animating(now) }

// message is what the results' side says when there are none to show.
func (s *Search) message() string {
	switch {
	case s.asked == "":
		return "Type, then Search, to find songs, albums and artists on Spotify."
	case s.err != nil && errors.Is(s.err, spotify.ErrOffline):
		return "You’re offline. Connect to Wi-Fi to search Spotify."
	case s.err != nil && strings.Contains(s.err.Error(), "limiting"):
		return "Spotify is limiting searches right now. Try again in a while."
	case s.err != nil:
		return "Couldn’t search Spotify. Check the connection and try again."
	case !s.have:
		return "Searching Spotify…"
	}
	return "Nothing on Spotify matches “" + s.asked + "”"
}

// Draw renders the frame.
func (s *Search) Draw(u *ui.Context) {
	s.refresh()
	widgets.DrawBackground(u, widgets.TabGround)
	s.kb.Draw(u, searchKbX, searchKbY, s.query)
	label := "RESULTS"
	if n := len(s.hits); n > 0 {
		label += " · " + strconv.Itoa(n)
	}
	u.Text(label, searchLabel, searchResX, searchKbY, theme.Ink3)
	if len(s.hits) > 0 {
		playing := s.e.Player.State().Track.SpotifyID
		u.PushClip(screens.GlowArea)
		s.list.Draw(u.Now, searchResX, searchListTop, func(i int, x, y, k float32) {
			box := gfx.Rect{X: x, Y: y, W: searchResW, H: searchRowH}
			r := s.rows[i]
			r.Playing = s.hits[i].kind == songHit && playing != "" && s.res.Tracks[s.hits[i].song].ID == playing
			widgets.RowGlow(u, box, widgets.RowRadius, k)
			u.PushClip(screens.Below(searchListTop))
			widgets.ListRow(u, r, box, k)
			u.PopClip()
		})
		u.PopClip()
	} else {
		box := gfx.Rect{X: searchResX, Y: searchListTop, W: searchResW, H: screens.ContentBot - searchListTop}
		lines := s.msg.Lines(u, s.message(), text.Body, box.W)
		lh := u.LineHeight(text.Body)
		y := box.Y + (box.H-float32(len(lines))*lh)/2
		for _, l := range lines {
			u.Text(l, text.Body, box.X+(box.W-u.TextWidth(l, text.Body))/2, y, theme.Ink2)
			y += lh
		}
	}
	widgets.StatusBar(u)
	widgets.Tabs(u, tab)
	widgets.HintBar(u, s.hints())
}

var (
	searchTypingHints = []widgets.Hint{{Key: "A", Label: "Type"}, {Key: "B", Label: "Delete"}, {Key: "X", Label: "Space"},
		{Key: "Y", Label: "Search"}}
	searchSongHints = []widgets.Hint{{Key: "A", Label: "Play"}, {Key: "Y", Label: "Add to queue"},
		{Key: "B", Label: "Keyboard"}}
	searchOpenHints = []widgets.Hint{{Key: "A", Label: "Open"}, {Key: "B", Label: "Keyboard"}}
)

func (s *Search) hints() []widgets.Hint {
	if !s.onResults {
		return searchTypingHints
	}
	if s.hits[s.list.Focus()].kind == songHit {
		return searchSongHints
	}
	return searchOpenHints
}
