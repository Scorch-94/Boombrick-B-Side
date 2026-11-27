// Package audiobooks finds the audiobooks on the SD card and keeps the
// listener's place in each.
//
// A book is a file in the Audiobooks folder, as an .m4b, or a folder in it
// whose audio files are the book's parts, in order. Where the listener is
// in a book is kept in the database by the book's path: the part and the
// time into it.
package audiobooks

import (
	"cmp"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/ujjwal313/boombrick/internal/audio/decode"
	"github.com/ujjwal313/boombrick/internal/library"
	"github.com/ujjwal313/boombrick/internal/player"
)

// Book is an audiobook.
type Book struct {
	Path     string // its file or folder: what its place is kept by
	Title    string
	Author   string
	Parts    []Part
	Duration time.Duration
	// Chapters are the chapters its files list, or else one for each file.
	Chapters []Chapter
}

// Chapter is a chapter of a book: its title, the part it's in, and where in
// that part it starts and how long it lasts.
type Chapter struct {
	Title         string
	Part          int
	Start, Length time.Duration
}

// Part is one file of a book.
type Part struct {
	Path     string
	Title    string
	Duration time.Duration
	Chapters []decode.Chapter // those the file lists, if any
}

// dir is the Audiobooks folder, once SetDir says; IsBook goes by it.
var dir atomic.Pointer[string]

// SetDir says where the Audiobooks folder is.
func SetDir(d string) {
	d = filepath.Clean(d)
	dir.Store(&d)
}

// IsBook reports whether path is part of an audiobook: a file in the
// Audiobooks folder. Now Playing and the headphones' buttons skip 30 seconds
// in one, as in a podcast episode.
func IsBook(path string) bool {
	d := dir.Load()
	return d != nil && path != "" && strings.HasPrefix(path, *d+string(filepath.Separator))
}

// BookOf is the path of the book path is part of: path itself for a file at
// the top of the Audiobooks folder, else that top folder.
func BookOf(path string) string {
	d := dir.Load()
	if d == nil || !IsBook(path) {
		return ""
	}
	rel := strings.TrimPrefix(path, *d+string(filepath.Separator))
	top, _, _ := strings.Cut(rel, string(filepath.Separator))
	return filepath.Join(*d, top)
}

// Scan finds the books in the Audiobooks folder dir, by title.
func Scan(dir string) []Book {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var books []Book
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		p := filepath.Join(dir, e.Name())
		var parts []string
		if e.IsDir() {
			parts = audioFiles(p)
		} else if decode.Supported(p) {
			parts = []string{p}
		}
		if b, ok := read(p, parts, e.IsDir()); ok {
			books = append(books, b)
		}
	}
	slices.SortFunc(books, func(a, b Book) int { return library.Compare(a.Title, b.Title) })
	m := make(map[string]Book, len(books))
	for _, b := range books {
		m[b.Path] = b
	}
	known.Store(&m)
	return books
}

// audioFiles is the audio files under folder, in the order of their track
// tags, then their paths.
func audioFiles(folder string) []string {
	var out []string
	filepath.WalkDir(folder, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && strings.HasPrefix(d.Name(), ".") && p != folder {
			return filepath.SkipDir
		}
		if !d.IsDir() && !strings.HasPrefix(d.Name(), ".") && decode.Supported(p) {
			out = append(out, p)
		}
		return nil
	})
	slices.SortFunc(out, library.Compare)
	return out
}

