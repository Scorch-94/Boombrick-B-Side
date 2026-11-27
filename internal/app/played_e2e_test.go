package app

import (
	"context"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/ujjwal313/boombrick/internal/history"
	"github.com/ujjwal313/boombrick/internal/library"
	"github.com/ujjwal313/boombrick/internal/player"
	"github.com/ujjwal313/boombrick/internal/store"
)

// A song heard through Watch, kept in the real table, comes out in the row.
func TestPlayedRowFromWatch(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "boombrick.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	h := history.New(st.DB)
	ix := library.NewIndex([]library.Album{{Title: "Copper Season", Artist: "Odalys", Tracks: []library.Track{
		{Path: "/m/1.flac", Title: "Rust", Artist: "Odalys", Duration: 4 * time.Second}}}})
	s := player.State{Queued: true, Playing: true, Track: player.Track{Path: "/m/1.flac", Title: "Rust"}, Duration: 4 * time.Second}
	start := time.Now()
	state := func() player.State {
		s := s
		s.Elapsed = time.Since(start).Truncate(time.Second)
		return s
	}
	added := make(chan struct{}, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go history.Watch(ctx, state, h, nil, func() { added <- struct{}{} }, nil)
	select {
	case <-added:
	case <-time.After(6 * time.Second):
		t.Fatal("a 4 s song heard through wasn't counted")
	}
	a := &App{ix: ix, opts: Options{History: h, Log: slog.New(slog.DiscardHandler)}}
	for _, most := range []bool{false, true} {
		if row, ok := a.playedRow("x", "", most); !ok || len(row.Items) != 1 || row.Items[0].Title != "Rust" {
			t.Fatalf("most %v: row %+v", most, row)
		}
	}
}
