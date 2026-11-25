package library

import (
	"bytes"
	"encoding/csv"
	"io"
	"strconv"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// csvRow is one song of a .csv playlist, as exported by Exportify, Soundiiz,
// TuneMyMusic and the like: by its Spotify URI where it has one, by title
// and artists, or by a file's path.
type csvRow struct {
	spotifyID string
	title     string
	album     string
	artists   []string
	duration  time.Duration // 0 if not given
	path      string
}

// The header names each column is known by, lower-cased.
var csvColumns = map[string][]string{
	"uri":      {"track uri", "spotify uri", "uri", "spotify track uri"},
	"title":    {"track name", "title", "track", "name", "song", "song name", "track title"},
	"artist":   {"artist name(s)", "artist name", "artists", "artist", "artist(s)"},
	"album":    {"album name", "album", "album title"},
	"duration": {"duration (ms)", "duration_ms", "duration ms"},
	"path":     {"path", "file", "location", "filename", "file path"},
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
				if strings.EqualFold(strings.TrimSpace(h), n) {
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
		row := csvRow{title: field(rec, "title"), album: field(rec, "album"), path: field(rec, "path")}
		if row.title == "" && row.path == "" {
			continue
		}
		if u := field(rec, "uri"); strings.HasPrefix(u, "spotify:track:") {
			row.spotifyID = strings.TrimPrefix(u, "spotify:track:")
		}
		for _, a := range strings.FieldsFunc(field(rec, "artist"), func(r rune) bool { return r == ';' }) {
			if a = strings.TrimSpace(a); a != "" {
				row.artists = append(row.artists, a)
			}
		}
		if ms, err := strconv.ParseInt(field(rec, "duration"), 10, 64); err == nil && ms > 0 {
			row.duration = time.Duration(ms) * time.Millisecond
		}
		rows = append(rows, row)
	}
	return rows
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
	// or the album or the length has to say which.
	c := byArtist(m.bare[bareTitle(row.title)], row.artists)
	if len(c) == 1 {
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
