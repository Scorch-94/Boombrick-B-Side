package browse

import (
	"path/filepath"
	"slices"
	"strings"
	"time"

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

const (
	albumsFilter = iota
	artistsFilter
	songsFilter
	playlistsFilter
	genresFilter
	filterCount
)

var filterNames = [filterCount]string{"Albums", "Artists", "Songs", "Playlists", "Genres"}

const listTop = contentTop + widgets.FiltersHeight

// Library is the Library tab: albums, artists, songs, playlists or genres,
// chosen with L2 and R2. The songs list opens with a Shuffle all songs row. Genres shows only when some song is tagged with
// one. Playlists are the .m3u files in the music folder's Playlists folder.
type Library struct {
	nav       screens.Nav
	p         Player
	musicDir  string
	rescan    func()
	ix        *library.Index
	filter    int
	views     [filterCount]views.Grid
	counts    [filterCount]string    // the filters' counts
	rows      [filterCount][]listRow // the artists', songs', playlists' and genres' rows, made as they first show
	names     []string               // the shown filters' names, for the filter row
	shownCts  []string               // the shown filters' counts, or only the chosen one's when all don't fit
	noLists   widgets.Wrapped        // the empty Playlists filter's message, wrapped
	noListMsg string                 // that message, made once
	allSongs  string                 // the Shuffle all row's line: the songs and their time
	laidOut   bool
	measured  bool            // allCounts is known for this index
	allCounts bool            // every filter's count fits in the row
	reading   bool            // the first scan is running with nothing indexed yet
	state     widgets.Wrapped // the empty or reading state's description, wrapped
}

// listRow is what an artist's, a song's or a genre's row shows that takes
// making.
type listRow struct {
	art      widgets.ArtSpec
	subtitle string
	duration string // a song's length, or ""
}

// New creates the tab. It shows nothing but its chrome until SetIndex.
func New(nav screens.Nav, p Player, musicDir string, rescan func()) *Library {
	l := &Library{nav: nav, p: p, musicDir: musicDir, rescan: rescan}
	l.views[albumsFilter] = screens.LibraryAlbumGrid()
	for _, f := range []int{artistsFilter, songsFilter, playlistsFilter, genresFilter} {
		l.views[f] = views.Grid{Cols: 1, CellW: theme.ScreenW - 2*side, CellH: widgets.RowHeight, Pad: pad,
			ViewH: contentBot - listTop}
	}
	return l
}

// SetIndex shows ix, keeping each filter's focus on the same album, artist,
// song or genre when it is still there. Without genres, the genres filter
// gives way to the songs.
func (l *Library) SetIndex(ix *library.Index) {
	old := l.ix
	l.ix = ix
	l.counts = [filterCount]string{screens.Count(len(ix.Albums)), screens.Count(len(ix.Artists)),
		screens.Count(len(ix.Songs)), screens.Count(len(ix.Playlists)), screens.Count(len(ix.Genres))}
	l.measured = false
	l.rows[artistsFilter] = make([]listRow, len(ix.Artists))
	l.rows[songsFilter] = make([]listRow, len(ix.Songs))
	var total time.Duration
	for _, t := range ix.Songs {
		total += t.Duration
	}
	l.allSongs = screens.SongsAndTime(len(ix.Songs), total)
	l.rows[playlistsFilter] = make([]listRow, len(ix.Playlists))
	l.rows[genresFilter] = make([]listRow, len(ix.Genres))
	if !slices.Contains(l.shown(), l.filter) {
		l.filter = songsFilter
	}
	if old == nil {
		l.views[albumsFilter].SetCount(len(ix.Albums))
		l.views[artistsFilter].SetCount(len(ix.Artists))
		l.views[songsFilter].SetCount(len(ix.Songs) + 1) // the Shuffle all row first
		l.views[playlistsFilter].SetCount(len(ix.Playlists))
		l.views[genresFilter].SetCount(len(ix.Genres))
		return
	}
	g := &l.views[albumsFilter]
	g.SetCountFocus(len(ix.Albums), refocus(old.Albums, ix.Albums, g.Focus(),
		func(a *library.Album) [2]string { return [2]string{a.Artist, a.Title} }))
	g = &l.views[artistsFilter]
	g.SetCountFocus(len(ix.Artists), refocus(old.Artists, ix.Artists, g.Focus(),
		func(a *library.Artist) string { return a.Name }))
	g = &l.views[songsFilter]
	f := g.Focus() // 0 is the Shuffle all row, which stays where it is
	if f > 0 {
		f = refocus(old.Songs, ix.Songs, f-1, func(s *library.Track) string { return s.Path }) + 1
	}
	g.SetCountFocus(len(ix.Songs)+1, f)
	g = &l.views[playlistsFilter]
	g.SetCountFocus(len(ix.Playlists), refocus(old.Playlists, ix.Playlists, g.Focus(),
		func(p *library.Playlist) string { return p.Path }))
	g = &l.views[genresFilter]
	g.SetCountFocus(len(ix.Genres), refocus(old.Genres, ix.Genres, g.Focus(),
		func(g *library.Genre) string { return strings.ToLower(g.Name) }))
}

// The filters shown: the genres only when there are some.
var (
	withGenres    = []int{albumsFilter, artistsFilter, songsFilter, playlistsFilter, genresFilter}
	withoutGenres = withGenres[:4]
)

// shown is the filters shown, in order.
func (l *Library) shown() []int {
	if len(l.ix.Genres) == 0 {
		return withoutGenres
	}
	return withGenres
}

// refocus finds the item that was at f in old within now, by key; without it
// focus keeps its position.
func refocus[T any, K comparable](old, now []T, f int, key func(T) K) int {
	if f < 0 || f >= len(old) {
		return f
	}
	k := key(old[f])
	if i := slices.IndexFunc(now, func(x T) bool { return key(x) == k }); i >= 0 {
		return i
	}
	return f
}

// SetReading says the first scan is running and nothing is indexed yet;
// until SetIndex the tab says the music is being read.
func (l *Library) SetReading() { l.reading = true }

func (l *Library) empty() bool { return l.ix != nil && len(l.ix.Albums) == 0 }

// Handle switches filters with L2/R2, moves with the D-pad, opens or plays
// with A and queues with Y. X opens the search. The empty
// library rescans with Y.
func (l *Library) Handle(ev input.Event, now time.Time) bool {
	if ev.Kind == input.Release || l.ix == nil {
		return false
	}
	if l.empty() {
		if ev.Kind == input.Press && ev.Button == input.Y && l.rescan != nil {
			l.rescan()
			l.nav.Toast(screens.Scanning)
			return true
		}
		return false
	}
	g := &l.views[l.filter]
	if ev.Kind == input.Press {
		switch ev.Button {
		case input.L2, input.R2:
			shown := l.shown()
			i := slices.Index(shown, l.filter) - 1
			if ev.Button == input.R2 {
				i += 2
			}
			if i < 0 || i >= len(shown) {
				return false
			}
			l.filter = shown[i]
			return true
		case input.A:
			if g.Count() > 0 {
				l.open(g.Focus())
			}
			return true
		case input.Y:
			if g.Count() > 0 {
				l.p.Enqueue(l.tracksAt(g.Focus()))
				l.nav.Toast(screens.AddedToQueue)
			} else if l.filter == playlistsFilter && l.rescan != nil {
				l.rescan()
				l.nav.Toast(screens.Scanning)
			}
			return true
		case input.X:
			l.nav.Push(NewSearch(l.nav, l.p, l.ix))
			return true
		}
	}
	return g.Move(ev.Button, now)
}

func (l *Library) open(i int) {
	switch l.filter {
	case albumsFilter:
		l.nav.Push(NewAlbumPage(l.nav, l.p, l.ix.Albums[i]))
	case artistsFilter:
		l.nav.Push(NewArtistPage(l.nav, l.p, l.ix.Artists[i]))
	case songsFilter:
		if i == 0 {
			shuffle(l.p, songTracks(l.ix.Songs))
			return
		}
		l.p.SetShuffle(false)
		l.p.Play(songTracks(l.ix.Songs), i-1)
	case playlistsFilter:
		l.nav.Push(NewPlaylistPage(l.nav, l.p, l.ix.Playlists[i]))
	case genresFilter:
		l.nav.Push(NewGenrePage(l.nav, l.p, l.ix.Genres[i]))
	}
}

// tracksAt is what Y queues for item i of the current filter.
func (l *Library) tracksAt(i int) []player.Track {
	switch l.filter {
	case albumsFilter:
		return screens.Tracks(l.ix.Albums[i].Tracks)
	case artistsFilter:
		return artistTracks(l.ix.Artists[i])
	case playlistsFilter:
		return songTracks(l.ix.Playlists[i].Songs)
	case genresFilter:
		return songTracks(l.ix.Genres[i].Songs)
	}
	if i == 0 { // the Shuffle all row: every song
		return songTracks(l.ix.Songs)
	}
	return []player.Track{screens.TrackOf(l.ix.Songs[i-1])}
}

// Animating reports whether the current view is moving.
func (l *Library) Animating(now time.Time) bool { return l.views[l.filter].Animating(now) }

// Draw renders the frame.
func (l *Library) Draw(u *ui.Context) {
	if !l.laidOut {
		l.laidOut = true
		screens.LayoutLibraryGrid(u, &l.views[albumsFilter], contentBot-listTop)
	}
	switch {
	case l.empty():
		widgets.DrawBackground(u, widgets.StateGround)
		widgets.EmptyState(u, &l.state, "Your library is empty", screens.EmptyLibraryText(), l.musicDir)
	case l.ix == nil && l.reading:
		screens.DrawReading(u, &l.state, l.musicDir)
	default:
		widgets.DrawBackground(u, widgets.TabGround)
		if l.ix != nil {
			l.drawFilters(u)
			l.drawView(u)
		}
	}
	widgets.StatusBar(u)
	widgets.Tabs(u, widgets.TabLibrary)
	widgets.HintBar(u, l.hints())
}

// drawFilters draws L2, the filters with their counts, and R2. When the
// counts don't all fit, only the chosen filter shows its count.
func (l *Library) drawFilters(u *ui.Context) {
	shown := l.shown()
	if !l.measured {
		l.measured = true
		l.names, l.shownCts = l.names[:0], l.shownCts[:0]
		for _, f := range shown {
			name := filterNames[f]
			if f == playlistsFilter && len(shown) == len(withGenres) {
				name = "Lists" // "Playlists" and "Genres" together are too wide for the row
			}
			l.names, l.shownCts = append(l.names, name), append(l.shownCts, l.counts[f])
		}
		l.allCounts = widgets.FiltersWidth(u, l.names, l.shownCts) <= theme.ScreenW-2*side
	}
	at := slices.Index(shown, l.filter)
	if !l.allCounts {
		for i, f := range shown {
			l.shownCts[i] = ""
			if i == at {
				l.shownCts[i] = l.counts[f]
			}
		}
	}
	widgets.Filters(u, l.names, l.shownCts, at, side, contentTop)
}

// row is item i's row in the artists, songs, playlists or genres, made the
// first time it shows rather than every frame. i counts from the first song,
// not the Shuffle all row.
func (l *Library) row(i int) *listRow {
	r := &l.rows[l.filter][i]
	if r.subtitle != "" {
		return r
	}
	switch l.filter {
	case artistsFilter:
		ar := l.ix.Artists[i]
		r.art, r.subtitle = screens.ArtistArt(ar), artistLine(len(ar.Albums), ar.Songs)
		return r
	case playlistsFilter:
		pl := l.ix.Playlists[i]
		r.art, r.subtitle = screens.PlaylistArt(pl), playlistLine(pl)
		return r
	case genresFilter:
		g := l.ix.Genres[i]
		r.art, r.subtitle = screens.GenreArt(g), screens.SongsAndTime(len(g.Songs), g.Duration)
		return r
	}
	s := l.ix.Songs[i]
	r.art, r.subtitle = screens.SongArt(s), s.Artist+" · "+s.Album
	if s.Duration > 0 {
		r.duration = screens.Clock(s.Duration)
	}
	return r
}

func (l *Library) drawView(u *ui.Context) {
	g := &l.views[l.filter]
	if l.filter == albumsFilter {
		drawLibraryAlbums(u, g, l.ix.Albums, func(i int) string { return l.ix.Albums[i].Artist })
		return
	}
	if l.filter == playlistsFilter && len(l.ix.Playlists) == 0 {
		l.drawNoPlaylists(u)
		return
	}
	playing := l.p.State().Track.Path
	u.PushClip(screens.GlowArea)
	g.Draw(u.Now, side, listTop, func(i int, x, y, k float32) {
		box := gfx.Rect{X: x, Y: y, W: g.CellW, H: widgets.RowHeight}
		if l.filter == songsFilter {
			if i == 0 {
				l.drawShuffleAll(u, box, k)
				return
			}
			i--
		}
		lr := l.row(i)
		r := widgets.Row{Art: &lr.art, Subtitle: lr.subtitle, Duration: lr.duration}
		switch l.filter {
		case artistsFilter:
			r.Title, r.Round = l.ix.Artists[i].Name, true
		case playlistsFilter:
			r.Title = l.ix.Playlists[i].Name
		case genresFilter:
			r.Title = l.ix.Genres[i].Name
		default:
			s := l.ix.Songs[i]
			r.Title, r.Playing = s.Title, s.Path == playing
		}
		widgets.RowGlow(u, box, widgets.RowRadius, k)
		u.PushClip(screens.Below(listTop))
		widgets.ListRow(u, r, box, k)
		u.PopClip()
	})
	u.PopClip()
}

// shuffleArt is the Shuffle all row's square, with the shuffle icon drawn
// over it.
var shuffleArt = widgets.ArtSpec{Hue: 20, Variant: 2}

// drawShuffleAll draws the row that shuffles every song in the library.
func (l *Library) drawShuffleAll(u *ui.Context, box gfx.Rect, k float32) {
	widgets.RowGlow(u, box, widgets.RowRadius, k)
	u.PushClip(screens.Below(listTop))
	widgets.ListRow(u, widgets.Row{Art: &shuffleArt, Title: "Shuffle all songs", Subtitle: l.allSongs}, box, k)
	// The art is 58 square, 16 in from the row's left, centred down it.
	u.Icon("shuffle", 30, 2.5, false, box.X+16+14, box.Y+(box.H-30)/2, theme.Paper)
	u.PopClip()
}

// drawNoPlaylists says where playlists go, in the list's place.
func (l *Library) drawNoPlaylists(u *ui.Context) {
	if l.noListMsg == "" {
		l.noListMsg = "No playlists yet. Put .m3u files in " + filepath.Join(l.musicDir, library.PlaylistDir) +
			", then press Y to rescan."
	}
	w := float32(theme.ScreenW - 4*side)
	lines := l.noLists.Lines(u, l.noListMsg, text.Body, w)
	lh := u.LineHeight(text.Body)
	y := listTop + (contentBot-listTop-float32(len(lines))*lh)/2
	for _, s := range lines {
		u.Text(s, text.Body, (theme.ScreenW-u.TextWidth(s, text.Body))/2, y, theme.Ink2)
		y += lh
	}
}

// The tab's hint bars: before the first index, for an empty library, and
// with albums, artists or genres, or songs shown.
var (
	tabHints    = []widgets.Hint{{Key: "L1/R1", Label: "Tabs"}}
	rescanHints = []widgets.Hint{{Key: "Y", Label: "Rescan SD card"}, {Key: "L1/R1", Label: "Tabs"}}
	openHints   = []widgets.Hint{{Key: "A", Label: "Open"}, {Key: "X", Label: "Search"}, {Key: "Y", Label: "Add to queue"},
		{Key: "L2/R2", Label: "Filter"}, {Key: "L1/R1", Label: "Tabs"}}
	noPlaylistHints = []widgets.Hint{{Key: "X", Label: "Search"}, {Key: "Y", Label: "Rescan SD card"},
		{Key: "L2/R2", Label: "Filter"}, {Key: "L1/R1", Label: "Tabs"}}
	// The tab row shows L1 and R1 itself; there is no room for their hint.
	shuffleAllHints = []widgets.Hint{{Key: "A", Label: "Shuffle all"}, {Key: "X", Label: "Search"},
		{Key: "Y", Label: "Add to queue"}, {Key: "L2/R2", Label: "Filter"}}
	songHints = []widgets.Hint{{Key: "A", Label: "Play"}, {Key: "X", Label: "Search"}, {Key: "Y", Label: "Add to queue"},
		{Key: "L2/R2", Label: "Filter"}, {Key: "L1/R1", Label: "Tabs"}}
)

func (l *Library) hints() []widgets.Hint {
	switch {
	case l.ix == nil:
		return tabHints
	case l.empty():
		return rescanHints
	case l.filter == playlistsFilter && len(l.ix.Playlists) == 0:
		return noPlaylistHints
	case l.filter == songsFilter && l.views[songsFilter].Focus() == 0:
		return shuffleAllHints
	case l.filter == songsFilter:
		return songHints
	}
	return openHints
}
