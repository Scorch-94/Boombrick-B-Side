package library

import (
	"bytes"
	"encoding/csv"
	"io"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// csvRow is one song of a .csv playlist, as exported by Exportify, Soundiiz,
// TuneMyMusic and the like: by its Spotify track where it has one, by title
// and artists, or by a file's path. A TuneMyMusic file can hold several
// playlists, each row naming its own.
type csvRow struct {
	spotifyID string
	title     string
	album     string
	artist    string        // as written, for showing
	artists   []string      // each artist, to match by
	duration  time.Duration // 0 if not given
	path      string
	playlist  string // "" when the file doesn't say
}

// The header names each column is known by, compared as foldText makes
// them, so "Spotify - id", "Spotify – ID" and "spotify_id" are one.
var csvColumns = map[string][]string{
	"uri":      {"track uri", "spotify uri", "uri", "spotify track uri", "spotify id", "spotify track id"},
	"title":    {"track name", "title", "track", "name", "song", "song name", "track title"},
	"artist":   {"artist name(s)", "artist name", "artists", "artist", "artist(s)"},
	"album":    {"album name", "album", "album title"},
	"duration": {"duration (ms)", "duration_ms", "duration ms"},
	"path":     {"path", "file", "location", "filename", "file path"},
	"playlist": {"playlist name", "playlist"},
}

// parseCSV reads a playlist .csv by its header row; columns it doesn't know
// are left alone, and rows with neither a title nor a path are skipped.
func parseCSV(data []byte) []csvRow {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	r := csv.NewReader(bytes.NewReader(data))
	r.FieldsPerRecord, r.LazyQuotes, r.ReuseRecord = -1, true, false
	head, err := r.Read()
	if err != nil {
		return nil
	}
	col := map[string]int{}
	for what, names := range csvColumns {
		col[what] = -1
	NAMES:
		for _, n := range names {
			for i, h := range head {
				if foldText(h) == foldText(n) {
					col[what] = i
					break NAMES
				}
			}
		}
	}
	field := func(rec []string, what string) string {
		if i := col[what]; i >= 0 && i < len(rec) {
			return strings.TrimSpace(rec[i])
		}
		return ""
	}
	var rows []csvRow
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		row := csvRow{title: field(rec, "title"), album: field(rec, "album"), path: field(rec, "path"),
			playlist: field(rec, "playlist")}
		if row.title == "" && row.path == "" {
			continue
		}
		row.spotifyID = spotifyTrack(field(rec, "uri"))
		row.artist, row.artists = splitArtists(field(rec, "artist"))
		if ms, err := strconv.ParseInt(field(rec, "duration"), 10, 64); err == nil && ms > 0 {
			row.duration = time.Duration(ms) * time.Millisecond
		}
		rows = append(rows, row)
	}
	return rows
}

// spotifyTrack is the track ID in a Spotify URI ("spotify:track:…"), link
// ("https://open.spotify.com/track/…?si=…") or a bare ID, as TuneMyMusic
// writes; "" for anything else.
func spotifyTrack(s string) string {
	switch {
	case strings.HasPrefix(s, "spotify:track:"):
		s = strings.TrimPrefix(s, "spotify:track:")
	case strings.Contains(s, "open.spotify.com/"):
		_, s, _ = strings.Cut(s, "/track/")
		s, _, _ = strings.Cut(s, "?")
	}
	if len(s) != 22 {
		return ""
	}
	for _, r := range s {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z') {
			return ""
		}
	}
	return s
}

// splitArtists is an artist column as one line for showing, and its artists
// to match by. Exportify separates them with ";"; others with ", " or " & ",
// which some names have in them, so a credit is matched whole as well as by
// each part.
func splitArtists(s string) (shown string, each []string) {
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == ';' })
	var names []string
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			names = append(names, p)
		}
	}
	shown = strings.Join(names, ", ")
	for _, n := range names {
		each = append(each, n)
		if len(names) > 1 {
			continue
		}
		for _, sep := range []string{", ", " & ", " feat. ", " ft. ", " x "} {
			if strings.Contains(n, sep) {
				for _, part := range strings.Split(n, sep) {
					if part = strings.TrimSpace(part); part != "" && !slices.Contains(each, part) {
						each = append(each, part)
					}
				}
			}
		}
	}
	return shown, each
}

// Find is the library's copy of a song known from elsewhere, as YouTube
// Music gives one: by its title and artist, matched as a .csv playlist's
// songs are, and only where its album or its length agrees, or neither is
// known, so another recording of the same title, as a live one, isn't
// taken for it. It's nil when the library hasn't the song.
func (ix *Index) Find(title, artist, album string, d time.Duration) *Track {
	if ix == nil || title == "" {
		return nil
	}
	ix.matchOnce.Do(func() { ix.matcher = newSongMatcher(ix) })
	_, artists := splitArtists(artist)
	row := csvRow{title: title, artists: artists, album: album, duration: d}
	t := ix.matcher.match(row)
	if t == nil || otherAlbum(t, row) && otherLength(t, row) {
		return nil
	}
	return t
}

