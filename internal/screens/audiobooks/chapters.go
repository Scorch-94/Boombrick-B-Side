package audiobooks

import (
	"strconv"
	"time"

	"github.com/ujjwal313/boombrick/internal/audiobooks"
	"github.com/ujjwal313/boombrick/internal/platform/input"
	"github.com/ujjwal313/boombrick/internal/player"
	"github.com/ujjwal313/boombrick/internal/screens"
	"github.com/ujjwal313/boombrick/internal/screens/nowplaying"
	"github.com/ujjwal313/boombrick/internal/theme"
	"github.com/ujjwal313/boombrick/internal/ui"
	"github.com/ujjwal313/boombrick/internal/ui/gfx"
	"github.com/ujjwal313/boombrick/internal/ui/text"
	"github.com/ujjwal313/boombrick/internal/ui/views"
	"github.com/ujjwal313/boombrick/internal/ui/widgets"
)

// restartWithin is how far into a chapter going back starts it again
// rather than going to the one before, as a track's previous does.
const restartWithin = 3 * time.Second

// play plays b from chapter i. With seek, in the part playing it only seeks,
// as Now Playing's L1 and R1 do; else it plays the book from that part,
// which opens Now Playing, as choosing a chapter from the list does.
func play(p Player, b audiobooks.Book, i int, seek bool) {
	c := b.Chapters[i]
	if s := p.State(); seek && s.Queued && b.Part(s.Track.Path) == c.Part && audiobooks.BookOf(s.Track.Path) == b.Path {
		p.Seek(c.Start)
		if !s.Playing {
			p.Toggle()
		}
		return
	}
	p.SetShuffle(false)
	p.PlayAt(b.Tracks(), c.Part, c.Start)
}

// Chapters gives Now Playing the chapters of the book playing. It is
// asked every frame, so what it shows is made again only as the chapter
// changes.
type Chapters struct {
	p    Player
	list func(b audiobooks.Book) // opens b's chapters

	of   [2]string // the book and the chapter's index what's below is for
	line nowplaying.Chapter
}

// NewChapters gives Now Playing the chapters of the books p plays, and
// opens a book's list with list.
func NewChapters(p Player, list func(audiobooks.Book)) *Chapters { return &Chapters{p: p, list: list} }

// current is the book playing and its chapter's index, for a book of more
// than one chapter.
func current(s player.State) (audiobooks.Book, int, bool) {
	if !s.Queued || !audiobooks.IsBook(s.Track.Path) {
		return audiobooks.Book{}, 0, false
	}
	b, ok := audiobooks.Lookup(s.Track.Path)
	if !ok || len(b.Chapters) < 2 {
		return b, 0, false
	}
	i := b.ChapterAt(b.Part(s.Track.Path), s.Elapsed)
	return b, i, i >= 0
}

// At is the chapter playing, if a book of chapters plays.
func (c *Chapters) At(s player.State) (nowplaying.Chapter, bool) {
	b, i, ok := current(s)
	if !ok {
		return nowplaying.Chapter{}, false
	}
	if of := [2]string{b.Path, strconv.Itoa(i)}; of != c.of {
		ch := b.Chapters[i]
		line := "Chapter " + strconv.Itoa(i+1) + " of " + strconv.Itoa(len(b.Chapters))
		if b.Author != "" {
			line = b.Author + " · " + line
		}
		c.of = of
		c.line = nowplaying.Chapter{Book: b.Title, Title: ch.Title, Line: line, Start: ch.Start, Length: ch.Length}
	}
	return c.line, true
}

// Step goes to the next chapter, or with delta -1 back to the start of the
// one playing, or the one before when it has only just begun. It reports
// false when no book of chapters plays.
func (c *Chapters) Step(s player.State, delta int) bool {
	b, i, ok := current(s)
	if !ok {
		return false
	}
	if delta < 0 && s.Elapsed-b.Chapters[i].Start > restartWithin {
		delta = 0
	}
	to := i + delta
	if to >= len(b.Chapters) {
		return true // the last chapter: nothing after
	}
	play(c.p, b, max(to, 0), true)
	return true
}

