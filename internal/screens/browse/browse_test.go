package browse

import (
	"fmt"
	"image"
	"slices"
	"testing"
	"time"

	"github.com/ujjwal313/boombrick/internal/fixture"
	"github.com/ujjwal313/boombrick/internal/library"
	"github.com/ujjwal313/boombrick/internal/platform/input"
	"github.com/ujjwal313/boombrick/internal/player"
	"github.com/ujjwal313/boombrick/internal/screens"
	"github.com/ujjwal313/boombrick/internal/theme"
	"github.com/ujjwal313/boombrick/internal/ui/anim"
	"github.com/ujjwal313/boombrick/internal/ui/uitest"
	"github.com/ujjwal313/boombrick/internal/ui/widgets"
)

func TestMain(m *testing.M) { uitest.Main(m) }

var t0 = time.Unix(1000, 0)

func press(b input.Button) input.Event { return input.Event{Button: b, Kind: input.Press} }

// fakePlayer records what the screens ask of the player.
type fakePlayer struct {
	played   []string // first path of each Play
	lists    [][]string
	start    []int
	queued   [][]string
	shuffled []bool
	playing  string
}

func (p *fakePlayer) State() player.State {
	return player.State{Queued: p.playing != "", Track: player.Track{Path: p.playing}}
}
func (p *fakePlayer) Play(ts []player.Track, start int) {
	p.played, p.start = append(p.played, ts[0].Path), append(p.start, start)
	p.lists = append(p.lists, paths(ts))
}
func (p *fakePlayer) SetShuffle(on bool)        { p.shuffled = append(p.shuffled, on) }
func (p *fakePlayer) Enqueue(ts []player.Track) { p.queued = append(p.queued, paths(ts)) }

func paths(ts []player.Track) []string {
	var paths []string
	for _, t := range ts {
		if t.Path == "" && t.SpotifyID != "" {
			paths = append(paths, "spotify:"+t.SpotifyID)
			continue
		}
		paths = append(paths, t.Path)
	}
	return paths
}

// localPlaylist is a playlist of library songs.
func localPlaylist(pl library.Playlist, songs ...*library.Track) *library.Playlist {
	for _, s := range songs {
		pl.Entries = append(pl.Entries, library.PlaylistEntry{Song: s})
		pl.Songs = append(pl.Songs, s)
	}
	return &pl
}

// fakeNav records pushes and toasts.
type fakeNav struct {
	pushed []screens.Screen
	toasts []string
}

func (n *fakeNav) Push(s screens.Screen) { n.pushed = append(n.pushed, s) }
func (n *fakeNav) Pop()                  {}
func (n *fakeNav) Toast(msg string)      { n.toasts = append(n.toasts, msg) }

// sampleIndex is a small library with names from the design.
func sampleIndex() *library.Index { return library.NewIndex(sampleAlbums()) }

// genreIndex is sampleIndex with genres tagged: Folk, Jazz, Pop and Rock.
func genreIndex() *library.Index {
	albums := sampleAlbums()
	tag := func(album, track int, genres ...string) { albums[album].Tracks[track].Genres = genres }
	tag(0, 0, "Rock")         // Rust
	tag(0, 1, "Rock", "Pop")  // Ember
	tag(0, 2, "Folk")         // Tin Roof
	tag(1, 0, "Jazz", "Rock") // Neon
	tag(2, 0, "Folk")         // Salt
	return library.NewIndex(albums)
}

// rockSongs are genreIndex's Rock songs, by artist.
var rockSongs = []string{"/m/a/1.flac", "/m/Copper Season/Rust.flac", "/m/Copper Season/Ember.flac"}

func sampleAlbums() []library.Album {
	track := func(album, title string, n int, d time.Duration) library.Track {
		return library.Track{Path: "/m/" + album + "/" + title + ".flac", Title: title, Artist: "Odalys", Album: album,
			Number: n, Duration: d}
	}
	return []library.Album{
		{Artist: "Odalys", Title: "Copper Season", Year: 2019, Duration: 15 * time.Minute, Tracks: []library.Track{
			track("Copper Season", "Rust", 1, 222*time.Second), track("Copper Season", "Ember", 2, 245*time.Second),
			track("Copper Season", "Tin Roof", 3, 178*time.Second), track("Copper Season", "Low Tide", 4, 210*time.Second),
		}},
		{Artist: "Kenji Arata", Title: "Afterhours", Year: 2021, Duration: 4 * time.Minute, Tracks: []library.Track{
			{Path: "/m/a/1.flac", Title: "Neon", Artist: "Kenji Arata", Album: "Afterhours", Duration: 4 * time.Minute},
		}},
		{Artist: "Odalys", Title: "Northbound", Year: 2016, Tracks: []library.Track{
			{Path: "/m/n/1.flac", Title: "Salt", Artist: "Odalys", Album: "Northbound"},
		}},
	}
}

