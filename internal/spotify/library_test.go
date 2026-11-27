package spotify

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/ujjwal313/boombrick/internal/store"
)

// fakeClient answers from its fields and counts what it is asked.
type fakeClient struct {
	releases  []Release
	mu        sync.Mutex
	status    Status
	playlists []Playlist
	tracks    map[string][]Track
	shows     []Show
	episodes  map[string][]Episode
	err       error // every list's answer, when set
	loginErr  error // a sign-in's answer, when set
	asked     map[string]int
}

func (f *fakeClient) count(what string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.asked[what]++
}

func (f *fakeClient) times(what string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.asked[what]
}

func (f *fakeClient) set(fn func()) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fn()
}

func (f *fakeClient) Status(context.Context) (Status, error) {
	f.count("status")
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.status, nil
}

func (f *fakeClient) Login(context.Context) (Login, error) {
	f.count("login")
	f.mu.Lock()
	err := f.loginErr
	f.mu.Unlock()
	if err != nil {
		return Login{}, err
	}
	f.set(func() {
		f.status = Status{State: SigningIn, Login: &Login{URL: "https://spotify.com/pair?code=ABCD", Code: "ABCD"}}
	})
	return Login{}, nil
}

func (f *fakeClient) Logout(context.Context) error {
	f.count("logout")
	f.set(func() { f.status = Status{State: SignedOut} })
	return nil
}

func (f *fakeClient) Playlists(context.Context) ([]Playlist, error) {
	f.count(playlistsKey)
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.playlists, f.err
}

func (f *fakeClient) Tracks(_ context.Context, uri string) ([]Track, error) {
	f.count(uri)
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.tracks[uri], f.err
}

func (f *fakeClient) Search(_ context.Context, q string) (Results, error) {
	f.count(searchKey + q)
	return Results{Artists: []Artist{{Name: q}}}, f.err
}

func (f *fakeClient) Releases(context.Context) ([]Release, error) {
	f.count(releasesKey)
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.releases, f.err
}

func (f *fakeClient) Shows(context.Context) ([]Show, error) {
	f.count(showsKey)
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.shows, f.err
}

func (f *fakeClient) Episodes(_ context.Context, uri string) ([]Episode, error) {
	f.count(uri)
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.episodes[uri], f.err
}

var (
	gym   = []Playlist{{URI: "spotify:playlist:5ABHKGoOzxkaa28ttQV9sE", Name: "Gym", Owner: "Ujjwal", Count: 1}}
	songs = []Track{{ID: "3UmaczJpikHgJFyBTAJVoz", Title: "Stan", Artists: []string{"Eminem", "Dido"}, Duration: 404000}}
)

func openDB(t *testing.T) *sql.DB {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "boombrick.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s.DB
}

// clock is a time the test moves on.
type clock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *clock) time() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *clock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// library makes a Library on db with a clock the test moves.
func library(t *testing.T, c Client, db *sql.DB) (*Library, *clock) {
	t.Helper()
	l := NewLibrary(c, db, slog.New(slog.NewTextHandler(io.Discard, nil)))
	clk := &clock{now: time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC)}
	l.now = clk.time
	t.Cleanup(l.Close)
	return l, clk
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	for deadline := time.Now().Add(2 * time.Second); !cond(); time.Sleep(time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
	}
}

