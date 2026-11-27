package library

import (
	"bytes"
	"encoding/csv"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ErrPlaylistName is returned for a playlist name with nothing in it.
var ErrPlaylistName = errors.New("library: a playlist needs a name")

// CleanPlaylistName is name as a playlist's file can be called: without the
// characters a file name can't hold on the SD card's FAT, trimmed.
func CleanPlaylistName(name string) string {
	name = strings.Map(func(r rune) rune {
		if strings.ContainsRune(`/\:*?"<>|`, r) || r < ' ' {
			return ' '
		}
		return r
	}, name)
	return strings.Join(strings.Fields(name), " ")
}

// NewPlaylist makes an empty playlist called name in music's Playlists
// folder, as an .m3u8 file, and returns it. A name taken gets a number: "Road
// trip 2".
func NewPlaylist(music, name string) (*Playlist, error) {
	name = CleanPlaylistName(name)
	if name == "" {
		return nil, ErrPlaylistName
	}
	dir := filepath.Join(music, PlaylistDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	path := free(dir, name, ".m3u8")
	pl := &Playlist{Name: strings.TrimSuffix(filepath.Base(path), ".m3u8"), Path: path}
	return pl, writeFile(path, []byte("#EXTM3U\n"))
}

// free is the path in dir for a file called name with ext, numbered past any
// file there already, whatever its case.
func free(dir, name, ext string) string {
	taken := map[string]bool{}
	if es, err := os.ReadDir(dir); err == nil {
		for _, e := range es {
			taken[strings.ToLower(e.Name())] = true
		}
	}
	for n := 1; ; n++ {
		try := name
		if n > 1 {
			try += " " + strconv.Itoa(n)
		}
		if !taken[strings.ToLower(try+ext)] {
			return filepath.Join(dir, try+ext)
		}
	}
}

// Editable reports whether Boombrick can save changes to pl: one read from
// an .m3u, .m3u8 or .csv file.
func (pl *Playlist) Editable() bool {
	switch strings.ToLower(filepath.Ext(pl.Path)) {
	case ".m3u", ".m3u8", ".csv":
		return true
	}
	return false
}

// SetEntries makes es pl's songs, counting its streams and time again.
func (pl *Playlist) SetEntries(es []PlaylistEntry) {
	pl.Entries, pl.Songs, pl.Streams, pl.Duration = es, nil, 0, 0
	for _, e := range es {
		if e.Song != nil {
			pl.Songs = append(pl.Songs, e.Song)
			pl.Duration += e.Song.Duration
		} else {
			pl.Streams++
			pl.Duration += e.Stream.Duration
		}
	}
}

// Save writes pl's entries to its file. An .m3u or .m3u8 gets each song's
// path from the Playlists folder, and a Spotify song as a spotify:track:
// line after an #EXTINF naming it; a .csv gets each song's title, artist,
// album, length, Spotify ID and path, with the file's other playlists kept.
// Entries the file had that weren't found are kept at the end.
func (pl *Playlist) Save(music string) error {
	if strings.EqualFold(filepath.Ext(pl.Path), ".csv") {
		return pl.saveCSV(music, pl.csvName, false)
	}
	var b bytes.Buffer
	b.WriteString("#EXTM3U\n")
	dir := filepath.Dir(pl.Path)
	for _, e := range pl.Entries {
		if s := e.Stream; s != nil {
			b.WriteString(extinfLine(s.Duration, s.Artist, s.Title))
			b.WriteString("spotify:track:" + s.SpotifyID + "\n")
			continue
		}
		t := e.Song
		b.WriteString(extinfLine(t.Duration, t.Artist, t.Title))
		p, err := filepath.Rel(dir, t.Path)
		if err != nil {
			p = t.Path
		}
		b.WriteString(filepath.ToSlash(p) + "\n")
	}
	for _, l := range pl.lostLines {
		b.WriteString(l + "\n")
	}
	return writeFile(pl.Path, b.Bytes())
}

func extinfLine(d time.Duration, artist, title string) string {
	name := title
	if artist != "" {
		name = artist + " - " + title
	}
	return "#EXTINF:" + strconv.Itoa(int(d/time.Second)) + "," + strings.ReplaceAll(name, "\n", " ") + "\n"
}

// csvHead is the header Boombrick writes a .csv playlist with: names
// parseCSV knows.
var csvHead = []string{"Track name", "Artist name(s)", "Album name", "Duration (ms)", "Spotify ID", "Path", "Playlist name"}

// saveCSV writes pl's file again: the rows of the file's other playlists as
// they were, then pl's under the name as, unless drop.
func (pl *Playlist) saveCSV(music, as string, drop bool) error {
	data, err := os.ReadFile(pl.Path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	var rows [][]string
	for _, r := range parseCSV(data) {
		if r.playlist != pl.csvName {
			rows = append(rows, csvRecord(r))
		}
	}
	if !drop {
		for _, e := range pl.Entries {
			r := csvRow{playlist: as}
			if s := e.Stream; s != nil {
				r.title, r.artist, r.album, r.duration, r.spotifyID = s.Title, s.Artist, s.Album, s.Duration, s.SpotifyID
			} else {
				t := e.Song
				r.title, r.artist, r.album, r.duration = t.Title, t.Artist, t.Album, t.Duration
				if p, err := filepath.Rel(music, t.Path); err == nil {
					r.path = filepath.ToSlash(p)
				} else {
					r.path = t.Path
				}
			}
			rows = append(rows, csvRecord(r))
		}
		for _, r := range pl.lostRows {
			r.playlist = as
			rows = append(rows, csvRecord(r))
		}
	}
	if len(rows) == 0 && drop {
		return os.Remove(pl.Path) // nothing left in the file
	}
	var b bytes.Buffer
	w := csv.NewWriter(&b)
	w.Write(csvHead)
	w.WriteAll(rows)
	if err := w.Error(); err != nil {
		return err
	}
	pl.csvName = as
	return writeFile(pl.Path, b.Bytes())
}

func csvRecord(r csvRow) []string {
	ms := ""
	if r.duration > 0 {
		ms = strconv.FormatInt(r.duration.Milliseconds(), 10)
	}
	return []string{r.title, r.artist, r.album, ms, r.spotifyID, r.path, r.playlist}
}

// Rename calls pl name: its file takes the name, or in a .csv holding
// several playlists, its rows do.
func (pl *Playlist) Rename(music, name string) error {
	name = CleanPlaylistName(name)
	if name == "" {
		return ErrPlaylistName
	}
	if strings.EqualFold(filepath.Ext(pl.Path), ".csv") && pl.csvName != "" {
		if err := pl.saveCSV(music, name, false); err != nil {
			return err
		}
		pl.Name = name
		return nil
	}
	ext := filepath.Ext(pl.Path)
	to := filepath.Join(filepath.Dir(pl.Path), name+ext)
	if !strings.EqualFold(to, pl.Path) {
		to = free(filepath.Dir(pl.Path), name, ext)
	}
	if err := os.Rename(pl.Path, to); err != nil {
		return err
	}
	pl.Path, pl.Name = to, strings.TrimSuffix(filepath.Base(to), ext)
	return nil
}

// Delete removes pl: its file, or from a .csv holding others, its rows.
func (pl *Playlist) Delete(music string) error {
	if strings.EqualFold(filepath.Ext(pl.Path), ".csv") {
		return pl.saveCSV(music, "", true)
	}
	return os.Remove(pl.Path)
}

// writeFile replaces path with data whole: written beside it, then moved
// over it, so a power cut leaves the old file or the new.
func writeFile(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}