func render(t *testing.T, s screens.Screen, now time.Time) *image.NRGBA {
	t.Helper()
	u, off := uitest.NewContext(t, theme.DeviceW, theme.DeviceH)
	u.Accent = theme.Derive(fixture.PlayingHue, fixture.PlayingChroma)
	u.Status = fixture.Status()
	u.Now = now
	s.Draw(u)
	return off.Image()
}

// fadesSoftly fails if the frame steps sharply anywhere on the line from one
// device pixel to another, as a focus glow cut off by a clip does.
func fadesSoftly(t *testing.T, img *image.NRGBA, from, to image.Point) {
	t.Helper()
	if p, found := uitest.HardEdge(img, from, to); found {
		t.Fatalf("a hard edge at %v, between %v and %v", p, from, to)
	}
}

// The first track sits just under the tabs, focused as the page opens; its
// glow fades out under them.
func TestAlbumPageFirstTrackGlow(t *testing.T) {
	pg := NewAlbumPage(&fakeNav{}, &fakePlayer{}, sampleIndex().Album("Odalys", "Copper Season"))
	img := render(t, pg, t0.Add(anim.Focus))
	uitest.Golden(t, "album-first-track", img)
	// Between the YouTube and Downloads tabs, down to the track.
	fadesSoftly(t, img, image.Pt(700, 120), image.Pt(700, 154))
}

// The page opens on the first track, with no pills: A plays the album in
// order from the focused track, Y queues that track alone.
func TestAlbumPage(t *testing.T) {
	ix := sampleIndex()
	p, nav := &fakePlayer{playing: "/m/Copper Season/Ember.flac"}, &fakeNav{}
	pg := NewAlbumPage(nav, p, ix.Album("Odalys", "Copper Season"))
	uitest.Golden(t, "album", render(t, pg, t0))
	if pg.list.Focus() != 0 {
		t.Fatalf("focus %d as the page opens, want the first track", pg.list.Focus())
	}
	pg.Handle(press(input.A), t0)
	if !slices.Equal(p.shuffled, []bool{false}) || p.start[0] != 0 {
		t.Fatalf("A on the first track: shuffle %v, start %v", p.shuffled, p.start)
	}
	if got := pg.hints()[0].Label; got != "Play" {
		t.Fatalf("A's hint = %q", got)
	}
	pg.Handle(press(input.Down), t0)
	pg.Handle(press(input.A), t0)
	if p.start[1] != 1 || !slices.Equal(p.shuffled, []bool{false, false}) {
		t.Fatalf("A on the second track: start %v, shuffle %v", p.start, p.shuffled)
	}
	pg.Handle(press(input.Y), t0)
	if len(p.queued) != 1 || !slices.Equal(p.queued[0], []string{"/m/Copper Season/Ember.flac"}) || nav.toasts[0] != "Added to queue" {
		t.Fatalf("Y on a track queued %v, toasts %v", p.queued, nav.toasts)
	}
	if pg.Handle(press(input.Left), t0) {
		t.Fatal("Left has nothing to move to")
	}
	img := render(t, pg, t0.Add(anim.Focus))
	uitest.Golden(t, "album-track-focused", img)
	// From the art across to the second track.
	fadesSoftly(t, img, image.Pt(295, 312), image.Pt(322, 312))
}

func TestArtistPage(t *testing.T) {
	ix := sampleIndex()
	p, nav := &fakePlayer{}, &fakeNav{}
	ar := ix.Artists[1] // Odalys: Northbound (2016), Copper Season (2019)
	pg := NewArtistPage(nav, p, ar)
	img := render(t, pg, t0)
	uitest.Golden(t, "artist", img)
	// Right of the artist's name, down to the first card.
	fadesSoftly(t, img, image.Pt(300, 150), image.Pt(300, 220))
	pg.Handle(press(input.Right), t0)
	pg.Handle(press(input.A), t0)
	if len(nav.pushed) != 1 {
		t.Fatal("A should open the album")
	}
	if ap, ok := nav.pushed[0].(*AlbumPage); !ok || ap.title != "Copper Season" {
		t.Fatalf("opened %+v", nav.pushed[0])
	}
}

// odalysSongs are Odalys's songs in sampleIndex, albums oldest first.
var odalysSongs = []string{"/m/n/1.flac", "/m/Copper Season/Rust.flac", "/m/Copper Season/Ember.flac",
	"/m/Copper Season/Tin Roof.flac", "/m/Copper Season/Low Tide.flac"}