// Lists come from the helper the first time, then from the store, without
// asking the helper again while they are fresh.
func TestListsAreKept(t *testing.T) {
	db := openDB(t)
	c := &fakeClient{playlists: gym, tracks: map[string][]Track{LikedURI: songs}, asked: map[string]int{}}
	l, _ := library(t, c, db)
	if _, have, _ := l.Playlists(); have {
		t.Fatal("playlists known before anything was asked")
	}
	waitFor(t, "the playlists", func() bool { _, have, _ := l.Playlists(); return have })
	waitFor(t, "Liked Songs", func() bool { _, have, _ := l.Tracks(LikedURI); return have })
	if ps, _, _ := l.Playlists(); !reflect.DeepEqual(ps, gym) {
		t.Fatalf("playlists %+v", ps)
	}

	c2 := &fakeClient{err: ErrOffline, asked: map[string]int{}}
	l2, _ := library(t, c2, db)
	waitFor(t, "the kept playlists", func() bool { _, have, _ := l2.Playlists(); return have })
	waitFor(t, "the kept tracks", func() bool { _, have, _ := l2.Tracks(LikedURI); return have })
	if ts, _, err := l2.Tracks(LikedURI); !reflect.DeepEqual(ts, songs) || err != nil {
		t.Fatalf("tracks %+v, %v", ts, err)
	}
	time.Sleep(20 * time.Millisecond)
	if n := c2.times(playlistsKey) + c2.times(LikedURI); n != 0 {
		t.Fatalf("asked the helper %d times for fresh lists", n)
	}
}

// Podcasts are kept like playlists: the shows and each list of episodes,
// read back from the store after a restart. Continue listening is asked
// for again after a minute, others after ten, and Stale asks at once.
func TestPodcastListsAreKept(t *testing.T) {
	db := openDB(t)
	owls := []Show{{URI: "spotify:show:1", Name: "Night Owls", Publisher: "Owl FM"}}
	ep := []Episode{{ID: "e1", Title: "Ep 12", Show: "Night Owls", Duration: 3_600_000, Position: 60_000}}
	c := &fakeClient{shows: owls, episodes: map[string][]Episode{ContinueURI: ep, "spotify:show:1": ep}, asked: map[string]int{}}
	l, now := library(t, c, db)
	waitFor(t, "the shows", func() bool { _, have, _ := l.Shows(); return have })
	waitFor(t, "continue", func() bool { _, have, _ := l.Episodes(ContinueURI); return have })
	waitFor(t, "a show", func() bool { _, have, _ := l.Episodes("spotify:show:1"); return have })
	if ss, _, _ := l.Shows(); !reflect.DeepEqual(ss, owls) {
		t.Fatalf("shows %+v", ss)
	}
	now.advance(2 * time.Minute)
	l.Episodes(ContinueURI)
	l.Episodes("spotify:show:1")
	waitFor(t, "continue again", func() bool { return c.times(ContinueURI) == 2 })
	time.Sleep(20 * time.Millisecond)
	if n := c.times("spotify:show:1"); n != 1 {
		t.Fatalf("a show asked %d times within ten minutes", n)
	}
	l.Stale("spotify:show:1")
	l.Episodes("spotify:show:1")
	waitFor(t, "the stale show", func() bool { return c.times("spotify:show:1") == 2 })

	c2 := &fakeClient{err: ErrOffline, asked: map[string]int{}}
	l2, _ := library(t, c2, db)
	waitFor(t, "the kept shows", func() bool { _, have, _ := l2.Shows(); return have })
	waitFor(t, "the kept episodes", func() bool { _, have, _ := l2.Episodes("spotify:show:1"); return have })
	if es, _, _ := l2.Episodes("spotify:show:1"); !reflect.DeepEqual(es, ep) {
		t.Fatalf("kept episodes %+v", es)
	}
}

// After ten minutes a list is asked for again; one that can't be had keeps
// what was had, with the error, and isn't asked again for a while unless
// Retry says so.
func TestOldListsAreAskedAgain(t *testing.T) {
	c := &fakeClient{playlists: gym, asked: map[string]int{}}
	l, now := library(t, c, openDB(t))
	waitFor(t, "the playlists", func() bool { _, have, _ := l.Playlists(); return have })
	c.set(func() { c.err = ErrOffline })
	now.advance(11 * time.Minute)
	waitFor(t, "the second asking", func() bool { _, _, err := l.Playlists(); return err == ErrOffline })
	if ps, have, _ := l.Playlists(); !have || !reflect.DeepEqual(ps, gym) {
		t.Fatalf("offline, the playlists are %+v, %v", ps, have)
	}
	for range 10 {
		l.Playlists()
	}
	time.Sleep(20 * time.Millisecond)
	if n := c.times(playlistsKey); n != 2 {
		t.Fatalf("asked %d times, failing", n)
	}
	c.set(func() { c.err = nil })
	l.Retry()
	waitFor(t, "the retry", func() bool { _, _, err := l.Playlists(); return err == nil })
}

