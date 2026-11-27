package library

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func names(lists []*Playlist) []string {
	var out []string
	for _, pl := range lists {
		out = append(out, pl.Name)
	}
	return out
}

func byName(t *testing.T, lists []*Playlist, name string) *Playlist {
	t.Helper()
	for _, pl := range lists {
		if pl.Name == name {
			return pl
		}
	}
	t.Fatalf("no playlist %q in %v", name, names(lists))
	return nil
}

func entryTitles(pl *Playlist) []string {
	var out []string
	for _, e := range pl.Entries {
		if e.Song != nil {
			out = append(out, e.Song.Title)
		} else {
			out = append(out, "spotify:"+e.Stream.Title)
		}
	}
	return out
}

// A playlist made on the device holds library songs and Spotify ones, in
// the order given, and reads back the same.
func TestNewPlaylistSavesAndReadsBack(t *testing.T) {
	music := t.TempDir()
	ix := csvIndex(music)
	pl, err := NewPlaylist(music, `Road/trip: "best"`)
	if err != nil {
		t.Fatal(err)
	}
	if pl.Name != "Road trip best" || filepath.Ext(pl.Path) != ".m3u8" {
		t.Fatalf("made %q at %s", pl.Name, pl.Path)
	}
	if again, _ := NewPlaylist(music, "road trip best"); again.Name != "road trip best 2" {
		t.Fatalf("a taken name made %q", again.Name)
	}
	if _, err := NewPlaylist(music, " / "); err != ErrPlaylistName {
		t.Fatalf("an empty name: %v", err)
	}
	pl.SetEntries([]PlaylistEntry{{Song: ix.Songs[0]},
		{Stream: &StreamSong{SpotifyID: "65u6c5NBv75ynFeJG4XA5Z", Title: "Stickerbush Symphony", Artist: "GameChops", Duration: 3 * time.Minute}},
		{Song: ix.Songs[2]}})
	if err := pl.Save(music); err != nil {
		t.Fatal(err)
	}
	back := byName(t, ReadPlaylists(music, ix), "Road trip best")
	if want := []string{ix.Songs[0].Title, "spotify:Stickerbush Symphony", ix.Songs[2].Title}; !slices.Equal(entryTitles(back), want) {
		t.Fatalf("read back %v, want %v", entryTitles(back), want)
	}
	if s := back.Entries[1].Stream; s.Artist != "GameChops" || s.Duration != 3*time.Minute || back.Streams != 1 {
		t.Fatalf("stream %+v", s)
	}
	// A Spotify song the import downloaded plays from its file.
	back.SetEntries([]PlaylistEntry{{Stream: &StreamSong{SpotifyID: "4m4Rh5mdcYDspyCEncdtPX", Title: "Aquatic Ambiance"}}})
	back.Save(music)
	if again := byName(t, ReadPlaylists(music, ix), "Road trip best"); len(again.Songs) != 1 || again.Streams != 0 {
		t.Fatalf("downloaded song: %+v", again.Entries)
	}
}

// Saving keeps lines that weren't found, and renaming and deleting move and
// remove the file.
func TestEditM3U(t *testing.T) {
	music := t.TempDir()
	ix := csvIndex(music)
	dir := filepath.Join(music, PlaylistDir)
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "Mix.m3u"), []byte("../ff/1.flac\nGone/away.flac\n../sc/1.flac\n"), 0o644)
	pl := byName(t, ReadPlaylists(music, ix), "Mix")
	es := slices.Clone(pl.Entries)
	slices.Reverse(es)
	pl.SetEntries(es)
	if err := pl.Save(music); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(pl.Path)
	if !strings.Contains(string(data), "Gone/away.flac") {
		t.Fatalf("the line not found went:\n%s", data)
	}
	if err := pl.Rename(music, "Evening"); err != nil {
		t.Fatal(err)
	}
	lists := ReadPlaylists(music, ix)
	ev := byName(t, lists, "Evening")
	if ev.Missing != 1 || !slices.Equal(entryTitles(ev), []string{"aquatic ambience", "Aquatic Ambience"}) {
		t.Fatalf("after: %v, %d missing", entryTitles(ev), ev.Missing)
	}
	if err := ev.Delete(music); err != nil {
		t.Fatal(err)
	}
	if lists := ReadPlaylists(music, ix); len(lists) != 0 {
		t.Fatalf("after deleting: %v", names(lists))
	}
}

// In a .csv holding several playlists, editing one leaves the others be.
func TestEditCSVWithOthers(t *testing.T) {
	music := t.TempDir()
	ix := csvIndex(music)
	dir := filepath.Join(music, PlaylistDir)
	os.MkdirAll(dir, 0o755)
	text := "Track name,Artist name,Album,Playlist name,Spotify - id\n" +
		"Aquatic Ambience,Funk Fiction,Aquatic Ambience,Water,3zF1RvARE71MBXeWETPoEk\n" +
		"aquatic ambience,Scizzie,aquatic ambience,Lofi,1AeAJE0BNsq9jyssQDsUZ9\n" +
		"Nothing Like It,Nobody,Nowhere,Water,\n" +
		"Aquatic Ambience,Helynt,Super Ambient World,Water,41dF02sXROxmP1ld8KmVPp\n"
	os.WriteFile(filepath.Join(dir, "Library.csv"), []byte(text), 0o644)
	water := byName(t, ReadPlaylists(music, ix), "Water")
	if len(water.Entries) != 2 || water.Missing != 1 {
		t.Fatalf("water %v, %d missing", entryTitles(water), water.Missing)
	}
	water.SetEntries(water.Entries[1:]) // drop Funk Fiction's
	if err := water.Save(music); err != nil {
		t.Fatal(err)
	}
	if err := water.Rename(music, "Sea"); err != nil {
		t.Fatal(err)
	}
	lists := ReadPlaylists(music, ix)
	if got := names(lists); !slices.Equal(got, []string{"Lofi", "Sea"}) {
		t.Fatalf("playlists %v", got)
	}
	sea := byName(t, lists, "Sea")
	if len(sea.Songs) != 1 || sea.Songs[0].Path != filepath.Join(music, "ha/1.flac") || sea.Missing != 1 {
		t.Fatalf("sea %v, %d missing", entryTitles(sea), sea.Missing)
	}
	if err := sea.Delete(music); err != nil {
		t.Fatal(err)
	}
	lists = ReadPlaylists(music, ix)
	if got := names(lists); !slices.Equal(got, []string{"Lofi"}) {
		t.Fatalf("after deleting Sea: %v", got)
	}
	if err := lists[0].Delete(music); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "Library.csv")); !os.IsNotExist(err) {
		t.Fatal("the emptied file stayed")
	}
}