// shuffledAll fails unless p's one Play was songs, from any of them, with
// shuffle turned on.
func shuffledAll(t *testing.T, p *fakePlayer, songs []string) {
	t.Helper()
	if !slices.Equal(p.shuffled, []bool{true}) || len(p.lists) != 1 || !slices.Equal(p.lists[0], songs) {
		t.Fatalf("shuffle %v, played %q; want shuffle on and %q", p.shuffled, p.lists, songs)
	}
	if s := p.start[0]; s < 0 || s >= len(songs) {
		t.Fatalf("started at %d of %d", s, len(songs))
	}
}

// X on an artist's page shuffles all their songs, whichever album is focused.
func TestArtistPageShufflesAll(t *testing.T) {
	p := &fakePlayer{}
	pg := NewArtistPage(&fakeNav{}, p, sampleIndex().Artists[1])
	pg.Handle(press(input.Right), t0)
	if !pg.Handle(press(input.X), t0) {
		t.Fatal("X should be taken")
	}
	shuffledAll(t, p, odalysSongs)
	if got := pg.hints()[1]; got.Key != "X" || got.Label != "Shuffle all" {
		t.Fatalf("the second hint is %+v", got)
	}
}

// X opens the search from any filter, and the hints say so.
func TestLibraryXOpensSearch(t *testing.T) {
	nav := &fakeNav{}
	l := New(nav, &fakePlayer{}, "/m", nil)
	l.SetIndex(sampleIndex())
	for f := range playlistsFilter + 1 {
		if !l.Handle(press(input.X), t0) {
			t.Fatalf("X on filter %d went unhandled", f)
		}
		if !slices.Contains(l.hints(), widgets.Hint{Key: "X", Label: "Search"}) {
			t.Fatalf("filter %d: hints %+v have no X Search", f, l.hints())
		}
		l.Handle(press(input.R2), t0)
	}
	if len(nav.pushed) != 4 {
		t.Fatalf("pushed %d screens, want 4", len(nav.pushed))
	}
	if _, ok := nav.pushed[0].(*Search); !ok {
		t.Fatalf("X pushed %T", nav.pushed[0])
	}
}

func typeQuery(s *Search, q string) {
	for _, r := range q {
		if r == ' ' {
			s.space()
		} else {
			s.typeRune(r)
		}
	}
}

// The search finds artists, then albums, then songs, every word anywhere in
// a name, without case or accents.
func TestSearchFinds(t *testing.T) {
	s := NewSearch(&fakeNav{}, &fakePlayer{}, sampleIndex())
	if len(s.hits) != 0 {
		t.Fatalf("%d hits before typing", len(s.hits))
	}
	typeQuery(s, "odalys")
	kinds := []int{}
	for _, h := range s.hits {
		kinds = append(kinds, h.kind)
	}
	// Odalys; Copper Season and Northbound; their five songs.
	want := []int{artistHit, albumHit, albumHit, songHit, songHit, songHit, songHit, songHit}
	if !slices.Equal(kinds, want) {
		t.Fatalf("kinds %v, want %v", kinds, want)
	}
	s.query = ""
	typeQuery(s, "TIN cOPPER")
	if len(s.hits) != 1 || s.hits[0].song == nil || s.hits[0].song.Title != "Tin Roof" {
		t.Fatalf("hits %+v, want Tin Roof alone", s.hits)
	}
	if fold("Beyoncé") != "beyonce" {
		t.Fatalf("fold(Beyoncé) = %q", fold("Beyoncé"))
	}
}

// On the results, A plays a song from the songs found, opens an album or an
// artist, and Y queues; B goes back to the keyboard, then deletes, and with
// nothing typed leaves B to the shell.
func TestSearchResults(t *testing.T) {
	p, nav := &fakePlayer{}, &fakeNav{}
	s := NewSearch(nav, p, sampleIndex())
	if s.Handle(press(input.Y), t0) {
		t.Fatal("Y went to results with none")
	}
	typeQuery(s, "o")
	if !s.Handle(press(input.Y), t0) || !s.onResults {
		t.Fatal("Y should go to the results")
	}
	s.Handle(press(input.A), t0) // Odalys
	if _, ok := nav.pushed[0].(*ArtistPage); !ok {
		t.Fatalf("A on the artist pushed %T", nav.pushed[0])
	}
	for s.hits[s.list.Focus()].kind != songHit {
		s.Handle(press(input.Down), t0)
	}
	s.Handle(press(input.Down), t0) // the second song found
	s.Handle(press(input.A), t0)
	if len(p.lists) != 1 || len(p.lists[0]) != len(s.songs) || p.start[0] != 1 {
		t.Fatalf("played %v from %v", p.lists, p.start)
	}
	s.Handle(press(input.Y), t0)
	if len(p.queued) != 1 || len(p.queued[0]) != 1 {
		t.Fatalf("queued %v", p.queued)
	}
	if !s.Back() || s.onResults {
		t.Fatal("B on the results should go to the keyboard")
	}
	if !s.Back() || s.query != "" {
		t.Fatalf("B should delete, query %q", s.query)
	}
	if s.Back() {
		t.Fatal("B with nothing typed should close the search")
	}
}

