package app

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/ujjwal313/boombrick/internal/history"
	"github.com/ujjwal313/boombrick/internal/library"
	"github.com/ujjwal313/boombrick/internal/player"
)

// Deleting a song takes its file and lyrics, and the folders it leaves empty
// with their covers, up to the music folder; a folder with other songs stays.
// A download is forgotten by the queue, and the library scanned again.
func TestDeleteSongs(t *testing.T) {
	music := t.TempDir()
	write := func(rel string) string {
		p := filepath.Join(music, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	dl := write("Boombrick/Odalys/Ember/01 Ember.opus")
	write("Boombrick/Odalys/Ember/01 Ember.lrc")
	write("Boombrick/Odalys/Ember/cover.jpg")
	other := write("Boombrick/Odalys/Rust/01 Rust.opus")
	mine := write("Odalys/Copper Season/01 Ember.flac")
	keep := write("Odalys/Copper Season/02 Rust.flac")

	q := newLiveQueue()
	forgot := []string{}
	scans := 0
	a := &App{opts: Options{MusicDir: music, Downloads: forgetting{q, &forgot}, Rescan: func() { scans++ }}}
	m := libraryManager{a}
	if !m.Downloaded(&library.Track{Path: dl}) || m.Downloaded(&library.Track{Path: mine}) {
		t.Fatal("Downloaded")
	}
	n := m.Delete([]*library.Track{{Path: dl}, {Path: mine}, {Path: filepath.Join(music, "gone.flac")}})
	if n != 3 || scans != 1 || len(forgot) != 3 || forgot[0] != dl {
		t.Fatalf("deleted %d, scans %d, forgot %v", n, scans, forgot)
	}
	for _, p := range []string{dl, mine, filepath.Join(music, "Boombrick/Odalys/Ember")} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("%s is still there", p)
		}
	}
	for _, p := range []string{other, keep, filepath.Join(music, "Boombrick")} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%s went: %v", p, err)
		}
	}
	if _, err := os.Stat(music); err != nil {
		t.Fatal("the music folder went")
	}
}

// forgetting is a queue that records the files it is told to forget.
type forgetting struct {
	*liveQueue
	forgot *[]string
}

func (f forgetting) ForgetFile(p string) { *f.forgot = append(*f.forgot, p) }

// albumRecorder records what Home's albums ask the player for.
type albumRecorder struct {
	tracks []player.Track
	start  int
}

func (r *albumRecorder) SetShuffle(bool) {}
func (r *albumRecorder) PlayFrom(ts []player.Track, start int, _ int64) {
	r.tracks, r.start = ts, start
}

// An album started from Home, as Random albums starts one, plays all its
// songs from the first.
func TestHomePlaysWholeAlbums(t *testing.T) {
	al := &library.Album{Title: "Copper Season", Artist: "Odalys", Tracks: []library.Track{
		{Path: "/m/1.flac"}, {Path: "/m/2.flac"}, {Path: "/m/3.flac"}}}
	r := &albumRecorder{}
	playAlbum(r, al, 0, 0)
	if len(r.tracks) != 3 || r.start != 0 || r.tracks[2].Path != "/m/3.flac" {
		t.Fatalf("played %+v from %d", r.tracks, r.start)
	}
}

type fakeHistory struct{ plays []history.Play }

func (h fakeHistory) Recent(n int) ([]history.Play, error) {
	return h.plays[:min(n, len(h.plays))], nil
}
func (h fakeHistory) Most(n int) ([]history.Play, error) { return h.plays[:min(n, len(h.plays))], nil }

// Recently played shows the songs still in the library, and Spotify's only
// with Spotify on.
func TestPlayedRow(t *testing.T) {
	ix := library.NewIndex([]library.Album{{Title: "Copper Season", Artist: "Odalys", Tracks: []library.Track{
		{Path: "/m/1.flac", Title: "Rust", Artist: "Odalys"}}}})
	h := fakeHistory{plays: []history.Play{
		{Track: player.Track{Path: "/m/gone.flac", Title: "Gone"}},
		{Track: player.Track{SpotifyID: "4uLU6hMCjMI75M1A2tKUQC", Title: "Never"}},
		{Track: player.Track{Path: "/m/1.flac", Title: "old title"}, Count: 3},
	}}
	a := &App{ix: ix, opts: Options{History: h, Log: slog.New(slog.DiscardHandler)}}
	row, ok := a.playedRow("Recently played", "", false)
	if !ok || len(row.Items) != 1 || row.Items[0].Title != "Rust" {
		t.Fatalf("row %+v", row)
	}
	if _, ok := (&App{ix: ix, opts: Options{Log: slog.New(slog.DiscardHandler)}}).playedRow("x", "", true); ok {
		t.Fatal("a row with no history")
	}
}
