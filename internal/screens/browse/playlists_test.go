package browse

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ujjwal313/boombrick/internal/library"
	"github.com/ujjwal313/boombrick/internal/platform/input"
	"github.com/ujjwal313/boombrick/internal/player"
	"github.com/ujjwal313/boombrick/internal/screens"
	"github.com/ujjwal313/boombrick/internal/ui/uitest"
)

// fakeEdits is a music folder of real files, its playlists read again on
// each save.
type fakeEdits struct {
	music string
	ix    *library.Index
	saves int
}

func (e *fakeEdits) Music() string                  { return e.music }
func (e *fakeEdits) Playlists() []*library.Playlist { return e.ix.Playlists }
func (e *fakeEdits) Song(path string) *library.Track {
	return e.ix.Song(path)
}
func (e *fakeEdits) Saved() {
	e.saves++
	e.ix.Playlists = library.ReadPlaylists(e.music, e.ix)
}

// stackNav keeps the pages pushed, as the app's does, popping them too.
type stackNav struct {
	stack  []screens.Screen
	toasts []string
}

func (n *stackNav) Push(s screens.Screen) { n.stack = append(n.stack, s) }
func (n *stackNav) Pop()                  { n.stack = n.stack[:len(n.stack)-1] }
func (n *stackNav) Toast(msg string)      { n.toasts = append(n.toasts, msg) }
func (n *stackNav) top() screens.Screen   { return n.stack[len(n.stack)-1] }

func editsIn(t *testing.T) *fakeEdits {
	t.Helper()
	music := t.TempDir()
	albums := sampleAlbums()
	for i := range albums {
		for j := range albums[i].Tracks {
			albums[i].Tracks[j].Path = filepath.Join(music, strings.TrimPrefix(albums[i].Tracks[j].Path, "/m/"))
		}
	}
	e := &fakeEdits{music: music, ix: library.NewIndex(albums)}
	Edits = e
	t.Cleanup(func() { Edits = nil })
	return e
}

func typeName(n *nameEntry, s string) {
	for _, r := range s {
		n.typeRune(r)
	}
}

// X on a song opens the playlists to add it to; New playlist names one and
// makes it with the song in it, and a playlist chosen gets it at the end.
func TestAddToPlaylist(t *testing.T) {
	e := editsIn(t)
	nav := &stackNav{}
	rust := e.ix.Songs[0]
	AddToPlaylist(nav, screens.TrackOf(rust), nil)
	add := nav.top().(*addTo)
	uitest.Golden(t, "add-to-playlist", render(t, add, t0))
	add.Handle(press(input.A), t0) // New playlist
	name := nav.top().(*nameEntry)
	typeName(name, "Road trip")
	uitest.Golden(t, "new-playlist-name", render(t, name, t0))
	name.Handle(press(input.Y), t0)
	if len(nav.stack) != 0 || len(e.ix.Playlists) != 1 || e.ix.Playlists[0].Name != "Road trip" {
		t.Fatalf("pages %d, playlists %v, toasts %v", len(nav.stack), e.ix.Playlists, nav.toasts)
	}
	if nav.toasts[len(nav.toasts)-1] != "Added to Road trip" {
		t.Fatalf("toasts %v", nav.toasts)
	}
	done := 0
	AddToPlaylist(nav, player.Track{SpotifyID: "4uLU6hMCjMI75M1A2tKUQC", Title: "Never", Artist: "Rick"}, func() { done++ })
	nav.top().Handle(press(input.Down), t0)
	nav.top().Handle(press(input.A), t0)
	pl := e.ix.Playlists[0]
	if len(pl.Entries) != 2 || pl.Entries[0].Song.Path != rust.Path || pl.Entries[1].Stream.SpotifyID != "4uLU6hMCjMI75M1A2tKUQC" || done != 1 {
		t.Fatalf("entries %+v, done %d", pl.Entries, done)
	}
	// An episode, a station or an audiobook can't go in.
	AddToPlaylist(nav, player.Track{SpotifyID: "episode:abc"}, nil)
	AddToPlaylist(nav, player.Track{Path: "/Audiobooks/Dune.m4b"}, nil)
	if len(nav.stack) != 0 || nav.toasts[len(nav.toasts)-1] != notPlaylistable {
		t.Fatalf("pages %d, toasts %v", len(nav.stack), nav.toasts)
	}
}