func TestSearchHintsFit(t *testing.T) {
	u, _ := uitest.NewContext(t, theme.DeviceW, theme.DeviceH)
	for _, h := range [][]widgets.Hint{searchTypingHints, searchAloneHints, searchSongHints, searchAlbumHints,
		searchArtistHints, openHints, songHints} {
		if w := widgets.HintsWidth(u, h); w > widgets.MaxHintsWidth {
			t.Errorf("hints %v are %.1f wide, more than %.1f", h, w, widgets.MaxHintsWidth)
		}
	}
}

func TestSearchGolden(t *testing.T) {
	s := NewSearch(&fakeNav{}, &fakePlayer{}, sampleIndex())
	typeQuery(s, "o")
	s.Handle(press(input.Right), t0)
	uitest.Golden(t, "library-search", render(t, s, t0.Add(anim.Focus)))
}

// With songs tagged, Genres is the last filter: A opens a genre and Y
// queues it.
func TestLibraryGenres(t *testing.T) {
	p, nav := &fakePlayer{}, &fakeNav{}
	l := New(nav, p, "/m", nil)
	l.SetIndex(genreIndex())
	for range 4 {
		l.Handle(press(input.R2), t0)
	}
	if l.filter != genresFilter || l.Handle(press(input.R2), t0) {
		t.Fatal("R2 from Playlists should show the genres, the last filter")
	}
	for range 3 {
		l.Handle(press(input.Down), t0) // Folk, Jazz, Pop, Rock
	}
	l.Handle(press(input.Y), t0)
	if len(p.queued) != 1 || !slices.Equal(p.queued[0], rockSongs) || len(nav.toasts) != 1 {
		t.Fatalf("Y on Rock queued %q, toasts %v", p.queued, nav.toasts)
	}
	l.Handle(press(input.A), t0)
	if pg, ok := nav.pushed[0].(*AlbumPage); !ok || pg.title != "Rock" {
		t.Fatalf("A on Rock opened %+v", nav.pushed)
	}
	uitest.Golden(t, "library-genres", render(t, l, t0.Add(anim.Focus)))
}

// A library without genre tags has no Genres filter; one whose genres go
// while it shows shows the songs instead.
func TestLibraryHidesGenresWithoutTags(t *testing.T) {
	l := New(&fakeNav{}, &fakePlayer{}, "/m", nil)
	l.SetIndex(sampleIndex())
	for range 3 {
		l.Handle(press(input.R2), t0)
	}
	if l.filter != playlistsFilter || l.Handle(press(input.R2), t0) {
		t.Fatal("R2 on Playlists should do nothing without genres")
	}
	l.SetIndex(genreIndex())
	if !l.Handle(press(input.R2), t0) || l.filter != genresFilter {
		t.Fatal("R2 on Playlists should show the genres once there are some")
	}
	l.SetIndex(sampleIndex())
	if l.filter != songsFilter {
		t.Fatalf("filter %d once the genres went, want the songs", l.filter)
	}
	render(t, l, t0)
}

// playlistIndex is sampleIndex with two playlists.
func playlistIndex() *library.Index {
	ix := sampleIndex()
	byTitle := map[string]*library.Track{}
	for _, t := range ix.Songs {
		byTitle[t.Title] = t
	}
	late := localPlaylist(library.Playlist{Name: "Late Drive", Path: "/m/Playlists/Late Drive.csv", Missing: 2,
		Streams: 1, Duration: 467*time.Second + 3*time.Minute}, byTitle["Neon"], byTitle["Rust"])
	late.Entries = append(late.Entries, library.PlaylistEntry{Stream: &library.StreamSong{SpotifyID: "2P7MOgeQ",
		Title: "Aquatic Ambience", Artist: "Moonlit Hollow", Album: "Aquatic Ambience", Duration: 3 * time.Minute}})
	ix.Playlists = []*library.Playlist{late,
		localPlaylist(library.Playlist{Name: "Morning", Path: "/m/Playlists/Morning.m3u"}, byTitle["Salt"])}
	return ix
}

