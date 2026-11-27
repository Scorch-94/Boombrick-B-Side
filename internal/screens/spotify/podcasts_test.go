package spotify

import (
	"fmt"
	"github.com/ujjwal313/boombrick/internal/platform/input"
	"slices"
	"testing"
	"time"

	"github.com/ujjwal313/boombrick/internal/player"
	"github.com/ujjwal313/boombrick/internal/spotify"
	"github.com/ujjwal313/boombrick/internal/ui/uitest"
	"github.com/ujjwal313/boombrick/internal/ui/widgets"
)

var (
	owls = spotify.Show{URI: "spotify:show:owls", Name: "Night Owls", Publisher: "Owl FM", Cover: cover + "ow"}
	tide = spotify.Show{URI: "spotify:show:tide", Name: "Low Tide Talks", Publisher: "Harbor Media", Cover: cover + "lt"}
	ep1  = spotify.Episode{ID: "ep1", Title: "The Long Night Shift", Show: "Night Owls", ShowURI: owls.URI, Publisher: "Owl FM",
		Cover: cover + "ow", Duration: 45 * 60_000, Published: time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC).Unix(),
		Position: 12 * 60_000}
	ep2 = spotify.Episode{ID: "ep2", Title: "Moths and Streetlights", Show: "Night Owls", ShowURI: owls.URI,
		Duration: 38 * 60_000, Published: time.Date(2026, 11, 13, 0, 0, 0, 0, time.UTC).Unix(), Finished: true}
	ep3 = spotify.Episode{ID: "ep3", Title: "Tides, Explained", Show: "Low Tide Talks", ShowURI: tide.URI,
		Duration: 61 * 60_000, Published: time.Date(2025, 6, 2, 0, 0, 0, 0, time.UTC).Unix(), External: true}
)

func podcastSource() *fakeSource {
	s := signedIn()
	s.shows = []spotify.Show{owls, tide}
	s.episodes = map[string][]spotify.Episode{
		spotify.ContinueURI:     {ep1},
		spotify.YourEpisodesURI: {ep3},
		owls.URI:                {ep1, ep2},
	}
	return s
}

// Continue listening plays an episode on its own from where the user left
// it; Y queues it. L2 and R2 change filter.
func TestPodcastsContinue(t *testing.T) {
	nav, p := &fakeNav{}, &fakePlayer{}
	tab := NewPodcasts(&Env{Nav: nav, Src: podcastSource(), Player: p})
	uitest.Golden(t, "podcasts-continue", render(t, tab))
	if r := tab.rows[continueFilter][0]; r.Title != ep1.Title || r.Subtitle != "Night Owls · 33 min left" {
		t.Fatalf("row %+v", r)
	}
	tab.Handle(press(input.A), t0)
	// 12 minutes in, at 44.1 kHz.
	if !slices.Equal(p.played, []string{"episode:ep1@0+31752000"}) {
		t.Fatalf("played %v", p.played)
	}
	tab.Handle(press(input.Y), t0)
	if !slices.Equal(p.queued[0], []string{"episode:ep1"}) || nav.toasts[0] != "Added to queue" {
		t.Fatalf("queued %v, toasts %v", p.queued, nav.toasts)
	}
	if tab.Handle(press(input.L2), t0) {
		t.Fatal("L2 on the first filter")
	}
	tab.Handle(press(input.R2), t0)
	tab.Handle(press(input.R2), t0)
	if tab.filter != savedFilter || tab.Handle(press(input.R2), t0) {
		t.Fatalf("filter %d after two R2", tab.filter)
	}
}

// Shows lists the podcasts followed; A opens one, newest episode first,
// played from where the user left it, or from the start once played.
func TestPodcastShows(t *testing.T) {
	nav, p := &fakeNav{}, &fakePlayer{}
	e := &Env{Nav: nav, Src: podcastSource(), Player: p}
	tab := NewPodcasts(e)
	tab.Handle(press(input.R2), t0)
	render(t, tab)
	if len(tab.rows[showsFilter]) != 2 || tab.rows[showsFilter][1].Subtitle != "Harbor Media" {
		t.Fatalf("show rows %+v", tab.rows[showsFilter])
	}
	uitest.Golden(t, "podcasts-shows", render(t, tab))
	tab.Handle(press(input.A), t0)
	pg, ok := nav.pushed[0].(*ShowPage)
	if !ok || pg.show.URI != owls.URI {
		t.Fatalf("A opened %+v", nav.pushed)
	}
	uitest.Golden(t, "podcast-show", render(t, pg))
	if pg.count != "2 episodes" || pg.rows[1].Subtitle != "Nov 13 · Played" {
		t.Fatalf("count %q, second row %+v", pg.count, pg.rows[1])
	}
	pg.Handle(press(input.Down), t0)
	pg.Handle(press(input.A), t0)
	if !slices.Equal(p.played, []string{"episode:ep2@0+0"}) {
		t.Fatalf("a played episode played %v", p.played)
	}
}

