package main

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/ujjwal313/boombrick/internal/player"
	"github.com/ujjwal313/boombrick/internal/spotify"
)

// How a podcast episode's place is kept in step with Spotify.
const (
	// episodeLook is how often the player is looked at.
	episodeLook = 2 * time.Second
	// episodeSaveEvery is how often the place is sent while an episode
	// plays; a pause sends it at once.
	episodeSaveEvery = 30 * time.Second
	// episodeMoved is how far the place has to have moved to be sent again.
	episodeMoved = 5 * time.Second
	// episodeFinished is how near its end an episode left counts as played
	// through.
	episodeFinished = 30 * time.Second
)

// positionSaver sends a place in an episode to Spotify. *spotify.Helper is
// one.
type positionSaver interface {
	SavePosition(ctx context.Context, id string, ms int64, finished bool) error
}

// episodeFollower keeps Spotify told where the user is in the podcast
// episode playing: every 30 s while it plays, when it pauses, and when it
// is left, as played through once it got near its end. Other devices then
// carry on from there, and Continue listening has it.
type episodeFollower struct {
	state func() player.State
	saver positionSaver
	saved func() // after a place is sent: Continue listening is out of date; may be nil
	log   *slog.Logger
	now   func() time.Time
	wg    sync.WaitGroup

	id       string // the episode last seen, "" if none
	elapsed  time.Duration
	duration time.Duration
	playing  bool
	sentAt   time.Time
	sent     time.Duration // the place last sent
}

// followEpisodes follows the player until ctx ends, sending the place one
// last time then, and waits for the sends under way.
func followEpisodes(ctx context.Context, p *player.Player, saver positionSaver, saved func(), log *slog.Logger) {
	f := &episodeFollower{state: p.State, saver: saver, saved: saved, log: log, now: time.Now}
	t := time.NewTicker(episodeLook)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			f.leave()
			f.wg.Wait()
			return
		case <-t.C:
			f.look()
		}
	}
}

// look compares what plays now with what played at the last look.
func (f *episodeFollower) look() {
	s := f.state()
	id, ok := spotify.IsEpisode(s.Track.SpotifyID)
	if !ok || s.Track.Path != "" {
		id = ""
	}
	if f.id != "" && id != f.id {
		f.leave()
	}
	if id == "" {
		f.id = ""
		return
	}
	now := f.now()
	if id != f.id {
		f.id, f.sentAt, f.sent = id, now, s.Elapsed // just started: nothing new to send yet
	}
	f.elapsed, f.duration = s.Elapsed, s.Duration
	moved := s.Elapsed - f.sent
	if moved < 0 {
		moved = -moved
	}
	paused := f.playing && !s.Playing
	if moved >= episodeMoved && (paused || s.Playing && now.Sub(f.sentAt) >= episodeSaveEvery) {
		f.send(id, s.Elapsed, false)
		f.sentAt, f.sent = now, s.Elapsed
	}
	f.playing = s.Playing
}

// leave sends the place in the episode followed as it is left: played
// through when it got near the end.
func (f *episodeFollower) leave() {
	if f.id == "" {
		return
	}
	done := f.duration > 0 && f.elapsed >= f.duration-episodeFinished
	if done || f.elapsed != f.sent {
		f.send(f.id, f.elapsed, done)
	}
	f.id, f.playing = "", false
}

// send tells Spotify, on a goroutine of its own, so the player isn't held
// up by the network.
func (f *episodeFollower) send(id string, at time.Duration, finished bool) {
	f.wg.Go(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := f.saver.SavePosition(ctx, id, at.Milliseconds(), finished); err != nil {
			f.log.Warn("save episode place", "id", id, "err", err)
			return
		}
		if f.saved != nil {
			f.saved()
		}
	})
}