// The Playlists filter lists the playlists; A opens one as a page in the
// file's order, Y queues it.
func TestLibraryPlaylists(t *testing.T) {
	p, nav := &fakePlayer{}, &fakeNav{}
	l := New(nav, p, "/m", nil)
	l.SetIndex(playlistIndex())
	for range 3 {
		l.Handle(press(input.R2), t0)
	}
	if l.filter != playlistsFilter {
		t.Fatalf("filter %d after three R2, want Playlists", l.filter)
	}
	if r := l.row(0); r.subtitle != "3 songs · 10 min · 1 Spotify · 2 not found" {
		t.Fatalf("the first row says %q", r.subtitle)
	}
	l.Handle(press(input.Y), t0)
	if len(p.queued) != 1 || !slices.Equal(p.queued[0], []string{"/m/a/1.flac", "/m/Copper Season/Rust.flac", "spotify:2P7MOgeQ"}) {
		t.Fatalf("Y queued %q", p.queued)
	}
	l.Handle(press(input.A), t0)
	pg, ok := nav.pushed[0].(*AlbumPage)
	if !ok || pg.title != "Late Drive" || pg.who != "Playlist · 2 not found" || pg.rows[0].Title != "Neon" || pg.rows[0].Subtitle != "Kenji Arata" ||
		pg.rows[2].Subtitle != "Spotify · Moonlit Hollow" {
		t.Fatalf("A opened %+v", nav.pushed[0])
	}
	pg.Handle(press(input.A), t0) // Play
	if len(p.lists) != 1 || p.lists[0][0] != "/m/a/1.flac" {
		t.Fatalf("Play played %v", p.lists)
	}
	uitest.Golden(t, "library-playlists", render(t, l, t0.Add(anim.Focus)))
	uitest.Golden(t, "playlist-page", render(t, pg, t0.Add(anim.Focus)))
}

// The songs list opens with Shuffle all songs: A plays every song in the
// library shuffled, Y queues them all, and the songs follow it.
func TestLibraryShuffleAllSongs(t *testing.T) {
	p, nav := &fakePlayer{}, &fakeNav{}
	l := New(nav, p, "/m", nil)
	ix := sampleIndex()
	l.SetIndex(ix)
	l.filter = songsFilter
	if l.views[songsFilter].Count() != len(ix.Songs)+1 || l.views[songsFilter].Focus() != 0 {
		t.Fatalf("count %d, focus %d", l.views[songsFilter].Count(), l.views[songsFilter].Focus())
	}
	if h := l.hints()[0]; h.Label != "Shuffle all" {
		t.Fatalf("A hint %q on the Shuffle all row", h.Label)
	}
	l.Handle(press(input.A), t0)
	all := paths(songTracks(ix.Songs))
	if len(p.lists) != 1 || !slices.Equal(p.lists[0], all) || !slices.Equal(p.shuffled, []bool{true}) {
		t.Fatalf("played %v, shuffle %v; want every song shuffled", p.lists, p.shuffled)
	}
	l.Handle(press(input.Y), t0)
	if len(p.queued) != 1 || len(p.queued[0]) != len(ix.Songs) {
		t.Fatalf("queued %v", p.queued)
	}
	l.Handle(press(input.Down), t0) // the first song
	l.Handle(press(input.A), t0)
	if len(p.lists) != 2 || p.start[1] != 0 || p.shuffled[1] {
		t.Fatalf("A on the first song started at %v, shuffle %v", p.start, p.shuffled)
	}
	l.Handle(press(input.Up), t0)
	uitest.Golden(t, "library-shuffle-all", render(t, l, t0.Add(anim.Focus)))
}

// With no playlists, the filter says where to put them, and Y rescans.
func TestLibraryNoPlaylists(t *testing.T) {
	scans, nav := 0, &fakeNav{}
	l := New(nav, &fakePlayer{}, "/mnt/SDCARD/Music", func() { scans++ })
	l.SetIndex(sampleIndex())
	l.filter = playlistsFilter
	if !l.Handle(press(input.Y), t0) || scans != 1 || len(nav.toasts) != 1 {
		t.Fatalf("Y rescanned %d times, toasts %v", scans, nav.toasts)
	}
	uitest.Golden(t, "library-no-playlists", render(t, l, t0))
}

// With genres too, the filter is called Lists so the row fits.
func TestLibraryListsWithGenres(t *testing.T) {
	l := New(&fakeNav{}, &fakePlayer{}, "/m", nil)
	ix := genreIndex()
	l.SetIndex(ix)
	render(t, l, t0)
	if !slices.Equal(l.names, []string{"Albums", "Artists", "Songs", "Lists", "Genres"}) {
		t.Fatalf("filters %q", l.names)
	}
}

