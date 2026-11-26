package settings

import (
	"slices"
	"testing"
	"time"

	"github.com/ujjwal313/boombrick/internal/download"
	"github.com/ujjwal313/boombrick/internal/platform/input"
	"github.com/ujjwal313/boombrick/internal/release"
	keys "github.com/ujjwal313/boombrick/internal/settings"
	"github.com/ujjwal313/boombrick/internal/spotify"
	"github.com/ujjwal313/boombrick/internal/ui/anim"
	"github.com/ujjwal313/boombrick/internal/ui/uitest"
)

var integrations = []Integration{
	{Key: keys.SpotifyOn, Name: "Spotify", Icon: "stream", About: "Stream your playlists and liked songs · needs Premium"},
	{Key: keys.YouTubeOn, Name: "YouTube", Icon: "circleplay", About: "Search YouTube Music and download songs"},
	{Key: keys.RadioOn, Name: "Radio", Icon: "radio", About: "Stations from Radio Browser"},
}

// switchEnv is Settings with the integrations, the Spotify account and the
// Downloader card, recording the switches.
type switchEnv struct {
	*env
	acct     *fakeAccount
	dl       *fakeDownloader
	tool     *fakeReleases
	switched []string
}

func newSwitchEnv(p prefs) *switchEnv {
	e := &switchEnv{env: &env{prefs: p, p: &fakePlayer{}, space: &fakeSpace{}},
		acct: &fakeAccount{st: spotify.Status{State: spotify.Ready, Name: "Ujjwal", Premium: true}},
		dl:   &fakeDownloader{d: download.Downloader{Version: "2026.09.27", Installed: t0.Add(-18 * 24 * time.Hour)}},
		tool: &fakeReleases{r: release.Release{Tag: "2026.10.11"}}}
	e.s = New(Options{Prefs: e.prefs, Player: e.p, Space: e.space, Version: "0.4.0", Device: "TrimUI Brick",
		Card:  func() (int64, int64, int64) { return 595 * gib / 10, 386 * gib / 10, 142 * gib / 10 },
		Toast: func(m string) { e.toasts = append(e.toasts, m) }, Spotify: e.acct, Copies: true,
		Downloader: e.dl, ToolReleases: e.tool, Integrations: integrations,
		Switched: func(key string, on bool) {
			e.switched = append(e.switched, key+"="+onOff(on))
		}})
	return e
}

func TestTheIntegrationsRowComesFirst(t *testing.T) {
	e := newSwitchEnv(prefs{})
	if e.s.focus != rowIntegrations {
		t.Fatalf("focus starts on %d, want Integrations", e.s.focus)
	}
	if got := e.s.rows[rowIntegrations].Value; got != "3 of 3 on" {
		t.Fatalf("the row says %q", got)
	}
	uitest.Golden(t, "settings-integrations-row", render(t, e.env, t0))
}

func TestTheIntegrationsPageSwitches(t *testing.T) {
	e := newSwitchEnv(prefs{})
	e.s.Handle(press(input.A), t0)
	if e.s.integrations == nil {
		t.Fatal("A on the row should open the page")
	}
	uitest.Golden(t, "integrations-page", render(t, e.env, t0))
	e.s.Handle(press(input.Down), t0)
	e.s.Handle(press(input.A), t0)
	if e.prefs.Bool(keys.YouTubeOn) {
		t.Fatal("A on YouTube should switch it off")
	}
	if !slices.Equal(e.switched, []string{keys.YouTubeOn + "=Off"}) {
		t.Fatalf("switched %v", e.switched)
	}
	if got := e.s.integrations.rows[1].Value; got != "Off" {
		t.Fatalf("the YouTube row says %q", got)
	}
	uitest.Golden(t, "integrations-page-off", render(t, e.env, t0.Add(anim.Focus)))
	if e.s.Back(); e.s.integrations != nil {
		t.Fatal("B should leave the page")
	}
	if got := e.s.rows[rowIntegrations].Value; got != "2 of 3 on" {
		t.Fatalf("back on Settings the row says %q", got)
	}
	e.s.Handle(press(input.A), t0)
	e.s.Handle(press(input.Down), t0)
	e.s.Handle(press(input.A), t0)
	if !e.prefs.Bool(keys.YouTubeOn) || len(e.switched) != 2 {
		t.Fatalf("switched back on: %v", e.switched)
	}
}