// songMatcher finds a .csv row's song in the library.
type songMatcher struct {
	ix    *Index
	exact map[string][]*Track // by folded title
	bare  map[string][]*Track // by folded title without extras
}

func newSongMatcher(ix *Index) *songMatcher {
	m := &songMatcher{ix: ix, exact: map[string][]*Track{}, bare: map[string][]*Track{}}
	for _, t := range ix.Songs {
		m.exact[foldText(t.Title)] = append(m.exact[foldText(t.Title)], t)
		m.bare[bareTitle(t.Title)] = append(m.bare[bareTitle(t.Title)], t)
	}
	return m
}

// match is row's song: the one an import downloaded as its Spotify track,
// or by title and one of its artists, with the album or the length to tell
// versions apart. A title matches as written, or with its extras taken off
// when that leaves only one song by the artist.
func (m *songMatcher) match(row csvRow) *Track {
	if row.spotifyID != "" {
		if t := m.ix.Spotify(row.spotifyID); t != nil {
			return t
		}
	}
	if row.title == "" {
		return nil
	}
	if t := best(byArtist(m.exact[foldText(row.title)], row.artists), row); t != nil {
		return t
	}
	// Without its extras a title can fit other versions too: one song fits,
	// as long as its album and length don't say it's another version, or
	// the album or the length has to say which.
	c := byArtist(m.bare[bareTitle(row.title)], row.artists)
	if len(c) == 1 && (sameAlbum(c[0], row) || sameLength(c[0], row) || !otherAlbum(c[0], row) && !otherLength(c[0], row)) {
		return c[0]
	}
	if t := best(c, row); t != nil && (sameAlbum(t, row) || sameLength(t, row)) {
		return t
	}
	return nil
}

func sameAlbum(t *Track, row csvRow) bool {
	return row.album != "" && foldText(t.Album) == foldText(row.album)
}

// otherAlbum and otherLength report that the row says t is another release:
// both give an album, or a length, and they differ.
func otherAlbum(t *Track, row csvRow) bool {
	return row.album != "" && t.Album != "" && !sameAlbum(t, row)
}

func otherLength(t *Track, row csvRow) bool {
	return row.duration > 0 && t.Duration > 0 && !sameLength(t, row)
}

func sameLength(t *Track, row csvRow) bool {
	d := t.Duration - row.duration
	return row.duration > 0 && t.Duration > 0 && d < 3*time.Second && d > -3*time.Second
}

// byArtist keeps the songs credited to one of artists; with none given,
// all of them.
func byArtist(ts []*Track, artists []string) []*Track {
	if len(artists) == 0 {
		return ts
	}
	var out []*Track
	for _, t := range ts {
		credit := foldText(t.Artist + " " + t.AlbumArtist)
		for _, a := range artists {
			if fa := foldText(a); fa != "" && strings.Contains(credit, fa) {
				out = append(out, t)
				break
			}
		}
	}
	return out
}

// best picks among the candidates: the one on the same album, then the one
// within 3 s of the length, then the first.
func best(ts []*Track, row csvRow) *Track {
	if len(ts) == 0 {
		return nil
	}
	for _, t := range ts {
		if sameAlbum(t, row) {
			return t
		}
	}
	for _, t := range ts {
		if sameLength(t, row) {
			return t
		}
	}
	return ts[0]
}

// foldText is s lower-cased, without accents and with runs of anything but
// letters and digits made one space.
func foldText(s string) string {
	var b strings.Builder
	space := false
	for _, r := range norm.NFD.String(s) {
		switch {
		case unicode.Is(unicode.Mn, r):
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			if space && b.Len() > 0 {
				b.WriteByte(' ')
			}
			space = false
			b.WriteRune(unicode.ToLower(r))
		default:
			space = true
		}
	}
	return b.String()
}

// bareTitle is a title without its extras: what's in brackets, and what
// follows " - ", as in "Song (From "Game") - Piano Version".
func bareTitle(s string) string {
	var b strings.Builder
	depth := 0
	for _, r := range s {
		switch r {
		case '(', '[':
			depth++
		case ')', ']':
			if depth > 0 {
				depth--
			}
		default:
			if depth == 0 {
				b.WriteRune(r)
			}
		}
	}
	t := b.String()
	if i := strings.Index(t, " - "); i > 0 {
		t = t[:i]
	}
	return foldText(t)
}
