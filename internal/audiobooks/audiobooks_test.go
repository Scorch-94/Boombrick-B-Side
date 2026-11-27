package audiobooks

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ujjwal313/boombrick/internal/player"
	"github.com/ujjwal313/boombrick/internal/store"
)

func touch(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, nil, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestScanFindsFilesAndFolders(t *testing.T) {
	d := t.TempDir()
	touch(t, filepath.Join(d, "Dune.m4b"))
	touch(t, filepath.Join(d, "Emma", "Part 02.m4a"))
	touch(t, filepath.Join(d, "Emma", "Part 01.m4a"))
	touch(t, filepath.Join(d, "Emma", "cover.jpg"))
	touch(t, filepath.Join(d, "notes.txt"))
	touch(t, filepath.Join(d, ".hidden.m4b"))
	os.MkdirAll(filepath.Join(d, "Empty"), 0o755)

	books := Scan(d)
	if len(books) != 2 {
		t.Fatalf("books = %+v, want Dune and Emma", books)
	}
	if books[0].Title != "Dune" || len(books[0].Parts) != 1 || books[0].Path != filepath.Join(d, "Dune.m4b") {
		t.Errorf("first = %+v", books[0])
	}
	emma := books[1]
	if emma.Title != "Emma" || emma.Path != filepath.Join(d, "Emma") || len(emma.Parts) != 2 ||
		emma.Parts[0].Title != "Part 01" || emma.Parts[1].Title != "Part 02" {
		t.Errorf("second = %+v", emma)
	}
	if ts := emma.Tracks(); len(ts) != 2 || ts[0].Album != "Emma" || ts[1].Path != emma.Parts[1].Path {
		t.Errorf("tracks = %+v", ts)
	}
}

func TestBookOf(t *testing.T) {
	d := t.TempDir()
	SetDir(d)
	t.Cleanup(func() { SetDir(filepath.Join(t.TempDir(), "none")) })
	cases := map[string]string{
		filepath.Join(d, "Dune.m4b"):                 filepath.Join(d, "Dune.m4b"),
		filepath.Join(d, "Emma", "CD1", "01.m4a"):    filepath.Join(d, "Emma"),
		filepath.Join(filepath.Dir(d), "Music", "x"): "",
		"": "",
	}
	for p, want := range cases {
		if got := BookOf(p); got != want {
			t.Errorf("BookOf(%q) = %q, want %q", p, got, want)
		}
		if IsBook(p) != (want != "") {
			t.Errorf("IsBook(%q) = %v", p, IsBook(p))
		}
	}
}

func book() Book {
	return Book{Path: "/b", Title: "B", Duration: 3 * time.Hour, Parts: []Part{
		{Path: "/b/1", Duration: time.Hour}, {Path: "/b/2", Duration: 2 * time.Hour}}}
}

func TestLeftAndStart(t *testing.T) {
	b := book()
	pl := Place{Part: "/b/2", Position: 30 * time.Minute}
	if got := b.Left(pl); got != 90*time.Minute {
		t.Errorf("Left = %v, want 1h30m", got)
	}
	if i, at := b.Start(pl); i != 1 || at != 30*time.Minute-rewind {
		t.Errorf("Start = %d, %v", i, at)
	}
	if i, at := b.Start(Place{Part: "/b/1", Position: 2 * time.Second}); i != 0 || at != 0 {
		t.Errorf("Start near the beginning = %d, %v, want 0, 0", i, at)
	}
	if i, at := b.Start(Place{Part: "/b/2", Finished: true}); i != 0 || at != 0 {
		t.Errorf("Start finished = %d, %v", i, at)
	}
	if i, _ := b.Start(Place{Part: "/gone"}); i != 0 {
		t.Errorf("Start of a part gone = %d", i)
	}
	if b.Left(Place{}) != b.Duration || b.Left(Place{Finished: true}) != 0 {
		t.Error("Left of a book not started or finished")
	}
}

func places(t *testing.T) *Places {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "boombrick.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return NewPlaces(st.DB)
}

func TestPlaces(t *testing.T) {
	ps := places(t)
	at := time.UnixMilli(1700000000000)
	if err := ps.Save("/b", Place{Part: "/b/1", Position: 90 * time.Second, Updated: at}); err != nil {
		t.Fatal(err)
	}
	ps.Save("/b", Place{Part: "/b/2", Position: 5 * time.Second, Updated: at})
	ps.Save("/c", Place{Part: "/c", Finished: true, Updated: at})
	all, err := ps.All()
	if err != nil {
		t.Fatal(err)
	}
	if got := all["/b"]; got.Part != "/b/2" || got.Position != 5*time.Second || !got.Updated.Equal(at) || got.Finished {
		t.Errorf("/b = %+v", got)
	}
	if !all["/c"].Finished {
		t.Errorf("/c = %+v", all["/c"])
	}
	ps.Forget("/b")
	if all, _ = ps.All(); len(all) != 1 {
		t.Errorf("after Forget = %+v", all)
	}
}

type saves struct{ got []Place }

func (s *saves) Save(_ string, pl Place) error { s.got = append(s.got, pl); return nil }

func TestFollowKeepsThePlace(t *testing.T) {
	d := t.TempDir()
	SetDir(d)
	part := filepath.Join(d, "Dune.m4b")
	st := player.State{Queued: true, Playing: true, Track: player.Track{Path: part}, Count: 1, Duration: time.Hour}
	s := &saves{}
	now := time.Unix(1700000000, 0)
	f := &follower{state: func() player.State { return st }, saver: s, now: func() time.Time { return now }}

	st.Elapsed = time.Minute
	f.look() // a book comes on: kept
	now = now.Add(5 * time.Second)
	st.Elapsed += 5 * time.Second
	f.look() // too soon to keep again
	if len(s.got) != 1 {
		t.Fatalf("saves = %+v", s.got)
	}
	now = now.Add(saveEvery)
	st.Elapsed += saveEvery
	f.look()
	st.Playing = false
	f.look() // paused: kept at once
	if len(s.got) != 3 || s.got[2].Position != st.Elapsed || s.got[2].Finished {
		t.Fatalf("saves = %+v", s.got)
	}

	// Heard to the end: finished.
	st.Playing, st.Elapsed = true, time.Hour-time.Second
	f.look()
	st.Playing = false
	f.look()
	if last := s.got[len(s.got)-1]; !last.Finished {
		t.Errorf("at the end: %+v", last)
	}

	// A song comes on: nothing more is kept for it.
	n := len(s.got)
	st.Track, st.Playing = player.Track{Path: "/Music/song.flac"}, true
	f.look()
	f.look()
	if len(s.got) != n+1 { // the book, left
		t.Errorf("after leaving: %+v", s.got[n:])
	}
}

func TestFollowKeepsOnQuit(t *testing.T) {
	d := t.TempDir()
	SetDir(d)
	st := player.State{Queued: true, Playing: true, Track: player.Track{Path: filepath.Join(d, "x.m4b")},
		Count: 1, Duration: time.Hour, Elapsed: 10 * time.Minute}
	s := &saves{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { Follow(ctx, func() player.State { return st }, s, nil, nil); close(done) }()
	time.Sleep(1500 * time.Millisecond)
	cancel()
	<-done
	if len(s.got) < 2 || s.got[len(s.got)-1].Position != 10*time.Minute {
		t.Errorf("saves = %+v", s.got)
	}
}
