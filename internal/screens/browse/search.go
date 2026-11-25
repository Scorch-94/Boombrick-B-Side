package browse

import (
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"

	"github.com/ujjwal313/boombrick/internal/library"
	"github.com/ujjwal313/boombrick/internal/platform/input"
	"github.com/ujjwal313/boombrick/internal/player"
	"github.com/ujjwal313/boombrick/internal/screens"
	"github.com/ujjwal313/boombrick/internal/theme"
	"github.com/ujjwal313/boombrick/internal/ui"
	"github.com/ujjwal313/boombrick/internal/ui/gfx"
	"github.com/ujjwal313/boombrick/internal/ui/text"
	"github.com/ujjwal313/boombrick/internal/ui/views"
	"github.com/ujjwal313/boombrick/internal/ui/widgets"
)

// The search screen, laid out as YouTube's: the keyboard on the left and the
// results on the right under their label, here every match, scrolling.
const (
	searchKbX      = side
	searchKbY      = contentTop + pad
	searchResX     = searchKbX + widgets.KeyboardW + 28
	searchResW     = theme.ScreenW - side - searchResX
	searchListTop  = searchKbY + 30 // under the label
	searchRowH     = 76
	searchMaxQuery = 80 // runes
	// searchMaxSongs caps the songs listed, so a single letter on a big
	// library doesn't build thousands of rows.
	searchMaxSongs = 200
)

var searchLabel = text.Style{Face: text.Figtree, Size: 20}

// Kinds of result, in the order they list.
const (
	artistHit = iota
	albumHit
	songHit
)

// hit is one search result: an artist, an album or a song of the index.
type hit struct {
	kind   int
	artist *library.Artist
	album  *library.Album
	song   *library.Track
}

// Search finds artists, albums and songs in the library as you type on the
// on-screen keyboard. Results update with every key; Right or Y goes to
// them.
type Search struct {
	nav       screens.Nav
	p         Player
	ix        *library.Index
	keys      searchKeys // the index's names, folded once
	kb        widgets.Keyboard
	query     string
	onResults bool
	hits      []hit
	songs     []*library.Track // the songs among hits, in order, for A to play from
	rows      []widgets.Row
	arts      []widgets.ArtSpec
	list      views.Grid
	msg       widgets.Wrapped
}

// searchKeys are the folded names searched: an artist's name, an album's
// title and artist, a song's title, artist and album.
type searchKeys struct {
	artists, albums, songs []string
}

// NewSearch opens the search on ix, with focus on the keyboard's "a".
func NewSearch(nav screens.Nav, p Player, ix *library.Index) *Search {
	s := &Search{nav: nav, p: p, ix: ix, kb: widgets.NewKeyboard()}
	s.keys.artists = make([]string, len(ix.Artists))
	for i, ar := range ix.Artists {
		s.keys.artists[i] = fold(ar.Name)
	}
	s.keys.albums = make([]string, len(ix.Albums))
	for i, a := range ix.Albums {
		s.keys.albums[i] = fold(a.Title + "\x00" + a.Artist)
	}
	s.keys.songs = make([]string, len(ix.Songs))
	for i, t := range ix.Songs {
		s.keys.songs[i] = fold(t.Title + "\x00" + t.Artist + "\x00" + t.Album)
	}
	s.find()
	return s
}

