// Package settings stores user preferences in the settings table and falls
// back to their defaults. Reads come from memory,
// and writes reach the database from a goroutine of their own, so the UI
// thread never waits for the SD card.
package settings

import (
	"database/sql"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"sync"
)

// Keys of every persisted setting.
const (
	StreamingKbps    = "spotify.streaming_kbps"
	DownloadQuality  = "downloads.quality"
	PreferDownloaded = "playback.prefer_downloaded"
	VolumeLevelling  = "playback.volume_levelling"
	// AlbumView is how the Library shows albums: AlbumGrid, AlbumLarge or
	// AlbumList.
	AlbumView = "library.album_view"
	// Clock is the status bar clock's format: Clock24 or Clock12.
	Clock = "display.clock"
	// Home's sections, each shown or not, and whether Continue listening
	// includes podcast episodes.
	HomeContinue  = "home.continue"
	HomeEpisodes  = "home.continue_episodes"
	HomeRandom    = "home.random"
	HomeShuffle   = "home.shuffle"
	HomePlaylists = "home.playlists"
	HomeSpotify   = "home.spotify"
	HomePodcasts  = "home.podcasts"
	HomeRadio     = "home.radio"
	HomeRecent    = "home.recent"
	// HomeOrder is the order of Home's sections: their keys, comma
	// separated. See Order.
	HomeOrder = "home.order"
	// The integrations' switches: an integration switched off is hidden and
	// does no work, but keeps its data.
	SpotifyOn = "integrations.spotify"
	YouTubeOn = "integrations.youtube"
	RadioOn   = "integrations.radio"
)

// The Library's album views.
const (
	AlbumGrid  = "grid"  // four by two, small covers
	AlbumLarge = "large" // three across, big covers: the original
	AlbumList  = "list"  // rows, as the songs and artists show
)

// The clock's formats.
const (
	Clock24 = "24" // 14:30
	Clock12 = "12" // 2:30 PM
)

// Defaults holds the value used for a key that has never been set.
var Defaults = map[string]string{
	StreamingKbps:    "160",
	DownloadQuality:  "best",
	PreferDownloaded: "true",
	VolumeLevelling:  "true",
	AlbumView:        AlbumGrid,
	Clock:            Clock24,
	SpotifyOn:        "true",
	YouTubeOn:        "true",
	RadioOn:          "true",
	HomeContinue:     "true",
	HomeEpisodes:     "true",
	HomeRandom:       "true",
	HomeShuffle:      "true",
	HomePlaylists:    "true",
	HomeSpotify:      "true",
	HomePodcasts:     "true",
	HomeRadio:        "true",
	HomeRecent:       "true",
}

// Settings keeps the table in memory and writes changes behind the caller.
type Settings struct {
	db   *sql.DB
	log  *slog.Logger
	wake chan struct{}
	stop chan struct{}
	done chan struct{}
	once sync.Once

	mu     sync.RWMutex
	values map[string]string
	dirty  map[string]bool // set since they were last written
}

// Load reads every stored setting and starts the writer; Close stops it.
func Load(db *sql.DB, log *slog.Logger) (*Settings, error) {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	rows, err := db.Query(`SELECT key, value FROM settings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	s := &Settings{db: db, log: log, wake: make(chan struct{}, 1), stop: make(chan struct{}), done: make(chan struct{}),
		values: map[string]string{}, dirty: map[string]bool{}}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		s.values[k] = v
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	go s.writer()
	return s, nil
}

// String returns the stored value or the default.
func (s *Settings) String(key string) string {
	s.mu.RLock()
	v, ok := s.values[key]
	s.mu.RUnlock()
	if !ok {
		return Defaults[key]
	}
	return v
}

// Int returns the setting as an integer, or the default if the stored value
// isn't a number.
func (s *Settings) Int(key string) int {
	if n, err := strconv.Atoi(s.String(key)); err == nil {
		return n
	}
	n, _ := strconv.Atoi(Defaults[key])
	return n
}

// Bool returns the setting as a boolean, or the default if the stored value
// isn't one.
func (s *Settings) Bool(key string) bool {
	if b, err := strconv.ParseBool(s.String(key)); err == nil {
		return b
	}
	b, _ := strconv.ParseBool(Defaults[key])
	return b
}

// Set stores a value. Reads see it at once; the database a moment later.
func (s *Settings) Set(key, value string) {
	s.mu.Lock()
	s.values[key] = value
	s.dirty[key] = true
	s.mu.Unlock()
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// SetBool stores a boolean.
func (s *Settings) SetBool(key string, v bool) { s.Set(key, strconv.FormatBool(v)) }

// Close writes what is waiting and stops the writer. Settings set after it
// stay in memory only.
func (s *Settings) Close() {
	s.once.Do(func() {
		close(s.stop)
		<-s.done
	})
}

func (s *Settings) writer() {
	defer close(s.done)
	for {
		select {
		case <-s.wake:
			s.flush()
		case <-s.stop:
			s.flush()
			return
		}
	}
}

// flush writes the settings changed since the last flush, with their values
// as they are now. One that fails to be written waits for the next flush.
func (s *Settings) flush() {
	s.mu.Lock()
	now := make(map[string]string, len(s.dirty))
	for k := range s.dirty {
		now[k] = s.values[k]
	}
	clear(s.dirty)
	s.mu.Unlock()
	var failed []string
	for k, v := range now {
		if _, err := s.db.Exec(`INSERT INTO settings (key, value) VALUES (?, ?)
			ON CONFLICT (key) DO UPDATE SET value = excluded.value`, k, v); err != nil {
			s.log.Warn("save a setting", "key", k, "err", err)
			failed = append(failed, k)
		}
	}
	if len(failed) == 0 {
		return
	}
	s.mu.Lock()
	for _, k := range failed {
		s.dirty[k] = true // with its value as it will be then
	}
	s.mu.Unlock()
}

// HomeSections are Home's sections' keys in their first order, top to bottom.
// Podcast episodes go with Continue listening, so have no place of their
// own.
var HomeSections = []string{HomeContinue, HomeShuffle, HomeRandom, HomePlaylists, HomeSpotify, HomePodcasts,
	HomeRadio, HomeRecent}

// Order is Home's sections in the order saved, a HomeOrder value. Keys it
// doesn't know are dropped, and a section it doesn't name, as one added in a
// later version, goes after the one before it in HomeSections.
func Order(saved string) []string {
	var out []string
	for _, k := range strings.Split(saved, ",") {
		if k = strings.TrimSpace(k); slices.Contains(HomeSections, k) && !slices.Contains(out, k) {
			out = append(out, k)
		}
	}
	for i, k := range HomeSections {
		if slices.Contains(out, k) {
			continue
		}
		at := 0
		if i > 0 {
			at = slices.Index(out, HomeSections[i-1]) + 1
		}
		out = slices.Insert(out, at, k)
	}
	return out
}
