package browse

import (
	"time"
	"unicode/utf8"

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

// PlaylistEdits is what making and changing playlists needs from the app.
type PlaylistEdits interface {
	// Music is the music folder, whose Playlists folder they're in.
	Music() string
	// Playlists are the playlists as last read.
	Playlists() []*library.Playlist
	// Song is the library's song with the file at path, or nil.
	Song(path string) *library.Track
	// Saved says a playlist file changed, to read them again.
	Saved()
}

// Edits makes playlists editable: X adds a song to one, and a playlist's
// page edits it. The app sets it; nil leaves playlists as they are.
var Edits PlaylistEdits

// notPlaylistable is the toast for a song a playlist can't hold.
const notPlaylistable = "Only songs on your SD card or from Spotify can go in a playlist"

// EntryOf is t as a playlist entry: the library's song, or a Spotify song
// to stream. Audiobooks, episodes, stations and YouTube songs not
// downloaded have none.
func EntryOf(t player.Track) (library.PlaylistEntry, bool) {
	if Edits == nil {
		return library.PlaylistEntry{}, false
	}
	if t.Path != "" {
		if s := Edits.Song(t.Path); s != nil {
			return library.PlaylistEntry{Song: s}, true
		}
		return library.PlaylistEntry{}, false
	}
	if len(t.SpotifyID) == 22 { // a Spotify song's ID; an episode's is "episode:…"
		return library.PlaylistEntry{Stream: &library.StreamSong{SpotifyID: t.SpotifyID, Title: t.Title,
			Artist: t.Artist, Album: t.Album, Duration: t.Duration}}, true
	}
	return library.PlaylistEntry{}, false
}

// AddToPlaylist opens the list of playlists to add t to, on nav; done, if
// set, runs once one is chosen. A song no playlist can hold gets a toast.
func AddToPlaylist(nav screens.Nav, t player.Track, done func()) {
	e, ok := EntryOf(t)
	if !ok {
		nav.Toast(notPlaylistable)
		return
	}
	nav.Push(newAddTo(nav, e, t.Title, done))
}

// addTo lists the playlists to add a song to, New playlist first.
type addTo struct {
	nav   screens.Nav
	entry library.PlaylistEntry
	song  string
	done  func()
	lists []*library.Playlist
	rows  []widgets.Row
	arts  []widgets.ArtSpec
	list  views.Grid
}

func newAddTo(nav screens.Nav, e library.PlaylistEntry, song string, done func()) *addTo {
	a := &addTo{nav: nav, entry: e, song: song, done: done}
	for _, pl := range Edits.Playlists() {
		if pl.Editable() {
			a.lists = append(a.lists, pl)
		}
	}
	a.rows = append(a.rows, widgets.Row{Title: "New playlist", Subtitle: "Name it, with this song in it"})
	a.arts = make([]widgets.ArtSpec, len(a.lists))
	for i, pl := range a.lists {
		a.arts[i] = screens.PlaylistArt(pl)
		a.rows = append(a.rows, widgets.Row{Art: &a.arts[i], Title: pl.Name, Subtitle: playlistLine(pl)})
	}
	a.list = views.Grid{Cols: 1, CellW: theme.ScreenW - 2*side, CellH: widgets.RowHeight, Pad: pad,
		ViewH: contentBot - addToTop}
	a.list.SetCount(len(a.rows))
	return a
}

// addToTop is where the list starts, under the page's heading.
const addToTop = contentTop + 96

var addToHints = []widgets.Hint{{Key: "A", Label: "Add"}, {Key: "B", Label: "Cancel"}}

// Handle adds the song to the playlist focused, or names a new one for it.
func (a *addTo) Handle(ev input.Event, now time.Time) bool {
	if ev.Kind == input.Press && ev.Button == input.A {
		i := a.list.Focus()
		if i == 0 {
			a.nav.Push(newNameEntry(a.nav, "New playlist", "", func(name string) bool {
				pl, err := library.NewPlaylist(Edits.Music(), name)
				if err != nil {
					a.nav.Toast("Couldn’t make the playlist")
					return false
				}
				a.nav.Pop() // the name
				if a.add(pl) != nil {
					a.nav.Toast("Couldn’t save the playlist")
				}
				return true
			}))
			return true
		}
		if a.add(a.lists[i-1]) != nil {
			a.nav.Toast("Couldn’t save the playlist")
		}
		return true
	}
	return a.list.Move(ev.Button, now)
}

// add puts the song at the end of pl, saves it and closes the list.
func (a *addTo) add(pl *library.Playlist) error {
	pl.SetEntries(append(append([]library.PlaylistEntry(nil), pl.Entries...), a.entry))
	if err := pl.Save(Edits.Music()); err != nil {
		return err
	}
	Edits.Saved()
	a.nav.Pop()
	a.nav.Toast("Added to " + pl.Name)
	if a.done != nil {
		a.done()
	}
	return nil
}

// Animating reports whether focus is moving.
func (a *addTo) Animating(now time.Time) bool { return a.list.Animating(now) }

// Draw renders the frame.
func (a *addTo) Draw(u *ui.Context) {
	widgets.DrawBackground(u, widgets.TabGround)
	u.Text("Add to playlist", text.Heading, side, contentTop+8, theme.Paper)
	u.Text(u.Fit(a.song, text.Meta, theme.ScreenW-2*side), text.Meta, side, contentTop+8+u.LineHeight(text.Heading), theme.Ink2)
	u.PushClip(screens.GlowArea)
	a.list.Draw(u.Now, side, addToTop, func(i int, x, y, k float32) {
		box := gfx.Rect{X: x, Y: y, W: a.list.CellW, H: widgets.RowHeight}
		widgets.RowGlow(u, box, widgets.RowRadius, k)
		u.PushClip(screens.Below(addToTop))
		widgets.ListRow(u, a.rows[i], box, k)
		u.PopClip()
	})
	u.PopClip()
	widgets.StatusBar(u)
	widgets.HintBar(u, addToHints)
}

// nameEntry types a playlist's name on the on-screen keyboard; Save hands it
// to save, which reports whether it was taken, and B leaves it.
type nameEntry struct {
	nav   screens.Nav
	title string
	name  string
	kb    widgets.Keyboard
	save  func(name string) bool
}

// maxName is the longest name, in runes.
const maxName = 60

func newNameEntry(nav screens.Nav, title, name string, save func(string) bool) *nameEntry {
	n := &nameEntry{nav: nav, title: title, name: name, save: save, kb: widgets.NewKeyboard()}
	n.kb.Done, n.kb.DoneIcon = "Save", "check"
	n.kb.SetActive(true, time.Time{})
	return n
}

var nameHints = []widgets.Hint{{Key: "A", Label: "Type"}, {Key: "X", Label: "Space"}, {Key: "Y", Label: "Save"},
	{Key: "B", Label: "Cancel"}}

// Handle types with A, spaces with X and saves with Y or the Save key.
func (n *nameEntry) Handle(ev input.Event, now time.Time) bool {
	if ev.Kind == input.Release {
		return false
	}
	if ev.Kind == input.Press {
		switch ev.Button {
		case input.A:
			switch act, r := n.kb.Press(); act {
			case widgets.TypeKey:
				n.typeRune(r)
			case widgets.SpaceKey:
				n.typeRune(' ')
			case widgets.BackspaceKey:
				if _, size := utf8.DecodeLastRuneInString(n.name); size > 0 {
					n.name = n.name[:len(n.name)-size]
				}
			case widgets.SearchKey:
				n.done()
			}
			return true
		case input.X:
			n.typeRune(' ')
			return true
		case input.Y:
			n.done()
			return true
		}
	}
	return n.kb.Move(ev.Button, now)
}

func (n *nameEntry) typeRune(r rune) {
	if utf8.RuneCountInString(n.name) < maxName && !(r == ' ' && (n.name == "" || n.name[len(n.name)-1] == ' ')) {
		n.name += string(r)
	}
}

func (n *nameEntry) done() {
	if library.CleanPlaylistName(n.name) == "" {
		n.nav.Toast("Type a name first")
		return
	}
	n.save(n.name)
}

// Animating reports whether focus is moving.
func (n *nameEntry) Animating(now time.Time) bool { return n.kb.Animating(now) }

// Draw renders the frame.
func (n *nameEntry) Draw(u *ui.Context) {
	widgets.DrawBackground(u, widgets.TabGround)
	n.kb.Draw(u, searchKbX, searchKbY, n.name)
	x := float32(searchResX)
	u.Text(n.title, text.Heading, x, searchKbY, theme.Paper)
	u.Text("Type the playlist’s name, then Save.", text.Meta, x, searchKbY+u.LineHeight(text.Heading), theme.Ink2)
	widgets.StatusBar(u)
	widgets.HintBar(u, nameHints)
}
