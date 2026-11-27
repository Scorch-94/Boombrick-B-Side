package settings

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ujjwal313/boombrick/internal/library"
	"github.com/ujjwal313/boombrick/internal/platform/input"
	"github.com/ujjwal313/boombrick/internal/ui/anim"
	"github.com/ujjwal313/boombrick/internal/ui/uitest"
)

// fakeLibrary is a library whose downloads are under /m/Boombrick, and which
// records what it is asked to delete.
type fakeLibrary struct {
	ix      *library.Index
	deleted [][]string
}

func (f *fakeLibrary) Index() *library.Index { return f.ix }
func (f *fakeLibrary) Downloaded(t *library.Track) bool {
	return strings.HasPrefix(t.Path, "/m/Boombrick/")
}
func (f *fakeLibrary) Delete(ts []*library.Track) int {
	var ps []string
	for _, t := range ts {
		ps = append(ps, t.Path)
	}
	f.deleted = append(f.deleted, ps)
	return len(ts)
}

func dupeLibrary() *fakeLibrary {
	tr := func(path, title, artist, album string, d time.Duration) library.Track {
		return library.Track{Path: path, Title: title, Artist: artist, Album: album, Duration: d}
	}
	return &fakeLibrary{ix: library.NewIndex([]library.Album{
		{Artist: "Odalys", Title: "Copper Season", Tracks: []library.Track{
			tr("/m/Odalys/Copper Season/01.flac", "Ember", "Odalys", "Copper Season", 200*time.Second),
			tr("/m/Odalys/Copper Season/02.flac", "Rust", "Odalys", "Copper Season", 180*time.Second)}},
		{Artist: "Odalys", Title: "Ember", Tracks: []library.Track{
			tr("/m/Boombrick/Odalys/Ember/01.opus", "Ember", "Odalys", "Ember", 201*time.Second)}},
		{Artist: "Kenji Arata", Title: "Neon", Tracks: []library.Track{
			tr("/m/Kenji Arata/Neon/01.flac", "Neon", "Kenji Arata", "Neon", 220*time.Second),
			tr("/m/Boombrick/Kenji Arata/Neon/01.opus", "Neon", "Kenji Arata", "Neon", 221*time.Second)}},
	})}
}

// The Library page lists each duplicate's copies, after Delete all duplicate
// downloads, which takes the downloads the library also has elsewhere. A
// opens a dialog on Cancel; Delete deletes and says so.
func TestLibraryPage(t *testing.T) {
	lib := dupeLibrary()
	var toasts []string
	s := New(Options{Prefs: prefs{}, Player: &fakePlayer{}, Library: lib, Toast: func(m string) { toasts = append(toasts, m) }})
	if !s.shown(rowLibrary) {
		t.Fatal("no Library row")
	}
	for s.focus != rowLibrary {
		s.Handle(press(input.Down), t0)
	}
	s.Handle(press(input.A), t0)
	pg := s.library
	if pg == nil {
		t.Fatal("A should open the Library page")
	}
	items := pg.items[dupesView]
	if len(items) != 5 || !items[0].all || items[0].row.Value != "2 songs" || pg.counts[dupesView] != "4" {
		t.Fatalf("duplicates: %d rows, first %+v, count %q", len(items), items[0].row, pg.counts[dupesView])
	}
	if items[1].row.Detail != "Odalys · Copper Season" || items[2].row.Detail != "Odalys · Ember · Downloaded" {
		t.Fatalf("copies %q, %q", items[1].row.Detail, items[2].row.Detail)
	}
	e := &env{prefs: prefs{}, p: &fakePlayer{}, s: s}
	uitest.Golden(t, "library-duplicates", render(t, e, t0.Add(anim.Focus)))

	// Delete all: the dialog starts on Cancel, so A there does nothing.
	s.Handle(press(input.A), t0)
	s.Handle(press(input.A), t0)
	render(t, e, t0.Add(time.Second))
	if len(lib.deleted) != 0 {
		t.Fatalf("A on Cancel deleted %v", lib.deleted)
	}
	s.Handle(press(input.A), t0.Add(time.Second))
	uitest.Golden(t, "library-delete-all", render(t, e, t0.Add(2*time.Second)))
	s.Handle(press(input.Left), t0.Add(2*time.Second))
	s.Handle(press(input.A), t0.Add(2*time.Second))
	want := []string{"/m/Boombrick/Odalys/Ember/01.opus", "/m/Boombrick/Kenji Arata/Neon/01.opus"}
	if len(lib.deleted) != 1 || !sameSet(lib.deleted[0], want) || !slices.Equal(toasts, []string{"Deleted 2 songs"}) {
		t.Fatalf("deleted %v, toasts %v", lib.deleted, toasts)
	}
	if len(pg.items[dupesView]) != 0 {
		t.Fatalf("after: %+v", pg.items[dupesView])
	}

	// Albums say which are duplicates; Songs list every song.
	render(t, e, t0.Add(3*time.Second))
	pg.gone = map[string]bool{}
	pg.read()
	s.Handle(press(input.R2), t0)
	if pg.view != albumsView || pg.items[albumsView][1].row.Value != "Duplicate" {
		t.Fatalf("albums %+v", pg.items[albumsView])
	}
	s.Handle(press(input.R2), t0)
	if pg.view != songsView || len(pg.items[songsView]) != 5 {
		t.Fatalf("songs %d", len(pg.items[songsView]))
	}
	s.Back()
	if s.library != nil {
		t.Fatal("B should close the page")
	}
}