// fold lowers s and strips its accents, so "beyonce" finds "Beyoncé".
func fold(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range norm.NFD.String(s) {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

// matches reports whether every word of the query is in key.
func matches(key string, words []string) bool {
	for _, w := range words {
		if !strings.Contains(key, w) {
			return false
		}
	}
	return true
}

// find lists the query's matches: artists, then albums, then songs.
func (s *Search) find() {
	s.hits, s.songs = s.hits[:0], s.songs[:0]
	words := strings.Fields(fold(s.query))
	if len(words) > 0 {
		for i, k := range s.keys.artists {
			if matches(k, words) {
				s.hits = append(s.hits, hit{kind: artistHit, artist: s.ix.Artists[i]})
			}
		}
		for i, k := range s.keys.albums {
			if matches(k, words) {
				s.hits = append(s.hits, hit{kind: albumHit, album: s.ix.Albums[i]})
			}
		}
		for i, k := range s.keys.songs {
			if len(s.songs) == searchMaxSongs {
				break
			}
			if matches(k, words) {
				s.hits = append(s.hits, hit{kind: songHit, song: s.ix.Songs[i]})
				s.songs = append(s.songs, s.ix.Songs[i])
			}
		}
	}
	s.rows, s.arts = s.rows[:0], s.arts[:0]
	for _, h := range s.hits {
		r, art := hitRow(h)
		s.rows, s.arts = append(s.rows, r), append(s.arts, art)
	}
	for i := range s.rows {
		s.rows[i].Art = &s.arts[i]
	}
	s.list = views.Grid{Cols: 1, CellW: searchResW, CellH: searchRowH, Pad: pad, ViewH: contentBot - searchListTop}
	s.list.SetCount(len(s.hits))
	if len(s.hits) == 0 && s.onResults {
		s.onResults = false
		s.kb.SetActive(true, time.Time{})
	}
	if !s.onResults {
		s.list.SetActive(false, time.Time{})
	}
}

// hitRow is h's row: its name, what it is, and its art.
func hitRow(h hit) (widgets.Row, widgets.ArtSpec) {
	switch h.kind {
	case artistHit:
		return widgets.Row{Title: h.artist.Name, Subtitle: "Artist · " + artistLine(len(h.artist.Albums), h.artist.Songs),
			Round: true}, screens.ArtistArt(h.artist)
	case albumHit:
		sub := "Album · " + h.album.Artist
		if h.album.Year > 0 {
			sub += " · " + strconv.Itoa(h.album.Year)
		}
		return widgets.Row{Title: h.album.Title, Subtitle: sub}, screens.AlbumArt(h.album)
	}
	r := widgets.Row{Title: h.song.Title, Subtitle: "Song · " + h.song.Artist + " · " + h.song.Album}
	if h.song.Duration > 0 {
		r.Duration = screens.Clock(h.song.Duration)
	}
	return r, screens.SongArt(h.song)
}

// Handle types and deletes on the keyboard, and plays, opens or queues a
// result. B is Back.
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
				s.toResults(now)
			}
			return true
		case input.X:
			return s.space()
		case input.Y:
			return s.toResults(now)
		}
	}
	if s.kb.Move(ev.Button, now) {
		return true
	}
	return ev.Button == input.Right && s.toResults(now)
}