// Only the track lists in use stay in memory: the four asked for last, and
// those asked for within a minute. The others are read from the store again
// when next asked for, without asking the helper.
func TestTrackListsNotInUseAreDropped(t *testing.T) {
	uris := []string{LikedURI}
	for i := range 7 {
		uris = append(uris, "spotify:playlist:"+strconv.Itoa(i))
	}
	c := &fakeClient{tracks: map[string][]Track{}, asked: map[string]int{}}
	for _, u := range uris {
		c.tracks[u] = songs
	}
	l, now := library(t, c, openDB(t))
	ask := func(u string) {
		t.Helper()
		v := l.Version()
		waitFor(t, u, func() bool { _, have, _ := l.Tracks(u); return have })
		if l.Version() == v {
			t.Fatalf("the version didn't change with %s", u)
		}
	}
	held := func() []string {
		l.mu.Lock()
		defer l.mu.Unlock()
		var out []string
		for k, e := range l.lists {
			if e.have {
				out = append(out, k)
			}
		}
		slices.Sort(out)
		return out
	}
	for _, u := range uris[:7] {
		ask(u)
		now.advance(time.Second)
	}
	if got := held(); len(got) != 7 {
		t.Fatalf("within a minute, kept %q", got)
	}
	now.advance(2 * idle)
	l.Tracks(uris[1])
	ask(uris[7])
	want := []string{uris[1], uris[5], uris[6], uris[7]}
	slices.Sort(want)
	if got := held(); !slices.Equal(got, want) {
		t.Fatalf("later, kept %q", got)
	}
	ask(uris[0])
	if ts, _, _ := l.Tracks(uris[0]); !reflect.DeepEqual(ts, songs) || c.times(uris[0]) != 1 {
		t.Fatalf("read again %+v, asking the helper %d times", ts, c.times(uris[0]))
	}
}

// Saved shows what is kept without asking the helper.
func TestSavedDoesntAskTheHelper(t *testing.T) {
	db := openDB(t)
	c := &fakeClient{playlists: gym, asked: map[string]int{}}
	l, _ := library(t, c, db)
	waitFor(t, "the playlists", func() bool { _, have, _ := l.Playlists(); return have })

	c2 := &fakeClient{asked: map[string]int{}}
	l2, _ := library(t, c2, db)
	waitFor(t, "the kept playlists", func() bool { _, have := l2.Saved(); return have })
	if ps, _ := l2.Saved(); !reflect.DeepEqual(ps, gym) {
		t.Fatalf("saved %+v", ps)
	}
	if n := c2.times(playlistsKey) + c2.times("status"); n != 0 {
		t.Fatalf("asked the helper %d times", n)
	}
}

// The status is asked once a second; signing in and out ask it again at
// once, and signing out forgets the lists.
// A sign-in that fails says why until the next one, though the helper's
// status, asked again meanwhile, has no error.
func TestFailedSignInIsKept(t *testing.T) {
	db := openDB(t)
	c := &fakeClient{status: Status{State: SignedOut}, loginErr: ErrOffline, asked: map[string]int{}}
	l, _ := library(t, c, db)
	l.Login()
	waitFor(t, "the failure", func() bool { return errors.Is(l.LoginErr(), ErrOffline) })
	waitFor(t, "the status after it", func() bool { st, err := l.Status(); return st.State == SignedOut && err == nil })
	if !errors.Is(l.LoginErr(), ErrOffline) {
		t.Fatalf("after the status, the failure is %v", l.LoginErr())
	}
	c.set(func() { c.loginErr = nil })
	l.Login()
	if err := l.LoginErr(); err != nil {
		t.Fatalf("trying again still says %v", err)
	}
	waitFor(t, "the code", func() bool { st, _ := l.Status(); return st.Login != nil })
}

