package browse

import (
	"strconv"
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

// The album page's layout (screens/layout.go).
const (
	albumArt = screens.PageArt
	albumTop = screens.PageTop
	listX    = screens.PageListX
)

// AlbumPage shows an album's tracks, or a genre's or a playlist's songs laid
// out the same way. Focus starts on the first track: A plays from the one
// focused, Y queues it.
type AlbumPage struct {
	nav    screens.Nav
	p      Player
	art    widgets.ArtSpec
	title  string
	who    string // the artist and the year when known, or a genre's artist count
	length string // the songs and their time
	tracks []player.Track
	rows   []widgets.Row
	list   views.Grid
	bar    []widgets.Hint // the hint bar in albumHints' place, as a playlist being edited has; nil: albumHints
}

// NewAlbumPage opens album a.
func NewAlbumPage(nav screens.Nav, p Player, a *library.Album) *AlbumPage {
	who := a.Artist
	if a.Year > 0 {
		who += " · " + strconv.Itoa(a.Year)
	}
	return newPage(nav, p, screens.AlbumArt(a), a.Title, who, screens.SongsAndTime(len(a.Tracks), a.Duration),
		screens.Tracks(a.Tracks))
}

// NewGenrePage opens genre g: its songs, each with its artist.
func NewGenrePage(nav screens.Nav, p Player, g *library.Genre) *AlbumPage {
	who := screens.Count(g.Artists) + " artists"
	if g.Artists == 1 {
		who = "1 artist"
	}
	pg := newPage(nav, p, screens.GenreArt(g), g.Name, who, screens.SongsAndTime(len(g.Songs), g.Duration),
		songTracks(g.Songs))
	for i, s := range g.Songs {
		pg.rows[i].Subtitle = s.Artist
	}
	return pg
}

// playlistPage lays playlist pl out: its songs in the file's order, each
// with its artist, and the ones streamed from Spotify marked so.
func playlistPage(nav screens.Nav, p Player, pl *library.Playlist) *AlbumPage {
	// The songs from Spotify say so in their rows; the line under the art
	// has room only for what's missing.
	who := "Playlist"
	if pl.Missing > 0 {
		who += " · " + strconv.Itoa(pl.Missing) + " not found"
	}
	pg := newPage(nav, p, screens.PlaylistArt(pl), pl.Name, who, screens.SongsAndTime(len(pl.Entries), pl.Duration),
		screens.PlaylistTracks(pl))
	for i, e := range pl.Entries {
		if e.Song != nil {
			pg.rows[i].Subtitle = e.Song.Artist
		} else {
			pg.rows[i].Subtitle = "Spotify · " + e.Stream.Artist
		}
	}
	return pg
}

// playlistLine is a playlist's songs and time, and how many of its entries
// stream from Spotify or aren't found.
func playlistLine(pl *library.Playlist) string {
	line := screens.SongsAndTime(len(pl.Entries), pl.Duration)
	if pl.Streams > 0 {
		line += " · " + strconv.Itoa(pl.Streams) + " Spotify"
	}
	if pl.Missing > 0 {
		line += " · " + strconv.Itoa(pl.Missing) + " not found"
	}
	return line
}

// newPage makes a page of tracks: art, title and two lines of details in
// the left column, the tracks numbered on the right.
func newPage(nav screens.Nav, p Player, art widgets.ArtSpec, title, who, length string, tracks []player.Track) *AlbumPage {
	pg := &AlbumPage{nav: nav, p: p, art: art, title: title, who: who, length: length, tracks: tracks,
		rows: make([]widgets.Row, len(tracks))}
	pg.list = views.Grid{Cols: 1, CellW: theme.ScreenW - side - listX, CellH: widgets.RowHeight, Pad: pad,
		ViewH: contentBot - contentTop}
	pg.list.SetCount(len(tracks))
	for i, t := range tracks {
		pg.rows[i] = widgets.Row{Index: strconv.Itoa(i + 1), Title: t.Title}
		if t.Duration > 0 {
			pg.rows[i].Duration = screens.Clock(t.Duration)
		}
	}
	return pg
}

// Handle moves along the tracks, plays from the focused one with A and
// queues it with Y.
func (pg *AlbumPage) Handle(ev input.Event, now time.Time) bool {
	if ev.Kind == input.Release {
		return false
	}
	switch ev.Button {
	case input.A:
		if ev.Kind == input.Press && len(pg.tracks) > 0 {
			pg.p.SetShuffle(false)
			pg.p.Play(pg.tracks, pg.list.Focus())
		}
		return ev.Kind == input.Press
	case input.Y:
		if ev.Kind == input.Press && len(pg.tracks) > 0 {
			f := pg.list.Focus()
			pg.p.Enqueue(pg.tracks[f : f+1])
			pg.nav.Toast(screens.AddedToQueue)
		}
		return ev.Kind == input.Press
	case input.X:
		if ev.Kind == input.Press && len(pg.tracks) > 0 && Edits != nil {
			AddToPlaylist(pg.nav, pg.tracks[pg.list.Focus()], nil)
		}
		return ev.Kind == input.Press
	}
	return pg.list.Move(ev.Button, now)
}

// Animating reports whether focus or the list is moving.
func (pg *AlbumPage) Animating(now time.Time) bool {
	return pg.list.Animating(now)
}

// Draw renders the frame.
func (pg *AlbumPage) Draw(u *ui.Context) {
	now := u.Now
	widgets.DrawBackground(u, widgets.TabGround)

	widgets.Art(u, pg.art, albumArt, widgets.CardRadius, gfx.Rect{X: side, Y: albumTop, W: albumArt, H: albumArt})
	y := float32(albumTop + albumArt + pad)
	u.Text(u.Fit(pg.title, text.Heading, albumArt), text.Heading, side, y, theme.Paper)
	y += u.LineHeight(text.Heading)
	u.Text(u.Fit(pg.who, text.Meta, albumArt), text.Meta, side, y, theme.Ink2)
	y += u.LineHeight(text.Meta)
	u.Text(u.Fit(pg.length, text.Meta, albumArt), text.Meta, side, y, theme.Ink3)

	playing := pg.p.State().Track
	u.PushClip(screens.GlowArea)
	pg.list.Draw(now, listX, contentTop, func(i int, x, y, k float32) {
		r := pg.rows[i]
		r.Playing = sameTrack(pg.tracks[i], playing)
		box := gfx.Rect{X: x, Y: y, W: pg.list.CellW, H: widgets.RowHeight}
		widgets.RowGlow(u, box, widgets.RowRadius, k)
		u.PushClip(screens.Below(contentTop))
		widgets.ListRow(u, r, box, k)
		u.PopClip()
	})
	u.PopClip()

	widgets.StatusBar(u)
	widgets.Tabs(u, widgets.TabLibrary)
	widgets.HintBar(u, pg.hints())
}

// albumHints is the page's hint bar.
var (
	albumHints = []widgets.Hint{{Key: "A", Label: "Play"}, {Key: "Y", Label: "Add to queue"}, {Key: "B", Label: "Back"},
		{Key: "L1/R1", Label: "Tabs"}}
	// With playlists editable, X adds a song to one.
	albumEditHints = []widgets.Hint{{Key: "A", Label: "Play"}, {Key: "Y", Label: "Queue"}, {Key: "X", Label: "Add to playlist"},
		{Key: "B", Label: "Back"}}
)

func (pg *AlbumPage) hints() []widgets.Hint {
	switch {
	case pg.bar != nil:
		return pg.bar
	case Edits != nil:
		return albumEditHints
	}
	return albumHints
}

// sameTrack reports whether a and b are the same song: the same file, or
// with no file the same Spotify track.
func sameTrack(a, b player.Track) bool {
	if a.Path != "" || b.Path != "" {
		return a.Path == b.Path
	}
	return a.SpotifyID != "" && a.SpotifyID == b.SpotifyID
}
