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

// PlaylistDir is the folder, inside the music folder, whose .m3u and .m3u8
// files are the Library's playlists.
const PlaylistDir = "Playlists"

// Playlist is an .m3u file's songs that are in the library, in the file's
// order. Missing counts the lines naming a file the library doesn't have.
type Playlist struct {
	Name     string // the file's name without its extension
	Path     string
	Songs    []*Track
	Missing  int
	Duration time.Duration
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
	var lists []*Playlist
	for _, e := range entries {
		name := e.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if e.IsDir() || (ext != ".m3u" && ext != ".m3u8") || strings.HasPrefix(name, ".") {
			continue
		}
		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		pl := &Playlist{Name: strings.TrimSuffix(name, filepath.Ext(name)), Path: path}
		for _, entry := range parseM3U(data) {
			var t *Track
			if filepath.IsAbs(entry) || hasDrive(entry) {
				t = find(entry)
				if t == nil {
					t = find(underMusic(entry, music))
				}
			} else {
				t = find(filepath.Join(dir, entry))
				if t == nil {
					t = find(filepath.Join(music, entry))
				}
			}
			if t == nil {
				pl.Missing++
				continue
			}
			pl.Songs = append(pl.Songs, t)
			pl.Duration += t.Duration
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