// Focus stops at the page's ends.
func TestTheIntegrationsPageFocus(t *testing.T) {
	e := newSwitchEnv(prefs{})
	e.s.Handle(press(input.A), t0)
	if e.s.Handle(press(input.Up), t0) {
		t.Fatal("Up from the first row should do nothing")
	}
	e.s.Handle(press(input.Down), t0)
	e.s.Handle(press(input.Down), t0)
	if e.s.Handle(press(input.Down), t0) {
		t.Fatal("Down from the last row should do nothing")
	}
	if e.s.integrations.focus != 2 {
		t.Fatalf("focus on %d", e.s.integrations.focus)
	}
}

// What belongs to an integration switched off is hidden: Spotify's
// Streaming quality, Prefer downloaded copies and account; YouTube's
// Download quality and Downloader card, and its release check.
func TestRowsOfAnIntegrationSwitchedOffAreHidden(t *testing.T) {
	both := newSwitchEnv(prefs{})
	render(t, both.env, t0)
	for _, item := range []int{rowStream, rowPrefer, rowQuality, pillTool, pillAccount} {
		if !both.s.shown(item) {
			t.Fatalf("item %d hidden with both on", item)
		}
	}

	sp := newSwitchEnv(prefs{keys.SpotifyOn: "false"})
	render(t, sp.env, t0)
	for _, item := range []int{rowStream, rowPrefer, pillAccount} {
		if sp.s.shown(item) {
			t.Errorf("item %d shown with Spotify off", item)
		}
	}
	if !sp.s.shown(rowQuality) || !sp.s.shown(pillTool) {
		t.Error("YouTube's rows went with Spotify")
	}
	if sp.s.rows[rowIntegrations].Value != "2 of 3 on" {
		t.Errorf("the row says %q", sp.s.rows[rowIntegrations].Value)
	}

	yt := newSwitchEnv(prefs{keys.YouTubeOn: "false"})
	yt.s.Opened()
	render(t, yt.env, t0)
	for _, item := range []int{rowQuality, pillTool} {
		if yt.s.shown(item) {
			t.Errorf("item %d shown with YouTube off", item)
		}
	}
	if yt.tool.checks != 0 {
		t.Error("yt-dlp's releases were checked with YouTube off")
	}
	uitest.Golden(t, "settings-youtube-off", render(t, yt.env, t0))
}

var homeSections = []Integration{
	{Key: keys.HomeContinue, Name: "Continue listening", Icon: "play", About: "Albums you stopped part way through"},
	{Key: keys.HomeEpisodes, Name: "Podcast episodes in Continue listening", Icon: "back30", About: "Spotify episodes you've started", Attached: true},
	{Key: keys.HomeShuffle, Name: "Shuffle all songs", Icon: "shuffle", About: "Every song on your SD card, shuffled"},
	{Key: keys.HomeRandom, Name: "Random albums", Icon: "refresh", About: "Six albums, new each launch"},
	{Key: keys.HomePlaylists, Name: "Playlists", Icon: "music", About: "Your .m3u and .csv playlists"},
	{Key: keys.HomeSpotify, Name: "Spotify playlists", Icon: "stream", About: "Liked Songs and your playlists"},
	{Key: keys.HomePodcasts, Name: "Podcasts", Icon: "headphones", About: "Shows you follow on Spotify"},
	{Key: keys.HomeRadio, Name: "Radio", Icon: "radio", About: "Your saved stations"},
	{Key: keys.HomeRecent, Name: "Recently added", Icon: "download", About: "The newest albums in your library"},
}

