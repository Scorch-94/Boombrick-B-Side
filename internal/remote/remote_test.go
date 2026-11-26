package remote

import (
	"encoding/binary"
	"io"
	"log/slog"
	"slices"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"

	"github.com/ujjwal313/boombrick/internal/player"
)

type fakePlayer struct {
	s     player.State
	calls []string
}

func (p *fakePlayer) State() player.State { return p.s }
func (p *fakePlayer) Toggle()             { p.calls = append(p.calls, "toggle"); p.s.Playing = !p.s.Playing }
func (p *fakePlayer) Next()               { p.calls = append(p.calls, "next") }
func (p *fakePlayer) Prev()               { p.calls = append(p.calls, "prev") }
func (p *fakePlayer) Seek(d time.Duration) {
	p.calls = append(p.calls, "seek "+d.String())
}

func newRemote(p *fakePlayer) (*Remote, *time.Time) {
	r := New(p, slog.New(slog.NewTextHandler(io.Discard, nil)))
	now := time.Date(2026, 11, 25, 17, 0, 0, 0, time.UTC)
	r.now = func() time.Time { return now }
	return r, &now
}

// Taps on a song: Play and Pause only change what needs changing, PlayPause
// toggles, and the double and triple taps skip.
func TestCommands(t *testing.T) {
	p := &fakePlayer{s: player.State{Queued: true, Track: player.Track{Path: "/m/a.flac"}}}
	r, now := newRemote(p)
	for _, c := range []Command{Play, Play, Pause, Pause, PlayPause, Stop, Next, Previous} {
		r.Do(c, "test")
		*now = now.Add(time.Second)
	}
	want := []string{"toggle", "toggle", "toggle", "toggle", "next", "prev"}
	if !slices.Equal(p.calls, want) {
		t.Fatalf("calls %v, want %v", p.calls, want)
	}
}

// On a podcast episode, the skips jump 30 seconds, as on Now Playing.
func TestEpisodeSkips(t *testing.T) {
	p := &fakePlayer{s: player.State{Queued: true, Track: player.Track{SpotifyID: "episode:abc"},
		Elapsed: 10 * time.Second, Duration: 20 * time.Minute}}
	r, now := newRemote(p)
	r.Do(Next, "test")
	*now = now.Add(time.Second)
	r.Do(Previous, "test")
	if want := []string{"seek 40s", "seek 0s"}; !slices.Equal(p.calls, want) {
		t.Fatalf("calls %v, want %v", p.calls, want)
	}
}

// One press reaching the app both ways, or with nothing queued, does
// nothing more.
func TestEchoAndEmptyQueue(t *testing.T) {
	p := &fakePlayer{s: player.State{Queued: true}}
	r, now := newRemote(p)
	r.Do(PlayPause, "headphones")
	r.Do(PlayPause, "key")
	*now = now.Add(time.Second)
	r.Do(PlayPause, "headphones")
	if len(p.calls) != 2 {
		t.Fatalf("calls %v", p.calls)
	}
	empty := &fakePlayer{}
	r, _ = newRemote(empty)
	r.Do(Next, "test")
	if len(empty.calls) != 0 {
		t.Fatalf("with nothing queued: %v", empty.calls)
	}
}

// event is an input_event of 64-bit Linux.
func event(typ, code uint16, value int32) []byte {
	b := make([]byte, 24)
	binary.LittleEndian.PutUint16(b[16:], typ)
	binary.LittleEndian.PutUint16(b[18:], code)
	binary.LittleEndian.PutUint32(b[20:], uint32(value))
	return b
}

func TestKeyCommand(t *testing.T) {
	for _, tc := range []struct {
		ev   []byte
		want Command
		ok   bool
	}{
		{event(1, keyPlayPause, 1), PlayPause, true},
		{event(1, keyPlayCD, 1), Play, true},
		{event(1, keyPauseCD, 1), Pause, true},
		{event(1, keyNextSong, 1), Next, true},
		{event(1, keyPreviousSong, 1), Previous, true},
		{event(1, keyNextSong, 0), "", false},            // released
		{event(1, keyNextSong, 2), "", false},            // repeating
		{event(0, 0, 0), "", false},                      // a sync
		{event(1, 30, 1), "", false},                     // A, not a media key
		{event(1, keyPlayPause, 1)[8:], PlayPause, true}, // 32-bit
	} {
		if c, ok := keyCommand(tc.ev); c != tc.want || ok != tc.ok {
			t.Errorf("% x: %q %v, want %q %v", tc.ev, c, ok, tc.want, tc.ok)
		}
	}
}

// BlueZ is told what plays: the status headphones go by, and the track.
func TestProperties(t *testing.T) {
	s := player.State{Queued: true, Playing: true, Track: player.Track{Title: "Neon", Artist: "Kenji Arata", Album: "Night"},
		Duration: 4 * time.Minute, Elapsed: 30 * time.Second, Index: 2, Count: 9, Repeat: player.RepeatAll}
	ps := properties(s)
	if ps["PlaybackStatus"].Value != "Playing" || ps["LoopStatus"].Value != "Playlist" || ps["Position"].Value != int64(30e6) {
		t.Fatalf("status %v, loop %v, position %v", ps["PlaybackStatus"].Value, ps["LoopStatus"].Value, ps["Position"].Value)
	}
	m := ps["Metadata"].Value.(map[string]dbus.Variant)
	if m["xesam:title"].Value() != "Neon" || !slices.Equal(m["xesam:artist"].Value().([]string), []string{"Kenji Arata"}) ||
		m["mpris:length"].Value() != int64(240e6) || m["mpris:trackid"].Value() != dbus.ObjectPath("/org/boombrick/track/t2") {
		t.Fatalf("metadata %v", m)
	}
	if status(player.State{Queued: true}) != "Paused" || status(player.State{}) != "Stopped" {
		t.Fatal("status")
	}
}

// What changes is passed on: the status on a pause, the track on a skip,
// and the position only when it jumps from where BlueZ counts it.
func TestChanges(t *testing.T) {
	a := player.State{Queued: true, Playing: true, Track: player.Track{Title: "Neon"}, Duration: time.Minute}
	b := a
	b.Playing = false
	if got := changed(shownOf(a), shownOf(b)); !slices.Equal(got, []string{"PlaybackStatus"}) {
		t.Fatalf("paused: %v", got)
	}
	c := a
	c.Track.Title, c.Index = "Rust", 1
	if got := changed(shownOf(a), shownOf(c)); !slices.Equal(got, []string{"Metadata"}) {
		t.Fatalf("skipped: %v", got)
	}
	t0 := time.Now()
	told := position{elapsed: 10 * time.Second, at: t0, playing: true}
	if told.jumped(player.State{Elapsed: 15 * time.Second}, t0.Add(5*time.Second)) {
		t.Fatal("playing on is no jump")
	}
	if !told.jumped(player.State{Elapsed: 45 * time.Second}, t0.Add(5*time.Second)) {
		t.Fatal("a seek is a jump")
	}
	paused := position{elapsed: 10 * time.Second, at: t0}
	if paused.jumped(player.State{Elapsed: 10 * time.Second}, t0.Add(time.Minute)) {
		t.Fatal("paused, the position stays")
	}
}