// handleResults plays a song from the songs found, opens an artist or an
// album, and queues any of them.
func (s *Search) handleResults(ev input.Event, now time.Time) bool {
	if ev.Kind == input.Press {
		h := s.hits[s.list.Focus()]
		switch ev.Button {
		case input.A:
			switch h.kind {
			case artistHit:
				s.nav.Push(NewArtistPage(s.nav, s.p, h.artist))
			case albumHit:
				s.nav.Push(NewAlbumPage(s.nav, s.p, h.album))
			default:
				s.p.SetShuffle(false)
				s.p.Play(songTracks(s.songs), s.songIndex(h.song))
			}
			return true
		case input.Y:
			s.p.Enqueue(hitTracks(h))
			s.nav.Toast(screens.AddedToQueue)
			return true
		case input.X:
			if h.kind != artistHit {
				return false
			}
			shuffle(s.p, artistTracks(h.artist))
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

func (s *Search) songIndex(t *library.Track) int {
	for i, x := range s.songs {
		if x == t {
			return i
		}
	}
	return 0
}

// hitTracks is what Y queues for h.
func hitTracks(h hit) []player.Track {
	switch h.kind {
	case artistHit:
		return artistTracks(h.artist)
	case albumHit:
		return screens.Tracks(h.album.Tracks)
	}
	return songTracks([]*library.Track{h.song})
}

// Back takes B: from the results to the keyboard, and on the keyboard it
// deletes. With nothing typed B closes the search.
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
		s.find()
	}
}

// space adds a space between words: never a leading one or two together.
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
	s.find()
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

// Draw renders the frame.
func (s *Search) Draw(u *ui.Context) {
	widgets.DrawBackground(u, widgets.TabGround)
	s.kb.Draw(u, searchKbX, searchKbY, s.query)
	label := "RESULTS"
	if n := len(s.hits); n > 0 {
		label += " · " + strconv.Itoa(n)
	}
	u.Text(label, searchLabel, searchResX, searchKbY, theme.Ink3)
	if len(s.hits) > 0 {
		s.drawResults(u)
	} else {
		msg := "Type to search your artists, albums and songs."
		if strings.TrimSpace(s.query) != "" {
			msg = "Nothing in your library matches “" + strings.TrimSpace(s.query) + "”"
		}
		box := gfx.Rect{X: searchResX, Y: searchListTop, W: searchResW, H: contentBot - searchListTop}
		lines := s.msg.Lines(u, msg, text.Body, box.W)
		lh := u.LineHeight(text.Body)
		y := box.Y + (box.H-float32(len(lines))*lh)/2
		for _, l := range lines {
			u.Text(l, text.Body, box.X+(box.W-u.TextWidth(l, text.Body))/2, y, theme.Ink2)
			y += lh
		}
	}
	widgets.StatusBar(u)
	widgets.Tabs(u, widgets.TabLibrary)
	widgets.HintBar(u, s.hints())
}

func (s *Search) drawResults(u *ui.Context) {
	playing := s.p.State().Track.Path
	u.PushClip(screens.GlowArea)
	s.list.Draw(u.Now, searchResX, searchListTop, func(i int, x, y, k float32) {
		box := gfx.Rect{X: x, Y: y, W: searchResW, H: searchRowH}
		r := s.rows[i]
		r.Playing = s.hits[i].kind == songHit && s.hits[i].song.Path == playing
		widgets.RowGlow(u, box, widgets.RowRadius, k)
		u.PushClip(screens.Below(searchListTop))
		widgets.ListRow(u, r, box, k)
		u.PopClip()
	})
	u.PopClip()
}

// The search's hint bars: typing, with or without results to go to, and on
// a song, an album or an artist.
var (
	searchTypingHints = []widgets.Hint{{Key: "A", Label: "Type"}, {Key: "B", Label: "Delete"}, {Key: "X", Label: "Space"},
		{Key: "Y", Label: "To results"}}
	searchAloneHints = []widgets.Hint{{Key: "A", Label: "Type"}, {Key: "B", Label: "Delete"}, {Key: "X", Label: "Space"}}
	searchSongHints  = []widgets.Hint{{Key: "A", Label: "Play"}, {Key: "Y", Label: "Add to queue"},
		{Key: "B", Label: "Keyboard"}}
	searchAlbumHints = []widgets.Hint{{Key: "A", Label: "Open"}, {Key: "Y", Label: "Add to queue"},
		{Key: "B", Label: "Keyboard"}}
	searchArtistHints = []widgets.Hint{{Key: "A", Label: "Open"}, {Key: "X", Label: "Shuffle"},
		{Key: "Y", Label: "Add to queue"}, {Key: "B", Label: "Keyboard"}}
)

func (s *Search) hints() []widgets.Hint {
	switch {
	case s.onResults:
		switch s.hits[s.list.Focus()].kind {
		case artistHit:
			return searchArtistHints
		case albumHit:
			return searchAlbumHints
		}
		return searchSongHints
	case len(s.hits) > 0:
		return searchTypingHints
	}
	return searchAloneHints
}
