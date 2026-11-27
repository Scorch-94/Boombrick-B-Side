package audiobooks

import (
	"context"
	"log/slog"
	"time"

	"github.com/ujjwal313/boombrick/internal/player"
)

// How the place in the book playing is kept.
const (
	// look is how often the player is looked at.
	look = time.Second
	// saveEvery is how often the place is kept while a book plays; pausing,
	// moving to another part, leaving the book and quitting keep it at once.
	saveEvery = 15 * time.Second
	// finishedNear is how near the end of its last part a book left counts
	// as finished.
	finishedNear = 30 * time.Second
)

// placeSaver keeps a book's place. *Places is one.
type placeSaver interface {
	Save(book string, pl Place) error
}

// follower keeps the place in the book playing: as Follow says.
type follower struct {
	state func() player.State
	saver placeSaver
	saved func()
	log   *slog.Logger
	now   func() time.Time

	book    string // the book last seen playing, "" if none
	part    string
	elapsed time.Duration
	near    bool // at the book's last part, near its end
	done    bool // the book was heard to its end, or left near it
	playing bool
	keptAt  time.Time
}

// Follow keeps the listener's place in the audiobook playing until ctx ends:
// every 15 s while it plays, when it pauses, when it moves to another part,
// when another book or song comes on, and when ctx ends, as when Boombrick
// quits. A book left near the end of its last part is finished. saved, which
// may be nil, hears of each place kept.
func Follow(ctx context.Context, state func() player.State, places placeSaver, saved func(), log *slog.Logger) {
	f := &follower{state: state, saver: places, saved: saved, log: log, now: time.Now}
	t := time.NewTicker(look)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			f.keep(f.done)
			return
		case <-t.C:
			f.look()
		}
	}
}

// look compares what plays now with what played at the last look.
func (f *follower) look() {
	s := f.state()
	book := ""
	if s.Queued {
		book = BookOf(s.Track.Path)
	}
	if f.book != "" && book != f.book {
		f.keep(f.done || f.near) // left: as finished if it was at the end
		f.book, f.done = "", false
	}
	if book == "" {
		return
	}
	now := f.now()
	moved := f.book != book || s.Track.Path != f.part
	paused := f.playing && !s.Playing
	if moved || s.Elapsed < f.elapsed-finishedNear {
		f.done = false // listening again, or gone back
	}
	f.book, f.part, f.elapsed, f.playing = book, s.Track.Path, s.Elapsed, s.Playing
	last := s.Index == s.Count-1 && s.Duration > 0
	f.near = last && s.Duration-s.Elapsed <= finishedNear
	if paused && last && s.Duration-s.Elapsed <= 2*time.Second {
		f.done = true // the last part played to its end and stopped
	}
	if moved || paused || s.Playing && now.Sub(f.keptAt) >= saveEvery {
		f.keep(f.done)
	}
}

// keep saves the place last seen, finished or not.
func (f *follower) keep(finished bool) {
	if f.book == "" {
		return
	}
	pl := Place{Part: f.part, Position: f.elapsed, Finished: finished, Updated: f.now()}
	if err := f.saver.Save(f.book, pl); err != nil {
		if f.log != nil {
			f.log.Warn("audiobook place", "book", f.book, "err", err)
		}
		return
	}
	f.keptAt = pl.Updated
	if f.saved != nil {
		f.saved()
	}
}
