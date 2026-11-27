package audiobooks

import (
	"context"
	"sync"
	"time"

	"github.com/ujjwal313/boombrick/internal/audio/decode"
	"github.com/ujjwal313/boombrick/internal/player"
)

// chapterSleeper is the player as the End of chapter timer needs it.
// *player.Player is one.
type chapterSleeper interface {
	State() player.State
	Sleep() player.Sleep
	SleepWithin(d time.Duration)
}

// armWithin is how long before a chapter's end the timer is told where it
// ends: its fade takes 10 s, and the player is looked at every second.
const armWithin = 12 * time.Second

// WatchChapterSleep makes the sleep timer's End of chapter stop at the end
// of the audiobook chapter playing, until ctx ends. A chapter that ends
// with its file needs nothing: the timer stops at the end of a track. One
// that ends inside a file, as an .m4b's do, is watched for: as its end
// nears, the timer is told when it comes, at the speed it plays.
func WatchChapterSleep(ctx context.Context, p chapterSleeper) {
	t := time.NewTicker(look)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if p.Sleep().Mode != player.SleepChapter {
				continue
			}
			s := p.State()
			if !s.Playing || !IsBook(s.Track.Path) {
				continue
			}
			end, ok := chapterEnd(s.Track.Path, s.Elapsed, s.Duration)
			if !ok {
				continue
			}
			left := end - s.Elapsed
			if s.Speed > 0 {
				left = time.Duration(float64(left) / s.Speed)
			}
			if left >= 0 && left <= armWithin {
				p.SleepWithin(left)
			}
		}
	}
}

// chapterEnd is where the chapter playing at elapsed into the file at path
// ends, when that's before the file's end (of length). It goes by the
// book's chapters as scanned, else by the file's own.
func chapterEnd(path string, elapsed, length time.Duration) (time.Duration, bool) {
	end := length
	if b, ok := Lookup(path); ok {
		if i := b.ChapterAt(b.Part(path), elapsed); i >= 0 {
			c := b.Chapters[i]
			end = c.Start + c.Length
		}
	} else {
		for _, c := range fileChapters(path) {
			if c.Start > elapsed {
				end = c.Start
				break
			}
		}
	}
	return end, end > 0 && (length <= 0 || end < length-time.Second)
}

// files remembers the chapters files list, for the background player,
// which scans no books.
var files struct {
	sync.Mutex
	path string
	cs   []decode.Chapter
}

func fileChapters(path string) []decode.Chapter {
	files.Lock()
	defer files.Unlock()
	if files.path != path {
		files.cs, _ = decode.Chapters(path)
		files.path = path
	}
	return files.cs
}
