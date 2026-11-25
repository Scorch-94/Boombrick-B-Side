package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/ujjwal313/boombrick/internal/player"
)

type savedPlaces struct {
	mu   sync.Mutex
	sent []string
}

func (s *savedPlaces) SavePosition(_ context.Context, id string, ms int64, finished bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sent = append(s.sent, fmt.Sprintf("%s %d %t", id, ms/1000, finished))
	return nil
}

func (s *savedPlaces) list() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.sent)
}

// The place in an episode goes to Spotify every 30 s while it plays, when
// it pauses, and when another track takes over: as played through once it
// was near its end. Songs send nothing.
func TestEpisodeFollower(t *testing.T) {
	var st player.State
	now := time.Date(2026, 11, 25, 9, 0, 0, 0, time.UTC)
	saver := &savedPlaces{}
	stale := 0
	f := &episodeFollower{state: func() player.State { return st }, saver: saver, saved: func() { stale++ },
		log: slog.New(slog.NewTextHandler(io.Discard, nil)), now: func() time.Time { return now }}
	look := func(d time.Duration, elapsed time.Duration, playing bool) {
		now = now.Add(d)
		st.Elapsed, st.Playing = elapsed, playing
		f.look()
		f.wg.Wait()
	}
	st.Track = player.Track{SpotifyID: "episode:ep1", Title: "Ep 1"}
	st.Duration = 40 * time.Minute
	look(0, 10*time.Minute, true) // starts where it was left: nothing new
	look(10*time.Second, 10*time.Minute+10*time.Second, true)
	look(25*time.Second, 10*time.Minute+35*time.Second, true) // 35 s on: sent
	look(2*time.Second, 10*time.Minute+37*time.Second, false) // paused 2 s later: moved too little
	look(time.Minute, 10*time.Minute+37*time.Second, false)
	look(time.Second, 11*time.Minute, true)                 // playing again, over 30 s since the last: sent
	look(time.Second, 11*time.Minute+10*time.Second, false) // paused, moved 10 s: sent
	look(time.Second, 39*time.Minute+45*time.Second, true)  // skipped near the end
	st.Track = player.Track{Path: "/m/song.flac"}           // a song takes over
	look(time.Second, 0, true)
	look(time.Minute, time.Minute, true)
	want := []string{"ep1 635 false", "ep1 660 false", "ep1 670 false", "ep1 2385 true"}
	if got := saver.list(); !slices.Equal(got, want) {
		t.Fatalf("sent %v, want %v", got, want)
	}
	if stale != 4 {
		t.Fatalf("Continue listening made stale %d times", stale)
	}
	st.Track = player.Track{SpotifyID: "episode:ep2"}
	look(time.Second, 5*time.Minute, true)
	look(time.Second, 6*time.Minute, true)
	f.leave() // quitting part way
	f.wg.Wait()
	if got := saver.list(); got[len(got)-1] != "ep2 360 false" {
		t.Fatalf("quitting sent %v", got)
	}
	st.Track = player.Track{SpotifyID: "4uLU6hMCjMI75M1A2tKUQC"} // a Spotify song
	look(time.Minute, time.Minute, true)
	look(time.Minute, 2*time.Minute, false)
	if n := len(saver.list()); n != 5 {
		t.Fatalf("a song sent places: %v", saver.list())
	}
}
