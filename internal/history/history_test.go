package history

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/ujjwal313/boombrick/internal/player"
	"github.com/ujjwal313/boombrick/internal/store"
)

func open(t *testing.T) *History {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "boombrick.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return New(st.DB)
}

func TestRecentAndMost(t *testing.T) {
	h := open(t)
	at := time.UnixMilli(1700000000000)
	rust := player.Track{Path: "/m/rust.flac", Title: "Rust", Artist: "Odalys", Album: "Copper Season", Duration: 3 * time.Minute}
	never := player.Track{SpotifyID: "4uLU6hMCjMI75M1A2tKUQC", Title: "Never", Artist: "Rick", Cover: "https://i.scdn.co/x"}
	ember := player.Track{Path: "/m/ember.flac", Title: "Ember"}
	h.Add(rust, at)
	h.Add(never, at.Add(time.Minute))
	h.Add(rust, at.Add(2*time.Minute))
	h.Add(ember, at.Add(3*time.Minute))
	recent, err := h.Recent(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(recent) != 3 || recent[0].Track.Title != "Ember" || recent[1].Track.Title != "Rust" || recent[2].Track.SpotifyID != never.SpotifyID {
		t.Fatalf("recent %+v", recent)
	}
	most, _ := h.Most(2)
	if len(most) != 2 || most[0].Track.Path != rust.Path || most[0].Count != 2 || most[0].Track.Duration != 3*time.Minute ||
		most[1].Track.Title != "Ember" {
		t.Fatalf("most %+v", most)
	}
	if recent[2].Track.Cover != never.Cover || recent[2].Track.Path != "" {
		t.Fatalf("a stream read back as %+v", recent[2].Track)
	}
	h.Forget(rust.Path)
	if most, _ := h.Most(10); len(most) != 2 {
		t.Fatalf("after forgetting: %+v", most)
	}
}

func TestCounts(t *testing.T) {
	book := func(t player.Track) bool { return t.Path == "/Audiobooks/x.m4b" }
	for _, c := range []struct {
		t    player.Track
		want bool
	}{
		{player.Track{Path: "/m/a.flac"}, true},
		{player.Track{SpotifyID: "4uLU6hMCjMI75M1A2tKUQC"}, true},
		{player.Track{SpotifyID: "episode:abc"}, false},
		{player.Track{Stream: "http://radio"}, false},
		{player.Track{VideoID: "abc"}, false},
		{player.Track{Path: "/Audiobooks/x.m4b"}, false},
	} {
		if got := Counts(c.t, book); got != c.want {
			t.Errorf("Counts(%+v) = %v", c.t, got)
		}
	}
}

// A song counts once heard for half its length or 30 s; skipping ahead
// isn't hearing, and playing it again counts again.
func TestCounter(t *testing.T) {
	var c counter
	s := player.State{Queued: true, Playing: true, Track: player.Track{Path: "/m/a.flac"}, Duration: 40 * time.Second}
	counts := 0
	step := func(by time.Duration) {
		s.Elapsed += by
		if c.look(s, nil) {
			counts++
		}
	}
	for range 19 {
		step(time.Second)
	}
	if counts != 0 {
		t.Fatal("counted before half")
	}
	step(time.Second)
	if counts != 1 {
		t.Fatal("not counted at half of a 40 s song")
	}
	step(time.Second)
	if counts != 1 {
		t.Fatal("counted twice")
	}
	s.Elapsed = 0 // on repeat
	for range 20 {
		step(time.Second)
	}
	if counts != 2 {
		t.Fatalf("a replay counted %d", counts)
	}
	// A long song skipped through counts only what was heard.
	s.Track, s.Elapsed, s.Duration, s.Index = player.Track{Path: "/m/b.flac"}, 0, 10*time.Minute, 1
	c.look(s, nil)
	step(5 * time.Minute)
	for range 29 {
		step(time.Second)
	}
	if counts != 2 {
		t.Fatal("a skip ahead counted as heard")
	}
	s.Playing = false
	for range 5 {
		c.look(s, nil)
	}
	s.Playing = true
	step(time.Second)
	if counts != 3 {
		t.Fatal("30 s of a long song didn't count")
	}
}
