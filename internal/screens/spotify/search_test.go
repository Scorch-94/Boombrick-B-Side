package spotify

import (
	"errors"
	"slices"
	"testing"

	"github.com/ujjwal313/boombrick/internal/platform/input"
	"github.com/ujjwal313/boombrick/internal/spotify"
	"github.com/ujjwal313/boombrick/internal/ui/uitest"
)

var stirling = spotify.Results{
	Tracks: []spotify.Track{
		{ID: "t1", Title: "Carol of the Bells", Artists: []string{"Lindsey Stirling", "Egzod"}, Album: "Holiday Favorites",
			Duration: 201000},
		{ID: "t2", Title: "Crystallize", Artists: []string{"Lindsey Stirling"}, Album: "Lindsey Stirling", Duration: 290000},
	},
	Albums:  []spotify.Release{{URI: "spotify:album:a1", Name: "Holiday Favorites", Artist: "Lindsey Stirling", Type: "Album", Tracks: 24}},
	Artists: []spotify.Artist{{URI: "spotify:artist:r1", Name: "Lindsey Stirling"}},
}

// X on the Spotify tab opens the search. Typing asks nothing; Y searches
// what's typed and goes to the results: songs, albums, then artists. A on a
// song plays the songs found from it, Y queues it; A on an album or an artist
// opens its page.
func TestSpotifySearch(t *testing.T) {
	nav, p := &fakeNav{}, &fakePlayer{}
	src := signedIn()
	src.results = map[string]spotify.Results{"lindsey stirling": stirling}
	e := &Env{Nav: nav, Src: src, Player: p}
	tab := NewTab(e)
	render(t, tab)
	tab.Handle(press(input.X), t0)
	s, ok := nav.pushed[len(nav.pushed)-1].(*Search)
	if !ok {
		t.Fatalf("X pushed %T", nav.pushed[len(nav.pushed)-1])
	}
	for _, r := range "lindsey stirling" {
		s.typeRune(r)
	}
	render(t, s)
	if len(src.searched) != 0 {
		t.Fatalf("typing searched %v", src.searched)
	}
	s.Handle(press(input.Y), t0)
	if !s.onResults || len(s.hits) != 4 || s.rows[2].Subtitle != "Album · Lindsey Stirling" || s.rows[3].Subtitle != "Artist" {
		t.Fatalf("on results %v, %d hits, rows %+v", s.onResults, len(s.hits), s.rows)
	}
	uitest.Golden(t, "spotify-search", render(t, s))
	s.Handle(press(input.Down), t0)
	s.Handle(press(input.A), t0)
	s.Handle(press(input.Y), t0)
	if !slices.Equal(p.played, []string{"t2@1"}) || len(p.queued) != 1 || !slices.Equal(p.queued[0], []string{"t2"}) {
		t.Fatalf("played %v, queued %v", p.played, p.queued)
	}
	s.Handle(press(input.Down), t0)
	s.Handle(press(input.A), t0)
	pg, ok := nav.pushed[len(nav.pushed)-1].(*PlaylistPage)
	if !ok || pg.pl.URI != "spotify:album:a1" {
		t.Fatalf("A on the album pushed %T", nav.pushed[len(nav.pushed)-1])
	}
	s.Handle(press(input.Down), t0)
	s.Handle(press(input.A), t0)
	if pg, ok := nav.pushed[len(nav.pushed)-1].(*PlaylistPage); !ok || pg.pl.URI != "spotify:artist:r1" {
		t.Fatalf("A on the artist pushed %T", nav.pushed[len(nav.pushed)-1])
	}
	if !s.Back() || s.onResults {
		t.Fatal("B goes back to the keyboard")
	}
}

// While Spotify is asked, and when it can't be, the results side says so.
func TestSpotifySearchStates(t *testing.T) {
	src := signedIn()
	s := NewSearch(&Env{Nav: &fakeNav{}, Src: src, Player: &fakePlayer{}})
	if s.message() != "Type, then Search, to find songs, albums and artists on Spotify." {
		t.Fatal(s.message())
	}
	for _, r := range "nothing" {
		s.typeRune(r)
	}
	s.Handle(press(input.Y), t0)
	if s.message() != "Searching Spotify…" || s.onResults {
		t.Fatalf("%q, on results %v", s.message(), s.onResults)
	}
	src.results = map[string]spotify.Results{"nothing": {}}
	src.version++
	render(t, s)
	if s.message() != "Nothing on Spotify matches “nothing”" {
		t.Fatal(s.message())
	}
	src.searchErr = errors.New("unavailable: Spotify is limiting searches; try again in a while")
	src.version++
	render(t, s)
	if s.message() != "Spotify is limiting searches right now. Try again in a while." {
		t.Fatal(s.message())
	}
}