// read makes the book at path from its parts' tags and lengths. A book's
// title is its album tag, else its single file's title tag, else its name;
// its author the album artist or artist.
func read(path string, parts []string, folder bool) (Book, bool) {
	if len(parts) == 0 {
		return Book{}, false
	}
	b := Book{Path: path}
	type tagged struct {
		Part
		disc, track int
	}
	ts := make([]tagged, len(parts))
	for i, p := range parts {
		tg, _ := library.ReadTags(p)
		d, _ := decode.Duration(p)
		cs, _ := decode.Chapters(p)
		name := strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
		ts[i] = tagged{Part: Part{Path: p, Title: cmp.Or(tg.Title, name), Duration: d, Chapters: cs}, disc: tg.Disc, track: tg.Track}
		if b.Title == "" {
			b.Title = tg.Album
			if !folder {
				b.Title = cmp.Or(tg.Album, tg.Title)
			}
		}
		if b.Author == "" {
			b.Author = cmp.Or(tg.AlbumArtist, tg.Artist)
		}
	}
	slices.SortStableFunc(ts, func(a, c tagged) int {
		if a.disc != c.disc {
			return a.disc - c.disc
		}
		if a.track != c.track && a.track > 0 && c.track > 0 {
			return a.track - c.track
		}
		return 0
	})
	for _, t := range ts {
		b.Parts = append(b.Parts, t.Part)
		b.Duration += t.Duration
	}
	if b.Title == "" {
		b.Title = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	b.Chapters = chapters(b)
	return b, true
}

// chapters are b's chapters: those each part lists, else the part itself.
// A part's list that starts after its start gains a chapter before it, so
// the whole part is covered.
func chapters(b Book) []Chapter {
	var out []Chapter
	for i, p := range b.Parts {
		cs := p.Chapters
		if len(cs) == 0 {
			title := p.Title
			if len(b.Parts) == 1 {
				title = b.Title
			}
			out = append(out, Chapter{Title: title, Part: i, Length: p.Duration})
			continue
		}
		if cs[0].Start > 0 {
			out = append(out, Chapter{Title: p.Title, Part: i, Length: cs[0].Start})
		}
		for j, c := range cs {
			end := p.Duration
			if j+1 < len(cs) && (p.Duration <= 0 || cs[j+1].Start < p.Duration) {
				end = cs[j+1].Start
			}
			if p.Duration > 0 && c.Start >= p.Duration {
				break // past the end of the audio: a damaged list
			}
			out = append(out, Chapter{Title: cmp.Or(c.Title, "Chapter "+strconv.Itoa(j+1)), Part: i, Start: c.Start,
				Length: max(end-c.Start, 0)})
		}
	}
	return out
}

// Part is the index of b's part at path, or -1.
func (b Book) Part(path string) int {
	return slices.IndexFunc(b.Parts, func(p Part) bool { return p.Path == path })
}

// ChapterAt is the index of the chapter playing at time at into part, or -1
// when b has no such part.
func (b Book) ChapterAt(part int, at time.Duration) int {
	found := -1
	for i, c := range b.Chapters {
		if c.Part > part {
			break
		}
		if c.Part == part && (found < 0 || c.Start <= at) {
			found = i
		}
	}
	return found
}

// known are the books the last scan found, by path, for Lookup.
var known atomic.Pointer[map[string]Book]

// Lookup is the book path is part of, as the last scan found it.
func Lookup(path string) (Book, bool) {
	m := known.Load()
	if m == nil {
		return Book{}, false
	}
	b, ok := (*m)[BookOf(path)]
	return b, ok
}

// Tracks are b's parts as queue entries.
func (b Book) Tracks() []player.Track {
	ts := make([]player.Track, len(b.Parts))
	for i, p := range b.Parts {
		title := p.Title
		if len(b.Parts) == 1 {
			title = b.Title
		}
		ts[i] = player.Track{Path: p.Path, Title: title, Artist: b.Author, AlbumArtist: b.Author, Album: b.Title,
			Duration: p.Duration}
	}
	return ts
}

// Left is how much of b is left to hear from place pl.
func (b Book) Left(pl Place) time.Duration {
	if pl.Finished {
		return 0
	}
	left := b.Duration
	for _, p := range b.Parts {
		if p.Path == pl.Part {
			return max(left-pl.Position, 0)
		}
		left -= p.Duration
	}
	return b.Duration
}

// Start is where to pick b up from pl: the part's index and the time into
// it, a few seconds back so the listener hears where they were. A book
// finished, or whose part is gone, starts at the beginning.
func (b Book) Start(pl Place) (int, time.Duration) {
	if pl.Finished {
		return 0, 0
	}
	for i, p := range b.Parts {
		if p.Path == pl.Part {
			return i, max(pl.Position-rewind, 0)
		}
	}
	return 0, 0
}

// rewind is how far back from where it was left a book is picked up.
const rewind = 5 * time.Second