// An episode Spotify doesn't host says so, and doesn't play.
func TestExternalEpisode(t *testing.T) {
	nav, p := &fakeNav{}, &fakePlayer{}
	tab := NewPodcasts(&Env{Nav: nav, Src: podcastSource(), Player: p})
	tab.Handle(press(input.R2), t0)
	tab.Handle(press(input.R2), t0)
	render(t, tab)
	if r := tab.rows[savedFilter][0]; r.Subtitle != "Low Tide Talks · Not playable yet" {
		t.Fatalf("row %+v", r)
	}
	tab.Handle(press(input.A), t0)
	tab.Handle(press(input.Y), t0)
	if len(p.played) != 0 || len(p.queued) != 0 || len(nav.toasts) != 2 || nav.toasts[0] != externalToast {
		t.Fatalf("played %v, queued %v, toasts %v", p.played, p.queued, nav.toasts)
	}
}

// Signed out, the tab says why it's empty and A leads to the sign-in.
func TestPodcastsSignedOut(t *testing.T) {
	nav := &fakeNav{}
	tab := NewPodcasts(&Env{Nav: nav, Src: &fakeSource{status: spotify.Status{State: spotify.SignedOut}}, Player: &fakePlayer{}})
	render(t, tab)
	if !tab.Handle(press(input.A), t0) {
		t.Fatal("A should lead to the sign-in")
	}
	if _, ok := nav.pushed[0].(*SignIn); !ok {
		t.Fatalf("pushed %T", nav.pushed[0])
	}
}

// An episode as a queue entry is streamed, and known as an episode.
func TestEpisodeTrack(t *testing.T) {
	tr := EpisodeTrack(ep1)
	if !tr.Streamed() || !IsEpisode(tr) || tr.Artist != "Night Owls" || tr.Album != "Owl FM" ||
		tr.Duration != 45*time.Minute {
		t.Fatalf("track %+v", tr)
	}
	if IsEpisode(player.Track{SpotifyID: "4uLU6hMCjMI75M1A2tKUQC"}) {
		t.Fatal("a song isn't an episode")
	}
	for ms, want := range map[int64]string{20_000: "1 min", 45 * 60_000: "45 min", 60 * 60_000: "1 hr", 125 * 60_000: "2 hr 5 min"} {
		if got := minutes(ms); got != want {
			t.Errorf("minutes(%d) = %q, want %q", ms, got, want)
		}
	}
}

func TestPodcastHintsFit(t *testing.T) {
	u, _ := uitest.NewContext(t, 1024, 768)
	for _, h := range [][]widgets.Hint{episodeHints, showHints, emptyListHints, retryPodHints, podSignInHints,
		showEpisodeHints, showRetryHints} {
		if w := widgets.HintsWidth(u, h); w > widgets.MaxHintsWidth {
			t.Errorf("hints %v are %.1f wide, more than %.1f", h, w, widgets.MaxHintsWidth)
		}
	}
}

// A show with more episodes than are listed says its list is the latest.
func TestLongShow(t *testing.T) {
	src := podcastSource()
	var eps []spotify.Episode
	for i := range spotify.MaxShowEpisodes {
		e := ep2
		e.ID, e.Title = fmt.Sprintf("e%d", i), fmt.Sprintf("Episode %d", spotify.MaxShowEpisodes-i)
		eps = append(eps, e)
	}
	src.episodes[owls.URI] = eps
	pg := NewShowPage(&Env{Nav: &fakeNav{}, Src: src, Player: &fakePlayer{}}, owls)
	uitest.Golden(t, "podcast-show-long", render(t, pg))
	if pg.count != "Latest 200 episodes" {
		t.Fatalf("count %q", pg.count)
	}
}