// When the filters' counts don't all fit, only the chosen filter shows its
// count.
func TestLibraryCountsThatDontFit(t *testing.T) {
	var albums []library.Album
	for i := range 1000 {
		a := library.Album{Artist: fmt.Sprintf("Artist %d", i%100), Title: fmt.Sprintf("Album %d", i)}
		for j := range 10 {
			a.Tracks = append(a.Tracks, library.Track{Path: fmt.Sprintf("/m/%d/%d.flac", i, j),
				Title: fmt.Sprintf("Song %d", j), Genres: []string{fmt.Sprintf("Genre %d", (i+j)%30)}})
		}
		albums = append(albums, a)
	}
	l := New(&fakeNav{}, &fakePlayer{}, "/m", nil)
	l.SetIndex(library.NewIndex(albums))
	l.Handle(press(input.R2), t0)
	img := render(t, l, t0.Add(anim.Focus))
	if l.allCounts {
		t.Fatal("1,000 albums, 100 artists, 10,000 songs and 30 genres can't all show their counts")
	}
	l.SetIndex(sampleIndex())
	render(t, l, t0)
	if !l.allCounts {
		t.Fatal("a small library's counts all fit")
	}
	uitest.Golden(t, "library-one-count", img)
}

// A genre's page is laid out as an album's, its songs each with their artist.
func TestGenrePage(t *testing.T) {
	p := &fakePlayer{playing: "/m/Copper Season/Ember.flac"}
	pg := NewGenrePage(&fakeNav{}, p, genreIndex().Genres[3])
	if pg.title != "Rock" || pg.who != "2 artists" || pg.length != "3 songs · 11 min" || pg.rows[0].Subtitle != "Kenji Arata" {
		t.Fatalf("page %q, %q, %q; first row %+v", pg.title, pg.who, pg.length, pg.rows[0])
	}
	pg.Handle(press(input.A), t0) // the first song
	pg.Handle(press(input.Down), t0)
	pg.Handle(press(input.A), t0) // the second song
	if !slices.Equal(p.shuffled, []bool{false, false}) || p.start[0] != 0 || p.start[1] != 1 {
		t.Fatalf("shuffle %v, starts %v", p.shuffled, p.start)
	}
	for _, l := range p.lists {
		if !slices.Equal(l, rockSongs) {
			t.Fatalf("played %q, want %q", l, rockSongs)
		}
	}
	pg.Handle(press(input.Y), t0)
	if len(p.queued) != 1 || !slices.Equal(p.queued[0], rockSongs[1:2]) {
		t.Fatalf("Y on the second song queued %q", p.queued)
	}
	if pg := NewGenrePage(&fakeNav{}, p, genreIndex().Genres[1]); pg.who != "1 artist" {
		t.Fatalf("Jazz is by %q", pg.who)
	}
	uitest.Golden(t, "genre", render(t, NewGenrePage(&fakeNav{}, p, genreIndex().Genres[3]), t0))
}

func TestPageHintsFit(t *testing.T) {
	u, _ := uitest.NewContext(t, 64, 64)
	ix := sampleIndex()
	shuffle := NewAlbumPage(&fakeNav{}, &fakePlayer{}, ix.Recent[0])
	shuffle.Handle(press(input.Right), t0)
	for _, hints := range [][]widgets.Hint{
		NewAlbumPage(&fakeNav{}, &fakePlayer{}, ix.Recent[0]).hints(),
		shuffle.hints(),
		NewArtistPage(&fakeNav{}, &fakePlayer{}, ix.Artists[0]).hints(),
	} {
		if w := widgets.HintsWidth(u, hints); w > widgets.MaxHintsWidth {
			t.Errorf("hints %v are %.1f wide, more than %.1f", hints, w, widgets.MaxHintsWidth)
		}
	}
}