// Select edits a playlist: A picks a song up and moves it, X takes one
// out, Y renames and Start deletes, each saved at once.
func TestEditPlaylist(t *testing.T) {
	e := editsIn(t)
	pl, err := library.NewPlaylist(e.music, "Mix")
	if err != nil {
		t.Fatal(err)
	}
	song := func(title string) *library.Track {
		for _, s := range e.ix.Songs {
			if s.Title == title {
				return s
			}
		}
		t.Fatalf("no %q", title)
		return nil
	}
	pl.SetEntries([]library.PlaylistEntry{{Song: song("Rust")}, {Song: song("Ember")}, {Song: song("Tin Roof")}})
	pl.Save(e.music)
	e.Saved()
	nav := &stackNav{}
	pg := NewPlaylistPage(nav, &fakePlayer{}, e.ix.Playlists[0])
	nav.Push(pg)
	if !pg.Claims(input.Select) || pg.Claims(input.Start) {
		t.Fatal("before editing, the page claims Select only")
	}
	pg.Handle(press(input.Select), t0)
	pg.Handle(press(input.A), t0) // pick the first up
	pg.Handle(press(input.Down), t0)
	pg.Handle(press(input.Down), t0)
	uitest.Golden(t, "playlist-moving", render(t, pg, t0))
	pg.Handle(press(input.A), t0) // drop it last
	titles := func() []string {
		var out []string
		for _, s := range e.ix.Playlists[0].Songs {
			out = append(out, s.Title)
		}
		return out
	}
	if got := titles(); !slices.Equal(got, []string{"Ember", "Tin Roof", "Rust"}) {
		t.Fatalf("after the move %v", got)
	}
	// B while moving puts the song back.
	pg.Handle(press(input.Up), t0)
	pg.Handle(press(input.A), t0)
	pg.Handle(press(input.Up), t0)
	pg.Back()
	if got := pg.rows[1].Title; got != "Tin Roof" {
		t.Fatalf("put back: row 1 is %q", got)
	}
	pg.Handle(press(input.X), t0) // take Tin Roof out
	if got := titles(); !slices.Equal(got, []string{"Ember", "Rust"}) {
		t.Fatalf("after removing %v", got)
	}
	pg.Handle(press(input.Y), t0)
	name := nav.top().(*nameEntry)
	name.name = ""
	typeName(name, "Evening")
	name.Handle(press(input.Y), t0)
	if e.ix.Playlists[0].Name != "Evening" || pg.title != "Evening" || nav.top() != pg {
		t.Fatalf("renamed to %q, title %q", e.ix.Playlists[0].Name, pg.title)
	}
	pg.Handle(press(input.Start), t0)
	uitest.Golden(t, "playlist-delete", render(t, pg, t0))
	pg.Handle(press(input.A), t0)
	if len(e.ix.Playlists) != 0 || len(nav.stack) != 0 {
		t.Fatalf("after deleting: %d playlists, %d pages", len(e.ix.Playlists), len(nav.stack))
	}
	if es, _ := os.ReadDir(filepath.Join(e.music, library.PlaylistDir)); len(es) != 0 {
		t.Fatalf("files left: %v", es)
	}
}

// Select on the Playlists filter makes an empty playlist.
func TestNewPlaylistFromTheLibrary(t *testing.T) {
	e := editsIn(t)
	nav := &stackNav{}
	l := New(nav, &fakePlayer{}, e.music, nil)
	l.SetIndex(e.ix)
	for range 3 {
		l.Handle(press(input.R2), t0)
	}
	if !l.Claims(input.Select) {
		t.Fatal("the Playlists filter doesn't take Select")
	}
	l.Handle(press(input.Select), t0)
	name := nav.top().(*nameEntry)
	typeName(name, "Focus")
	name.Handle(press(input.Y), t0)
	if len(e.ix.Playlists) != 1 || e.ix.Playlists[0].Name != "Focus" || len(nav.stack) != 0 {
		t.Fatalf("playlists %v, pages %d", e.ix.Playlists, len(nav.stack))
	}
}
