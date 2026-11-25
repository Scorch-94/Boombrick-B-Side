package library

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
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

// A .csv playlist, as Exportify writes one, matches by the Spotify track an
// import downloaded, then by title and artists, telling versions apart by
// album and length, and by a title's bare form when only one song fits.
func TestReadPlaylistsCSV(t *testing.T) {
	music := t.TempDir()
	tr := func(rel, title, artist, album string, d time.Duration, spotifyID string) Track {
		return Track{Path: filepath.Join(music, rel), Title: title, Artist: artist, Album: album, Duration: d, SpotifyID: spotifyID}
	}
	ix := NewIndex([]Album{
		{Artist: "Funk Fiction", Title: "Aquatic Ambience", Tracks: []Track{
			tr("ff/1.flac", "Aquatic Ambience", "Funk Fiction", "Aquatic Ambience", 194*time.Second, "")}},
		{Artist: "Scizzie", Title: "aquatic ambience", Tracks: []Track{
			tr("sc/1.flac", "aquatic ambience", "Scizzie", "aquatic ambience", 123*time.Second, ""),
			tr("sc/2.flac", "aquatic ambience - slowed + reverb", "Scizzie", "aquatic ambience (Remixes)", 154*time.Second, "")}},
		{Artist: "Helynt", Title: "Super Lofi World", Tracks: []Track{
			tr("hl/1.flac", "Aquatic Ambience", "Helynt, GameChops", "Super Lofi World", 165*time.Second, "")}},
		{Artist: "Helynt", Title: "Super Ambient World", Tracks: []Track{
			tr("ha/1.flac", "Aquatic Ambience", "Helynt", "Super Ambient World", 216*time.Second, "")}},
		{Artist: "Smooth McGroove", Title: "Remixed 2", Tracks: []Track{
			tr("sm/1.opus", "Aquatic Ambience", "Smooth McGroove", "Remixed 2", 217*time.Second, "")}},
		{Artist: "Koreskape", Title: "Aquatic Ambiance", Tracks: []Track{
			tr("Boombrick/ko.opus", "Whatever the tags say", "Someone", "Something", 193*time.Second, "4m4Rh5mdcYDspyCEncdtPX")}},
	})
	dir := filepath.Join(music, PlaylistDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	csvText := "\ufeffTrack URI,ISRC,Track Name,Album Name,Artist Name(s),Release Date,Duration (ms),Popularity\n" +
		`spotify:track:3zF1RvARE71MBXeWETPoEk,QZES91874867,"Aquatic Ambience","Aquatic Ambience","Funk Fiction",2019-01-15,194000,43` + "\n" +
		`spotify:track:65u6c5NBv75ynFeJG4XA5Z,QZ22B1739607,"Aquatic Ambience (From ""Donkey Kong Country"")","Smooth McGroove Remixed 2","GameChops;Smooth McGroove;Holder",2017-01-13,217612,21` + "\n" +
		`spotify:track:4m4Rh5mdcYDspyCEncdtPX,QZK6Q2003901,"Aquatic Ambiance","Aquatic Ambiance","Koreskape;GameChops",2020-08-07,193491,25` + "\n" +
		`spotify:track:0imko8mKmhQJlvBVZWgP4K,USANG2224442,"aquatic ambience - slowed + reverb","aquatic ambience (Remixes)","Scizzie",2022-10-31,154419,59` + "\n" +
		`spotify:track:1AeAJE0BNsq9jyssQDsUZ9,USANG2218395,"aquatic ambience","aquatic ambience","Scizzie",2022-10-15,123535,67` + "\n" +
		`spotify:track:41dF02sXROxmP1ld8KmVPp,QZWFQ2582933,"Aquatic Ambience","Super Ambient World","Helynt;GameChops;mellow mode",2025-09-02,216000,24` + "\n" +
		`spotify:track:0jiZzd2lyWPtn9lc1P3jNh,QZHN41952109,"Aquatic Ambience","Super Lofi World","Helynt;GameChops",2019-10-18,165269,30` + "\n" +
		`spotify:track:2P7MOgeQVeDDN43ji37XmX,QT6G22502227,"Aquatic Ambience","Aquatic Ambience","Moonlit Hollow",2025-12-19,147935,21` + "\n" +
		"\n"
	if err := os.WriteFile(filepath.Join(dir, "Aquatic_Ambiance.csv"), []byte(csvText), 0o644); err != nil {
		t.Fatal(err)
	}
	lists := ReadPlaylists(music, ix)
	if len(lists) != 1 || lists[0].Name != "Aquatic Ambiance" {
		t.Fatalf("playlists %+v", lists)
	}
	var got []string
	for _, s := range lists[0].Songs {
		got = append(got, strings.TrimPrefix(s.Path, music+"/"))
	}
	want := []string{"ff/1.flac", "sm/1.opus", "Boombrick/ko.opus", "sc/2.flac", "sc/1.flac", "ha/1.flac", "hl/1.flac"}
	if !slices.Equal(got, want) {
		t.Fatalf("songs %v, want %v", got, want)
	}
	// Moonlit Hollow's isn't in the library, so it streams from Spotify,
	// last as in the file.
	pl := lists[0]
	last := pl.Entries[len(pl.Entries)-1].Stream
	if pl.Missing != 0 || pl.Streams != 1 || len(pl.Entries) != 8 || last == nil ||
		*last != (StreamSong{SpotifyID: "2P7MOgeQVeDDN43ji37XmX", Title: "Aquatic Ambience", Artist: "Moonlit Hollow",
			Album: "Aquatic Ambience", Duration: 147935 * time.Millisecond}) {
		t.Fatalf("missing %d, streams %d, last entry %+v", pl.Missing, pl.Streams, last)
	}
}

// A .csv with a path column plays those files, as an .m3u does.
func TestReadPlaylistsCSVPaths(t *testing.T) {
	music := t.TempDir()
	ix := NewIndex([]Album{{Artist: "A", Title: "B", Tracks: []Track{{Path: filepath.Join(music, "A/B/1.flac"), Title: "One"}}}})
	dir := filepath.Join(music, PlaylistDir)
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "paths.csv"), []byte("Title,Path\nOne,A/B/1.flac\nGone,A/B/9.flac\n"), 0o644)
	lists := ReadPlaylists(music, ix)
	if len(lists) != 1 || len(lists[0].Songs) != 1 || lists[0].Missing != 1 {
		t.Fatalf("playlists %+v", lists[0])
	}
}

func TestBareTitle(t *testing.T) {
	for in, want := range map[string]string{
		`Aquatic Ambiance (From "Donkey Kong Country") - Piano Version`: "aquatic ambiance",
		"Donkey Kong Country - Aquatic Ambience":                        "donkey kong country",
		"AQUATIC/AMBIANCE":                                              "aquatic ambiance",
		"Café [Live]":                                                   "cafe",
	} {
		if got := bareTitle(in); got != want {
			t.Errorf("bareTitle(%q) = %q, want %q", in, got, want)
		}
	}
}