// List opens the chapters of the book playing, reporting false when none
// with chapters plays.
func (c *Chapters) List(s player.State) bool {
	b, _, ok := current(s)
	if ok {
		c.list(b)
	}
	return ok
}

var chapterHints = []widgets.Hint{{Key: "A", Label: "Play"}, {Key: "B", Label: "Back"}, {Key: "L1/R1", Label: "Tabs"}}

// ChapterPage is a book's chapters, the one playing or last heard focused.
type ChapterPage struct {
	p     Player
	book  audiobooks.Book
	title string
	count string
	rows  []widgets.Row
	art   widgets.ArtSpec
	list  views.Grid
}

// NewChapterPage lists b's chapters, focusing chapter at.
func NewChapterPage(p Player, b audiobooks.Book, at int) *ChapterPage {
	pg := &ChapterPage{p: p, book: b, title: b.Title, count: strconv.Itoa(len(b.Chapters)) + " chapters"}
	pg.art = bookArt(b)
	pg.list = views.Grid{Cols: 1, CellW: theme.ScreenW - screens.Side - screens.PageListX, CellH: widgets.RowHeight,
		Pad: screens.Pad, ViewH: screens.ContentBot - screens.ContentTop}
	pg.rows = make([]widgets.Row, len(b.Chapters))
	for i, c := range b.Chapters {
		pg.rows[i] = widgets.Row{Index: strconv.Itoa(i + 1), Title: c.Title, Duration: clock(c.Length)}
	}
	pg.list.SetCount(len(pg.rows))
	pg.list.SetFocus(at, time.Time{}) // already there when the page shows
	return pg
}

// clock is a length as the lists show one: "4:05", "1:02:09".
func clock(d time.Duration) string {
	s := int(d / time.Second)
	two := func(n int) string {
		if n < 10 {
			return "0" + strconv.Itoa(n)
		}
		return strconv.Itoa(n)
	}
	if s >= 3600 {
		return strconv.Itoa(s/3600) + ":" + two(s/60%60) + ":" + two(s%60)
	}
	return strconv.Itoa(s/60) + ":" + two(s%60)
}

// Handle plays the focused chapter with A.
func (pg *ChapterPage) Handle(ev input.Event, now time.Time) bool {
	if ev.Kind == input.Press && ev.Button == input.A && len(pg.rows) > 0 {
		play(pg.p, pg.book, pg.list.Focus(), false)
		return true
	}
	return pg.list.Move(ev.Button, now)
}

// Animating reports whether focus is moving.
func (pg *ChapterPage) Animating(now time.Time) bool { return pg.list.Animating(now) }

// Draw renders the frame.
func (pg *ChapterPage) Draw(u *ui.Context) {
	widgets.DrawBackground(u, widgets.TabGround)
	side, w := float32(screens.Side), float32(screens.PageArt)
	widgets.Art(u, pg.art, screens.PageArt, widgets.CardRadius, gfx.Rect{X: side, Y: screens.PageTop, W: w, H: w})
	y := float32(screens.PageTop + screens.PageArt + screens.Pad)
	u.Text(u.Fit(pg.title, text.Heading, w), text.Heading, side, y, theme.Paper)
	y += u.LineHeight(text.Heading)
	u.Text(u.Fit(pg.book.Author, text.Meta, w), text.Meta, side, y, theme.Ink2)
	y += u.LineHeight(text.Meta)
	u.Text(pg.count, text.Meta, side, y, theme.Ink3)

	s := pg.p.State()
	playing := -1
	if b, i, ok := current(s); ok && b.Path == pg.book.Path {
		playing = i
	}
	u.PushClip(screens.GlowArea)
	pg.list.Draw(u.Now, screens.PageListX, screens.ContentTop, func(i int, x, y, k float32) {
		r := pg.rows[i]
		r.Playing = i == playing
		box := gfx.Rect{X: x, Y: y, W: pg.list.CellW, H: widgets.RowHeight}
		widgets.RowGlow(u, box, widgets.RowRadius, k)
		u.PushClip(screens.Below(screens.ContentTop))
		widgets.ListRow(u, r, box, k)
		u.PopClip()
	})
	u.PopClip()
	widgets.StatusBar(u)
	widgets.Tabs(u, widgets.TabAudiobooks)
	widgets.HintBar(u, chapterHints)
}