// Home sections opens a page of switches, one per section, all on at
// first; each switch is kept and Home hears of it.
func TestTheHomeSectionsPage(t *testing.T) {
	p := prefs{}
	homes := 0
	s := New(Options{Prefs: p, Player: &fakePlayer{}, HomeSections: homeSections, HomeSwitched: func() { homes++ }})
	if !s.shown(rowHome) || s.rows[rowHome].Value != "9 of 9 on" {
		t.Fatalf("the row shown %v, says %q", s.shown(rowHome), s.rows[rowHome].Value)
	}
	for s.focus != rowHome {
		s.Handle(press(input.Down), t0)
	}
	s.Handle(press(input.A), t0)
	if s.integrations == nil || s.integrations.title != "Home sections" {
		t.Fatal("A should open the Home sections page")
	}
	for range 6 {
		s.Handle(press(input.Down), t0)
	}
	s.Handle(press(input.A), t0)
	if p.Bool(keys.HomePodcasts) || homes != 1 || s.integrations.rows[6].Value != "Off" {
		t.Fatalf("Podcasts on %v, Home told %d times", p.Bool(keys.HomePodcasts), homes)
	}
	e := &env{prefs: p, p: &fakePlayer{}, s: s}
	render(t, e, t0) // starts the scroll to the focused row
	if s.integrations.scroll.Target() <= 0 {
		t.Fatal("the page should scroll to show Podcasts")
	}
	uitest.Golden(t, "home-sections-page", render(t, e, t0.Add(anim.Focus)))
	s.Back()
	if s.rows[rowHome].Value != "8 of 9 on" {
		t.Fatalf("back on Settings the row says %q", s.rows[rowHome].Value)
	}
	if New(Options{Prefs: p, Player: &fakePlayer{}}).shown(rowHome) {
		t.Fatal("a Home sections row with no sections")
	}
}

// On Home sections, X picks a section up, Up and Down move it, and A drops
// it, saving the order; B puts it back. Podcast episodes move with Continue
// listening, and can't be picked up alone.
func TestReorderHomeSections(t *testing.T) {
	p := prefs{}
	homes := 0
	s := New(Options{Prefs: p, Player: &fakePlayer{}, HomeSections: homeSections, HomeSwitched: func() { homes++ }})
	for s.focus != rowHome {
		s.Handle(press(input.Down), t0)
	}
	s.Handle(press(input.A), t0)
	pg := s.integrations
	names := func() []string {
		var out []string
		for _, in := range pg.items {
			out = append(out, in.Name)
		}
		return out
	}
	if s.Handle(press(input.Down), t0); s.Handle(press(input.X), t0) || pg.moving {
		t.Fatal("X picked up Podcast episodes alone")
	}
	s.Handle(press(input.Up), t0)
	s.Handle(press(input.X), t0) // Continue listening, with its episodes
	if !pg.moving || pg.rows[0].Value != "Moving" || pg.rows[1].Value != "Moving" {
		t.Fatalf("moving %v, rows %v", pg.moving, pg.rows[:2])
	}
	s.Handle(press(input.Down), t0)
	s.Handle(press(input.Down), t0)
	uitest.Golden(t, "home-sections-moving", render(t, &env{prefs: p, p: &fakePlayer{}, s: s}, t0.Add(anim.Focus)))
	want := []string{"Shuffle all songs", "Random albums", "Continue listening", "Podcast episodes in Continue listening", "Playlists"}
	if got := names(); !slices.Equal(got[:5], want) || pg.focus != 2 {
		t.Fatalf("order %v, focus %d", got, pg.focus)
	}
	s.Handle(press(input.A), t0)
	if pg.moving || homes != 1 || p[keys.HomeOrder] != "home.shuffle,home.random,home.continue,home.playlists,home.spotify,home.podcasts,home.radio,home.recent" {
		t.Fatalf("dropped: moving %v, Home told %d, order %q", pg.moving, homes, p[keys.HomeOrder])
	}
	// Moved again and cancelled, it goes back.
	s.Handle(press(input.X), t0)
	s.Handle(press(input.Up), t0)
	s.Back()
	if got := names(); !slices.Equal(got[:3], want[:3]) || pg.moving || s.integrations == nil || pg.focus != 2 {
		t.Fatalf("cancelled: %v, focus %d", got, pg.focus)
	}
	// Opened again, the page keeps the order.
	s.Back()
	s.Handle(press(input.A), t0)
	pg = s.integrations
	if got := names(); !slices.Equal(got[:5], want) {
		t.Fatalf("reopened: %v", got)
	}
	if s.Handle(press(input.X), t0); s.Handle(press(input.Up), t0) {
		t.Fatal("moved above the top")
	}
}
