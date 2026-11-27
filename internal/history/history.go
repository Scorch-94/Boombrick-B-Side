// Package history counts the songs played, for Home's Recently played and
// Most played.
package history

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"github.com/ujjwal313/boombrick/internal/player"
)

// Play is a song as the history has it: enough to play it again, how many
// times it has been played and when it last was.
type Play struct {
	Track player.Track
	Count int
	Last  time.Time
}

// History keeps the songs played in the database.
type History struct{ db *sql.DB }

// New keeps the history in db.
func New(db *sql.DB) *History { return &History{db: db} }

// key is t's row: its file, or for a song streamed its Spotify track.
func key(t player.Track) string {
	if t.Path != "" {
		return t.Path
	}
	return "spotify:" + t.SpotifyID
}

// Counts reports whether t is a song the history counts: one on the card or
// streamed from Spotify, but not spoken word (as spoken says), a station or a
// song still downloading.
func Counts(t player.Track, spoken func(player.Track) bool) bool {
	if t.Live() || spoken != nil && spoken(t) {
		return false
	}
	return t.Path != "" || len(t.SpotifyID) == 22
}

// Add counts a play of t at when.
func (h *History) Add(t player.Track, when time.Time) error {
	_, err := h.db.Exec(`INSERT INTO plays (song, path, spotify, title, artist, album, cover, duration, count, last)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1, ?)
		ON CONFLICT (song) DO UPDATE SET title = excluded.title, artist = excluded.artist, album = excluded.album,
			cover = excluded.cover, duration = excluded.duration, count = count + 1, last = excluded.last`,
		key(t), t.Path, t.SpotifyID, t.Title, t.Artist, t.Album, t.Cover, t.Duration.Milliseconds(), when.UnixMilli())
	return err
}

// Recent is the n songs played last, the latest first.
func (h *History) Recent(n int) ([]Play, error) {
	return h.query(`SELECT path, spotify, title, artist, album, cover, duration, count, last FROM plays
		ORDER BY last DESC LIMIT ?`, n)
}

// Most is the n songs played most, ties going to the one played last.
func (h *History) Most(n int) ([]Play, error) {
	return h.query(`SELECT path, spotify, title, artist, album, cover, duration, count, last FROM plays
		ORDER BY count DESC, last DESC LIMIT ?`, n)
}

func (h *History) query(q string, n int) ([]Play, error) {
	rows, err := h.db.Query(q, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Play
	for rows.Next() {
		var p Play
		var d, last int64
		t := &p.Track
		if err := rows.Scan(&t.Path, &t.SpotifyID, &t.Title, &t.Artist, &t.Album, &t.Cover, &d, &p.Count, &last); err != nil {
			return nil, err
		}
		t.AlbumArtist, t.Duration, p.Last = t.Artist, time.Duration(d)*time.Millisecond, time.UnixMilli(last)
		out = append(out, p)
	}
	return out, rows.Err()
}

// Forget drops a song from the history, as when it's deleted.
func (h *History) Forget(path string) error {
	_, err := h.db.Exec(`DELETE FROM plays WHERE song = ?`, path)
	return err
}

// heard is how much of a song counts as a play: half of it, or 30 seconds
// of a longer one, as scrobblers count.
func heard(length time.Duration) time.Duration {
	if length <= 0 {
		return 30 * time.Second
	}
	return min(length/2, 30*time.Second)
}

// adder counts plays. *History is one.
type adder interface {
	Add(t player.Track, when time.Time) error
}

// counter is Watch's state: the song playing and how long it has been heard.
type counter struct {
	key     string
	index   int
	elapsed time.Duration
	heard   time.Duration
	counted bool
}

// Watch counts each song the player plays once it has been heard for a
// while (half of it, or 30 s), until ctx ends. A song heard again, from its
// start or on repeat, counts again. added, which may be nil, hears of each.
func Watch(ctx context.Context, state func() player.State, h adder, spoken func(player.Track) bool, added func(),
	log *slog.Logger) {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	var c counter
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-t.C:
			if s := state(); c.look(s, spoken) {
				if err := h.Add(s.Track, now); err != nil {
					if log != nil {
						log.Warn("play history", "err", err)
					}
					continue
				}
				if added != nil {
					added()
				}
			}
		}
	}
}

// look takes the player's state, a second after the last, and reports
// whether the song playing has now been heard enough to count.
func (c *counter) look(s player.State, spoken func(player.Track) bool) bool {
	if !s.Queued || !Counts(s.Track, spoken) {
		*c = counter{}
		return false
	}
	k := key(s.Track)
	// Another song, the same one at another place in the queue, or this one
	// started again: a new play.
	if k != c.key || s.Index != c.index || s.Elapsed+5*time.Second < c.elapsed {
		*c = counter{key: k, index: s.Index}
	}
	if s.Playing && !s.Waiting && !s.Loading {
		if step := s.Elapsed - c.elapsed; step > 0 && step <= 3*time.Second {
			c.heard += step // seeking ahead doesn't count as hearing it
		}
	}
	c.elapsed = s.Elapsed
	if !c.counted && c.heard >= heard(s.Duration) {
		c.counted = true
		return true
	}
	return false
}
