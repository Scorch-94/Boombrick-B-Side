// Package remote takes play, pause and skip commands from Bluetooth
// headphones and speakers, as their taps and buttons send them over AVRCP,
// and gives them to the player.
//
// BlueZ hands such a command to the media player registered with it, so
// Boombrick registers one (see bluez.go) and tells it what is playing:
// headphones send Play or Pause by what they are told, and pause by
// themselves when taken out of an ear only while they know something plays.
// With no player registered, BlueZ types the command on a keyboard of its
// own instead, which keys.go reads, so the buttons work either way.
package remote

import (
	"context"
	"github.com/ujjwal313/boombrick/internal/audiobooks"
	"log/slog"
	"sync"
	"time"

	"github.com/ujjwal313/boombrick/internal/player"
	"github.com/ujjwal313/boombrick/internal/spotify"
)

// Player is what the commands drive. *player.Player is one.
type Player interface {
	State() player.State
	Toggle()
	Next()
	Prev()
	Seek(d time.Duration)
}

// Command is one of the commands headphones send.
type Command string

const (
	Play      Command = "Play"
	Pause     Command = "Pause"
	PlayPause Command = "PlayPause"
	Stop      Command = "Stop"
	Next      Command = "Next"
	Previous  Command = "Previous"
	Forward   Command = "FastForward"
	Rewind    Command = "Rewind"
)

// episodeSkip is how far Next and Previous jump in a podcast episode, as Left
// and Right do on Now Playing.
const episodeSkip = 30 * time.Second

// echo is how soon the same command coming the other way, as both BlueZ's
// player and its keyboard, is taken for the same press.
const echo = 300 * time.Millisecond

// Remote gives the commands to the player, logging each so that what the
// headphones send can be read in the log.
type Remote struct {
	p   Player
	log *slog.Logger
	now func() time.Time

	mu   sync.Mutex
	last Command
	at   time.Time
}

// New returns a Remote driving p.
func New(p Player, log *slog.Logger) *Remote {
	return &Remote{p: p, log: log, now: time.Now}
}

// Start listens for commands until ctx ends: as BlueZ's media player, and on
// the keyboards BlueZ makes for headphones. It returns at once; done closes
// when both have stopped.
func (r *Remote) Start(ctx context.Context) (done <-chan struct{}) {
	ch := make(chan struct{})
	var wg sync.WaitGroup
	wg.Go(func() { runBluez(ctx, r) })
	wg.Go(func() { runKeys(ctx, r) })
	go func() { wg.Wait(); close(ch) }()
	return ch
}

// Do carries out c, from where it came, for the log. Play and Pause only
// change what needs changing, so a Pause while paused does nothing.
func (r *Remote) Do(c Command, from string) {
	now := r.now()
	r.mu.Lock()
	again := c == r.last && now.Sub(r.at) < echo
	r.last, r.at = c, now
	r.mu.Unlock()
	if again {
		r.log.Info("bluetooth remote: repeat ignored", "command", string(c), "from", from)
		return
	}
	s := r.p.State()
	r.log.Info("bluetooth remote", "command", string(c), "from", from, "playing", s.Playing)
	if !s.Queued {
		return
	}
	ep := isEpisode(s.Track)
	switch c {
	case Play:
		if !s.Playing {
			r.p.Toggle()
		}
	case Pause, Stop:
		if s.Playing {
			r.p.Toggle()
		}
	case PlayPause:
		r.p.Toggle()
	case Next, Forward:
		if ep {
			r.p.Seek(min(s.Elapsed+episodeSkip, max(s.Duration-time.Second, 0)))
			return
		}
		r.p.Next()
	case Previous, Rewind:
		if ep {
			r.p.Seek(max(s.Elapsed-episodeSkip, 0))
			return
		}
		r.p.Prev()
	}
}

func isEpisode(t player.Track) bool {
	_, ok := spotify.IsEpisode(t.SpotifyID)
	return ok || audiobooks.IsBook(t.Path)
}