func TestLibraryFilters(t *testing.T) {
	ix := sampleIndex()
	p, nav := &fakePlayer{}, &fakeNav{}
	l := New(nav, p, "/mnt/SDCARD/Music", nil)
	l.SetIndex(ix)
	img := render(t, l, t0)
	uitest.Golden(t, "library-albums", img)
	// Between the Albums and Artists filters, down to the first card.
	fadesSoftly(t, img, image.Pt(300, 150), image.Pt(300, 216))
	l.Handle(press(input.A), t0)
	if ap, ok := nav.pushed[0].(*AlbumPage); !ok || ap.title != ix.Albums[0].Title {
		t.Fatalf("A on the first album opened %+v", nav.pushed)
	}
	if !l.Handle(press(input.R2), t0) || l.filter != artistsFilter {
		t.Fatal("R2 should show artists")
	}
	img = render(t, l, t0.Add(anim.Focus))
	uitest.Golden(t, "library-artists", img)
	// Right of the filters, down to the first row.
	fadesSoftly(t, img, image.Pt(960, 150), image.Pt(960, 225)) // past R2, now that Playlists makes four filters
	l.Handle(press(input.Y), t0)                                // Kenji Arata's one album
	if len(p.queued) != 1 || len(p.queued[0]) != 1 {
		t.Fatalf("Y on an artist queued %v", p.queued)
	}
	l.Handle(press(input.R2), t0)
	l.Handle(press(input.Down), t0) // past Shuffle all songs
	l.Handle(press(input.Down), t0)
	l.Handle(press(input.A), t0)
	if len(p.played) != 1 || p.start[0] != 1 {
		t.Fatalf("A on the second song: played %v from %v", p.played, p.start)
	}
	if !slices.Equal(p.shuffled, []bool{false}) {
		t.Fatalf("shuffle set %v: A on a song plays the list in order from it", p.shuffled)
	}
	uitest.Golden(t, "library-songs", render(t, l, t0.Add(anim.Focus)))
	if !l.Handle(press(input.R2), t0) || l.filter != playlistsFilter {
		t.Fatal("R2 after Songs should show playlists")
	}
	if l.Handle(press(input.R2), t0) {
		t.Fatal("R2 on the last filter does nothing")
	}
	l.Handle(press(input.L2), t0)
	l.Handle(press(input.L2), t0)
	l.Handle(press(input.L2), t0)
	if l.filter != albumsFilter {
		t.Fatal("L2 goes back through the filters")
	}
}

// Frames drawn again allocate nothing: rows' lines and art, counts and hint
// bars are made once.
func TestFramesDrawnAgainAllocateNothing(t *testing.T) {
	u, _ := uitest.NewContext(t, theme.DeviceW, theme.DeviceH)
	u.Status = fixture.Status()
	u.Now = t0
	ix := genreIndex()
	draw := func(name string, s screens.Screen) {
		t.Helper()
		s.Draw(u)
		if n := testing.AllocsPerRun(10, func() { s.Draw(u) }); n != 0 {
			t.Errorf("%s: a frame allocates %v times", name, n)
		}
	}
	l := New(&fakeNav{}, &fakePlayer{playing: ix.Songs[1].Path}, "/m", nil)
	l.SetIndex(ix)
	for f := range filterCount {
		l.filter = f
		draw(filterNames[f], l)
	}
	draw("an artist's page", NewArtistPage(&fakeNav{}, &fakePlayer{}, ix.Artists[0]))
	draw("an album's page", NewAlbumPage(&fakeNav{}, &fakePlayer{}, ix.Albums[0]))
	draw("a genre's page", NewGenrePage(&fakeNav{}, &fakePlayer{}, ix.Genres[0]))
}

// A new index's rows are made afresh.
func TestNewIndexRowsAreMadeAfresh(t *testing.T) {
	u, _ := uitest.NewContext(t, theme.DeviceW, theme.DeviceH)
	l := New(&fakeNav{}, &fakePlayer{}, "/m", nil)
	l.SetIndex(sampleIndex())
	l.filter = songsFilter
	l.Draw(u)
	l.SetIndex(library.NewIndex([]library.Album{{Artist: "Marisol Vega", Title: "Salt & Static", Tracks: []library.Track{
		{Path: "/m/s/1.flac", Title: "Low Tide Radio", Artist: "Marisol Vega", Album: "Salt & Static", Duration: 222 * time.Second},
	}}}))
	l.Draw(u)
	if r := l.rows[songsFilter][0]; r.subtitle != "Marisol Vega · Salt & Static" || r.duration != "3:42" {
		t.Fatalf("the first song's row: %q, %q", r.subtitle, r.duration)
	}
	if l.counts != [filterCount]string{"1", "1", "1", "0", "0"} {
		t.Fatalf("counts %q", l.counts)
	}
}

func TestEmptyLibrary(t *testing.T) {
	scans := 0
	nav := &fakeNav{}
	l := New(nav, &fakePlayer{}, "/mnt/SDCARD/Music", func() { scans++ })
	if l.Handle(press(input.Y), t0) || scans != 0 {
		t.Fatal("before the index loads, Y does nothing")
	}
	l.SetIndex(library.NewIndex(nil))
	uitest.Golden(t, "library-empty", render(t, l, t0))
	if !l.Handle(press(input.Y), t0) || scans != 1 || len(nav.toasts) != 1 {
		t.Fatalf("Y on the empty library: %d scans, toasts %v", scans, nav.toasts)
	}
}

