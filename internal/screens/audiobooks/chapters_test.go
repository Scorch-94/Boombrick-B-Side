package audiobooks

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ujjwal313/boombrick/internal/audiobooks"
	"github.com/ujjwal313/boombrick/internal/platform/input"
	"github.com/ujjwal313/boombrick/internal/player"
)

type fakePlayer struct {
	st     player.State
	seeks  []time.Duration
	played []string // "part@time"
}

func (f *fakePlayer) PlayAt(ts []player.Track, start int, at time.Duration) {
	f.played = append(f.played, filepath.Base(ts[start].Path)+"@"+at.String())
	f.st.Track, f.st.Elapsed, f.st.Queued, f.st.Playing = ts[start], at, true, true
}
func (f *fakePlayer) SetShuffle(bool)      {}
func (f *fakePlayer) State() player.State  { return f.st }
func (f *fakePlayer) Seek(d time.Duration) { f.seeks = append(f.seeks, d); f.st.Elapsed = d }
func (f *fakePlayer) Toggle()              { f.st.Playing = !f.st.Playing }

// twoParts scans a book of two files, which list no chapters: a chapter
// each.
func twoParts(t *testing.T) (audiobooks.Book, *fakePlayer) {
	d := t.TempDir()
	audiobooks.SetDir(d)
	for _, n := range []string{"1.m4a", "2.m4a"} {
		p := filepath.Join(d, "Book", n)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, nil, 0o644)
	}
	books := audiobooks.Scan(d)
	if len(books) != 1 || len(books[0].Chapters) != 2 {
		t.Fatalf("books %+v", books)
	}
	return books[0], &fakePlayer{}
}

func TestChapterSteps(t *testing.T) {
	b, p := twoParts(t)
	c := NewChapters(p, nil)
	p.st = player.State{Queued: true, Playing: true, Track: b.Tracks()[0], Elapsed: 30 * time.Second}
	ch, ok := c.At(p.st)
	if !ok || ch.Title != "1" || ch.Line != "Chapter 1 of 2" || ch.Book != "Book" {
		t.Fatalf("At = %+v, %v", ch, ok)
	}
	c.Step(p.st, 1) // into the next part: plays it
	if len(p.played) != 1 || p.played[0] != "2.m4a@0s" {
		t.Fatalf("played %v", p.played)
	}
	if ch, _ := c.At(p.st); ch.Line != "Chapter 2 of 2" {
		t.Errorf("At after = %+v", ch)
	}
	c.Step(p.st, 1) // the last: stays
	p.st.Elapsed = 2 * time.Second
	c.Step(p.st, -1) // just begun: the chapter before
	if len(p.played) != 2 || p.played[1] != "1.m4a@0s" {
		t.Fatalf("played %v", p.played)
	}
	p.st.Elapsed, p.st.Playing = 10*time.Second, false
	c.Step(p.st, -1) // well into it: its start, by seeking, and playing
	if len(p.seeks) != 1 || p.seeks[0] != 0 || !p.st.Playing {
		t.Fatalf("seeks %v, playing %v", p.seeks, p.st.Playing)
	}
	p.st.Track = player.Track{Path: "/Music/song.flac"}
	if c.Step(p.st, 1) || c.List(p.st) {
		t.Error("stepped or listed in a song")
	}
}

func TestChapterPagePlays(t *testing.T) {
	b, p := twoParts(t)
	p.st = player.State{Queued: true, Playing: true, Track: b.Tracks()[0]}
	pg := NewChapterPage(p, b, 1)
	if pg.list.Focus() != 1 || pg.count != "2 chapters" {
		t.Fatalf("focus %d, %q", pg.list.Focus(), pg.count)
	}
	pg.Handle(input.Event{Kind: input.Press, Button: input.A}, time.Now())
	if len(p.played) != 1 || p.played[0] != "2.m4a@0s" {
		t.Errorf("played %v", p.played)
	}
	if clock(3723*time.Second) != "1:02:03" || clock(65*time.Second) != "1:05" {
		t.Error(clock(3723*time.Second), clock(65*time.Second))
	}
}
