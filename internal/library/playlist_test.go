package library

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Entries resolve relative to the playlist, relative to the music folder,
// absolute, written on Windows or another device, or in another case; the
// rest count as missing, and comments are skipped.
func TestReadPlaylists(t *testing.T) {
	music := t.TempDir()
	track := func(rel string, d time.Duration) Track {
		return Track{Path: filepath.Join(music, rel), Title: filepath.Base(rel), Duration: d}
	}
	ix := NewIndex([]Album{{Artist: "Odalys", Title: "Copper Season", Tracks: []Track{
		track("Odalys/Copper Season/01 Rust.flac", time.Minute),
		track("Odalys/Copper Season/02 Ember.flac", 2*time.Minute),
		track("Odalys/Copper Season/03 Tin Roof.mp3", 3*time.Minute),
		track("Odalys/Copper Season/04 Low Tide.opus", 4*time.Minute),
		track("Odalys/Copper Season/05 Salt.m4a", 5*time.Minute),
	}}})
	dir := filepath.Join(music, PlaylistDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("Road Trip.m3u", "\xef\xbb\xbf#EXTM3U\n#EXTINF:222,Odalys - Rust\n../Odalys/Copper Season/01 Rust.flac\n\n"+
		"Odalys/Copper Season/02 Ember.flac\r\n"+
		filepath.Join(music, "Odalys/Copper Season/03 Tin Roof.mp3")+"\n"+
		`C:\Users\me\Music\Odalys\Copper Season\04 Low Tide.opus`+"\n"+
		"/storage/sdcard1/Music/odalys/copper season/05 salt.m4a\n"+
		"Nobody/Gone.flac\n")
	write("A Side.m3u8", "file://"+filepath.Join(music, "Odalys/Copper Season/05 Salt.m4a")+"\n")
	write("notes.txt", "Odalys/Copper Season/01 Rust.flac\n")
	write(".hidden.m3u", "Odalys/Copper Season/01 Rust.flac\n")

	lists := ReadPlaylists(music, ix)
	if len(lists) != 2 || lists[0].Name != "A Side" || lists[1].Name != "Road Trip" {
		t.Fatalf("playlists %+v", lists)
	}
	rt := lists[1]
	var titles []string
	for _, s := range rt.Songs {
		titles = append(titles, s.Title)
	}
	want := []string{"01 Rust.flac", "02 Ember.flac", "03 Tin Roof.mp3", "04 Low Tide.opus", "05 Salt.m4a"}
	if len(titles) != len(want) {
		t.Fatalf("songs %q, want %q", titles, want)
	}
	for i := range want {
		if titles[i] != want[i] {
			t.Fatalf("songs %q, want %q", titles, want)
		}
	}
	if rt.Missing != 1 || rt.Duration != 15*time.Minute {
		t.Fatalf("missing %d, duration %v", rt.Missing, rt.Duration)
	}
	if len(lists[0].Songs) != 1 {
		t.Fatalf("A Side has %d songs", len(lists[0].Songs))
	}
}

func TestReadPlaylistsWithoutTheFolder(t *testing.T) {
	if lists := ReadPlaylists(t.TempDir(), NewIndex(nil)); lists != nil {
		t.Fatalf("playlists %+v without the folder", lists)
	}
}
