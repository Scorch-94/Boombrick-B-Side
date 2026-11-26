package library

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// PlaylistDir is the folder, inside the music folder, whose .m3u, .m3u8 and
// .csv files are the Library's playlists.
const PlaylistDir = "Playlists"

// Playlist is a playlist file's songs, in the file's order. Entries holds
// every song it can play: the library's, and for a .csv the songs the
// library doesn't have but Spotify does, which are streamed. Songs is just
// the library's. Missing counts the entries that are neither, and Streams
// the Spotify ones.
type Playlist struct {
	Name     string // the file's name without its extension
	Path     string
	Entries  []PlaylistEntry
	Songs    []*Track
	Streams  int
	Missing  int
	Duration time.Duration // of every entry
}

// PlaylistEntry is one of a playlist's songs: the library's Song, or a
// Stream from Spotify.
type PlaylistEntry struct {
	Song   *Track
	Stream *StreamSong
}

// StreamSong is a playlist song the library doesn't have, to stream from
// Spotify by its track ID.
type StreamSong struct {
	SpotifyID, Title, Artist, Album string
	Duration                        time.Duration // 0 if not given
}

// add appends a library song.
func (pl *Playlist) add(t *Track) {
	pl.Entries = append(pl.Entries, PlaylistEntry{Song: t})
	pl.Songs = append(pl.Songs, t)
	pl.Duration += t.Duration
}

// ReadPlaylists reads the playlists in music's Playlists folder and matches
// their entries to ix's songs. An entry may be a path relative to the
// playlist, relative to the music folder, or absolute; "\" works as "/",
// and case is ignored when no exact match is found. A folder that isn't
// there is no playlists. Files that can't be read are skipped.
func ReadPlaylists(music string, ix *Index) []*Playlist {
	dir := filepath.Join(music, PlaylistDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	byPath := make(map[string]*Track, len(ix.Songs))
	byLower := make(map[string]*Track, len(ix.Songs))
	for _, t := range ix.Songs {
		p := filepath.Clean(t.Path)
		byPath[p] = t
		byLower[strings.ToLower(p)] = t
	}
	find := func(p string) *Track {
		p = filepath.Clean(p)
		if t := byPath[p]; t != nil {
			return t
		}
		return byLower[strings.ToLower(p)]
	}
	// resolve finds a playlist's path entry: relative to the playlist, to
	// the music folder, or absolute, from this device or another.
	resolve := func(entry string) *Track {
		entry = strings.ReplaceAll(strings.TrimPrefix(entry, "file://"), `\`, "/")
		if filepath.IsAbs(entry) || hasDrive(entry) {
			if t := find(entry); t != nil {
				return t
			}
			return find(underMusic(entry, music))
		}
		if t := find(filepath.Join(dir, entry)); t != nil {
			return t
		}
		return find(filepath.Join(music, entry))
	}
	var songs *songMatcher // made for the first .csv
	var lists []*Playlist
	for _, e := range entries {
		name := e.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if e.IsDir() || (ext != ".m3u" && ext != ".m3u8" && ext != ".csv") || strings.HasPrefix(name, ".") {
			continue
		}
		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		pl := &Playlist{Name: strings.TrimSuffix(name, filepath.Ext(name)), Path: path}
		if ext == ".csv" {
			if songs == nil {
				songs = newSongMatcher(ix)
			}
			pl.Name = strings.ReplaceAll(pl.Name, "_", " ")
			// A file naming its rows' playlists, as TuneMyMusic's, is one
			// playlist for each name, in the file's order; rows naming none
			// are the file's own.
			byName := map[string]*Playlist{}
			var order []*Playlist
			for _, row := range parseCSV(data) {
				p := byName[row.playlist]
				if p == nil {
					p = &Playlist{Name: pl.Name, Path: path}
					if row.playlist != "" {
						p.Name = row.playlist
					}
					byName[row.playlist] = p
					order = append(order, p)
				}
				var t *Track
				if row.path != "" {
					t = resolve(row.path)
				} else {
					t = songs.match(row)
				}
				switch {
				case t != nil:
					p.add(t)
				case row.spotifyID != "" && row.title != "":
					p.Entries = append(p.Entries, PlaylistEntry{Stream: &StreamSong{SpotifyID: row.spotifyID,
						Title: row.title, Artist: row.artist, Album: row.album, Duration: row.duration}})
					p.Streams++
					p.Duration += row.duration
				default:
					p.Missing++
				}
			}
			if len(order) == 0 {
				order = append(order, pl) // an empty file is still a playlist
			}
			lists = append(lists, order...)
			continue
		}
		for _, entry := range parseM3U(data) {
			t := resolve(entry)
			if t == nil {
				pl.Missing++
				continue
			}
			pl.add(t)
		}
		lists = append(lists, pl)
	}
	slices.SortStableFunc(lists, func(x, y *Playlist) int { return Compare(x.Name, y.Name) })
	return lists
}

// parseM3U is an .m3u file's entries: every line but the blank ones and the
// #EXTM3U, #EXTINF and other comments, with "\" made "/" and a "file://"
// prefix dropped. A UTF-8 byte order mark is ignored.
func parseM3U(data []byte) []string {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	var out []string
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 4096), 64<<10)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "file://")
		line = strings.ReplaceAll(line, `\`, "/")
		out = append(out, line)
	}
	return out
}

// underMusic maps an absolute path written on another machine or mount onto
// music: from the last "/Music/" in it, case ignored, it is taken as under
// music. "/storage/sdcard1/Music/A/b.flac" is music's "A/b.flac".
func underMusic(p, music string) string {
	i := strings.LastIndex(strings.ToLower(p), "/music/")
	if i < 0 {
		return p
	}
	return filepath.Join(music, p[i+len("/music/"):])
}

// hasDrive reports whether p starts with a Windows drive, as "C:/".
func hasDrive(p string) bool {
	return len(p) >= 3 && p[1] == ':' && p[2] == '/' &&
		(p[0] >= 'A' && p[0] <= 'Z' || p[0] >= 'a' && p[0] <= 'z')
}