func sameSet(a, b []string) bool {
	a, b = slices.Clone(a), slices.Clone(b)
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(a, b)
}

// X searches the view shown: what's typed finds rows by title, artist,
// album or folder, accents aside. Y goes to the matches, A on one asks to
// delete it, and B steps back out: matches, then typing, then the search.
func TestLibraryPageSearch(t *testing.T) {
	lib := dupeLibrary()
	s := New(Options{Prefs: prefs{}, Player: &fakePlayer{}, Library: lib})
	s.library = newLibraryPage(lib)
	pg := s.library
	s.Handle(press(input.R2), t0)
	s.Handle(press(input.R2), t0) // Songs
	s.Handle(press(input.X), t0)
	if !pg.searching {
		t.Fatal("X should open the search")
	}
	for _, r := range "ÓDALYS emb" {
		pg.typeRune(r)
	}
	if len(pg.hits) != 2 {
		t.Fatalf("songs for %q: %v", pg.query, pg.hits)
	}
	e := &env{prefs: prefs{}, p: &fakePlayer{}, s: s}
	uitest.Golden(t, "library-search", render(t, e, t0.Add(anim.Focus)))
	s.Handle(press(input.L2), t0) // Albums: one matches
	if pg.view != albumsView || len(pg.hits) != 1 || pg.items[albumsView][pg.hits[0]].row.Label != "Ember" {
		t.Fatalf("albums %v", pg.hits)
	}
	s.Handle(press(input.Y), t0)
	s.Handle(press(input.A), t0)
	if !pg.onHits || pg.dialog == nil || pg.doomed[0].Path != "/m/Boombrick/Odalys/Ember/01.opus" {
		t.Fatalf("on hits %v, dialog %v, doomed %v", pg.onHits, pg.dialog != nil, pg.doomed)
	}
	s.Back() // closes the dialog
	if pg.dialog == nil || !pg.dialog.closing {
		t.Fatal("B should close the dialog")
	}
	pg.dialog = nil // Back closes it on the wall clock, which the test's frames don't follow
	s.Back() // to the keyboard
	if pg.onHits || !pg.searching {
		t.Fatal("B on the matches goes to the keyboard")
	}
	for pg.query != "" {
		s.Back()
	}
	s.Back()
	if pg.searching || s.library == nil {
		t.Fatal("B with nothing typed closes the search, not the page")
	}
}