func TestStatusSignInAndOut(t *testing.T) {
	db := openDB(t)
	c := &fakeClient{status: Status{State: SignedOut}, playlists: gym, asked: map[string]int{}}
	l, now := library(t, c, db)
	waitFor(t, "the status", func() bool { st, _ := l.Status(); return st.State == SignedOut })
	for range 10 {
		l.Status()
	}
	if n := c.times("status"); n != 1 {
		t.Fatalf("asked the status %d times within a second", n)
	}
	l.Login()
	waitFor(t, "the code", func() bool { st, _ := l.Status(); return st.State == SigningIn && st.Login != nil })

	c.set(func() {
		c.status = Status{State: Ready, User: "bmfjclzv7pstc3zzlq0nteyni", Name: "Ujjwal", Premium: true}
	})
	now.advance(statusEvery)
	waitFor(t, "signed in", func() bool { st, _ := l.Status(); return st.State == Ready && st.Name == "Ujjwal" })
	waitFor(t, "the playlists", func() bool { _, have, _ := l.Playlists(); return have })

	l.Logout()
	waitFor(t, "signed out", func() bool { st, _ := l.Status(); return st.State == SignedOut })
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM spotify_cache`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("%d lists kept after signing out, %v", n, err)
	}
	if _, have := l.Saved(); have {
		t.Fatal("the playlists are still known after signing out")
	}
}

// slowClient holds the playlists' answer until the test lets it go.
type slowClient struct {
	*fakeClient
	waiting, answer chan struct{}
}

func (s *slowClient) Playlists(ctx context.Context) ([]Playlist, error) {
	s.waiting <- struct{}{}
	<-s.answer
	return s.fakeClient.Playlists(ctx)
}

// A list asked for before signing out and answered after it is the old
// account's: it is neither kept nor shown.
func TestListsAnsweredAfterSigningOutAreDropped(t *testing.T) {
	db := openDB(t)
	c := &slowClient{fakeClient: &fakeClient{playlists: gym, asked: map[string]int{}},
		waiting: make(chan struct{}), answer: make(chan struct{})}
	l, _ := library(t, c, db)
	l.Playlists()
	<-c.waiting
	l.forget()
	close(c.answer)
	l.Close()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM spotify_cache`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("%d lists kept after signing out, %v", n, err)
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if e := l.lists[playlistsKey]; e != nil && e.have {
		t.Fatal("the old account's playlists are shown")
	}
}

// Home's episodes in progress come from the store while the helper isn't
// running, without asking it; once it is, they are asked for when due.
func TestInProgressStartsNothing(t *testing.T) {
	db := openDB(t)
	ep := []Episode{{ID: "e1", Title: "Ep 12", Show: "Night Owls", Duration: 60_000, Position: 30_000, PlayedAt: 1}}
	c := &fakeClient{status: Status{State: Ready}, episodes: map[string][]Episode{ContinueURI: ep}, asked: map[string]int{}}
	l, now := library(t, c, db)
	waitFor(t, "continue", func() bool { _, have, _ := l.Episodes(ContinueURI); return have })

	c2 := &fakeClient{status: Status{State: Ready}, episodes: map[string][]Episode{ContinueURI: ep}, asked: map[string]int{}}
	l2, _ := library(t, c2, db)
	waitFor(t, "the kept episodes", func() bool { _, have := l2.InProgress(); return have })
	time.Sleep(20 * time.Millisecond)
	if n := c2.times(ContinueURI) + c2.times("status"); n != 0 {
		t.Fatalf("Home asked the helper %d times before it was running", n)
	}
	if es, _ := l2.InProgress(); !reflect.DeepEqual(es, ep) {
		t.Fatalf("kept %+v", es)
	}

	l.Status()
	waitFor(t, "the status", func() bool { st, _ := l.Status(); return st.State == Ready })
	now.advance(2 * time.Minute)
	l.InProgress()
	waitFor(t, "continue again, the helper running", func() bool { return c.times(ContinueURI) == 2 })
}