// While the first scan runs with nothing indexed, the tab says the music is
// being read, with only the tab hint, until the index arrives.
func TestLibraryWhileReading(t *testing.T) {
	scans := 0
	l := New(&fakeNav{}, &fakePlayer{}, "/mnt/SDCARD/Music", func() { scans++ })
	if got := l.hints(); len(got) != 1 || got[0].Key != "L1/R1" {
		t.Fatalf("hints before the index loads = %v, want only L1/R1 Tabs", got)
	}
	l.SetReading()
	render(t, l, t0) // the reading state; Home's golden covers how it looks
	if got := l.hints(); len(got) != 1 || got[0].Key != "L1/R1" {
		t.Fatalf("hints while reading = %v, want only L1/R1 Tabs", got)
	}
	if l.Handle(press(input.Y), t0) || scans != 0 {
		t.Fatal("Y while reading should not rescan")
	}
	l.SetIndex(sampleIndex())
	if got := l.hints(); len(got) != 5 {
		t.Fatalf("hints once the index loads = %v", got)
	}
}

func TestLibraryHintsFit(t *testing.T) {
	u, _ := uitest.NewContext(t, 64, 64)
	l := New(&fakeNav{}, &fakePlayer{}, "", nil)
	l.SetIndex(genreIndex())
	for f := range filterCount {
		l.filter = f
		if w := widgets.HintsWidth(u, l.hints()); w > widgets.MaxHintsWidth {
			t.Errorf("filter %d hints are %.1f wide, more than %.1f", f, w, widgets.MaxHintsWidth)
		}
	}
}

func TestEmptyAlbum(t *testing.T) {
	p, nav := &fakePlayer{}, &fakeNav{}
	pg := NewAlbumPage(nav, p, &library.Album{Title: "Nothing", Artist: "Nobody"})
	pg.Handle(press(input.A), t0)
	pg.Handle(press(input.Y), t0)
	if len(p.played) != 0 || len(p.queued) != 0 || len(nav.toasts) != 0 {
		t.Fatalf("an empty album played %v, queued %v, toasted %v", p.played, p.queued, nav.toasts)
	}
}

func TestSongCounts(t *testing.T) {
	if got := artistLine(1, 1); got != "1 album · 1 song" {
		t.Errorf("one album, one song: %q", got)
	}
	if got := artistLine(2, 1234); got != "2 albums · 1,234 songs" {
		t.Errorf("two albums: %q", got)
	}
}

// A reload that inserts albums, artists, songs and genres ahead of the
// focused ones keeps focus on the same items.
func TestLibraryKeepsFocusAcrossReloads(t *testing.T) {
	l := New(&fakeNav{}, &fakePlayer{}, "/m", nil)
	ix := genreIndex()
	l.SetIndex(ix)
	render(t, l, t0) // lays out the album grid
	for f := range filterCount {
		l.filter = f
		l.views[f].SetFocus(l.views[f].Count()-1, t0)
	}
	album, artist, song := ix.Albums[len(ix.Albums)-1], ix.Artists[len(ix.Artists)-1], ix.Songs[len(ix.Songs)-1]
	genre := ix.Genres[len(ix.Genres)-1]

	more := []library.Album{{Artist: "Aaron", Title: "Aardvark", Tracks: []library.Track{
		{Path: "/m/aa/1.flac", Title: "Aa", Artist: "Aaron", Album: "Aardvark", Genres: []string{"Ambient"}},
	}}}
	for _, a := range ix.Recent {
		more = append(more, *a)
	}
	ix2 := library.NewIndex(more)
	l.SetIndex(ix2)
	if a := ix2.Albums[l.views[albumsFilter].Focus()]; a.Title != album.Title {
		t.Errorf("album focus moved to %q, want %q", a.Title, album.Title)
	}
	if a := ix2.Artists[l.views[artistsFilter].Focus()]; a.Name != artist.Name {
		t.Errorf("artist focus moved to %q, want %q", a.Name, artist.Name)
	}
	if s := ix2.Songs[l.views[songsFilter].Focus()-1]; s.Path != song.Path {
		t.Errorf("song focus moved to %q, want %q", s.Path, song.Path)
	}
	if g := ix2.Genres[l.views[genresFilter].Focus()]; g.Name != genre.Name {
		t.Errorf("genre focus moved to %q, want %q", g.Name, genre.Name)
	}
}

func TestLibraryAlbumsWithArt(t *testing.T) {
	l := New(&fakeNav{}, &fakePlayer{}, "/m", nil)
	l.SetIndex(sampleIndex())
	u, off := uitest.NewContext(t, theme.DeviceW, theme.DeviceH)
	uitest.WithCovers(u)
	u.Accent = theme.Derive(fixture.PlayingHue, fixture.PlayingChroma)
	u.Status = fixture.Status()
	for _, at := range []time.Time{t0, t0.Add(anim.ArtFade)} {
		u.BeginFrame(at)
		l.Draw(u)
	}
	uitest.Golden(t, "library-albums-art", off.Image())
}
