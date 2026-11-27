// Package audiobooks is the Audiobooks tab: the books in the SD card's
// Audiobooks folder, each picked up where it was left.
package audiobooks

import (
	"strconv"
	"sync"
	"time"

	"github.com/ujjwal313/boombrick/internal/audiobooks"
	"github.com/ujjwal313/boombrick/internal/media"
	"github.com/ujjwal313/boombrick/internal/platform/input"
	"github.com/ujjwal313/boombrick/internal/player"
	"github.com/ujjwal313/boombrick/internal/screens"
	"github.com/ujjwal313/boombrick/internal/theme"
	"github.com/ujjwal313/boombrick/internal/ui"
	"github.com/ujjwal313/boombrick/internal/ui/gfx"
	"github.com/ujjwal313/boombrick/internal/ui/views"
	"github.com/ujjwal313/boombrick/internal/ui/widgets"
)

// Player plays a book from its place. *player.Player is one.
type Player interface {
	PlayAt(tracks []player.Track, start int, at time.Duration)
	SetShuffle(on bool)
	State() player.State
}

// Places keeps where the listener is in each book. *audiobooks.Places is
// one.
type Places interface {
	All() (map[string]audiobooks.Place, error)
	Forget(book string) error
}

// placesEvery is how often the shown tab reads the places again, as a book
// playing moves on.
const placesEvery = 5 * time.Second

// Tab is the Audiobooks tab: the books, those under way first, most recent
// first, then those not started, then those finished, each by title. A
// plays a book from where it was left; X starts it over.
type Tab struct {
	nav    screens.Nav
	p      Player
	places Places
	dir    string

	mu       sync.Mutex
	scanned  []audiobooks.Book // what the last scan found, nil while the first runs
	gen      int               // scans done, so a frame takes a new one once
	took     int
	scanning bool

	books []audiobooks.Book
	at    map[string]audiobooks.Place
	rows  []widgets.Row
	arts  []widgets.ArtSpec
	list  views.Grid
	msg   widgets.Wrapped
	dirty bool      // the places changed: the rows are made again at the next frame
	read  time.Time // when the places were last read
}

// New makes the tab for the books in dir, and finds them.
func New(nav screens.Nav, p Player, places Places, dir string) *Tab {
	t := &Tab{nav: nav, p: p, places: places, dir: dir, took: -1}
	t.list = views.Grid{Cols: 1, CellW: theme.ScreenW - 2*screens.Side, CellH: widgets.RowHeight, Pad: screens.Pad,
		ViewH: screens.ContentBot - screens.ContentTop}
	t.Rescan()
	return t
}

// Rescan finds the books again, on a goroutine: reading a book's tags and
// length takes a moment for each.
func (t *Tab) Rescan() {
	t.mu.Lock()
	if t.scanning {
		t.mu.Unlock()
		return
	}
	t.scanning = true
	t.mu.Unlock()
	go func() {
		books := audiobooks.Scan(t.dir)
		t.mu.Lock()
		t.scanned, t.gen, t.scanning = books, t.gen+1, false
		t.mu.Unlock()
	}()
}

// Changed says the places changed, as when a book plays on: the rows say so
// at the next frame.
func (t *Tab) Changed() { t.dirty = true }

// refresh takes a new scan, or the places, as they come.
func (t *Tab) refresh(now time.Time) {
	t.mu.Lock()
	gen, books := t.gen, t.scanned
	t.mu.Unlock()
	if gen == t.took && !t.dirty && now.Sub(t.read) < placesEvery {
		return
	}
	t.took, t.dirty, t.read = gen, false, now
	at, _ := t.places.All()
	if at == nil {
		at = map[string]audiobooks.Place{}
	}
	t.books, t.at = order(books, at), at
	t.rows, t.arts = t.rows[:0], t.arts[:0]
	for _, b := range t.books {
		t.rows = append(t.rows, widgets.Row{Title: b.Title, Subtitle: subtitle(b, at[b.Path]),
			Duration: progress(b, at[b.Path])})
		art := widgets.SpecFor(b.Title)
		art.Cover = media.Cover{Artist: b.Author, Album: b.Title, Path: b.Parts[0].Path}
		t.arts = append(t.arts, art)
	}
	for i := range t.rows {
		t.rows[i].Art = &t.arts[i]
	}
	t.list.SetCountFocus(len(t.rows), min(t.list.Focus(), max(len(t.rows)-1, 0)))
}

// order is the books under way first, most recently heard first, then
// those not started, then those finished, each by title as Scan has them.
func order(books []audiobooks.Book, at map[string]audiobooks.Place) []audiobooks.Book {
	var going, fresh, done []audiobooks.Book
	for _, b := range books {
		pl, ok := at[b.Path]
		switch {
		case !ok:
			fresh = append(fresh, b)
		case pl.Finished:
			done = append(done, b)
		default:
			going = append(going, b)
		}
	}
	sortByRecent(going, at)
	return append(append(going, fresh...), done...)
}

func sortByRecent(bs []audiobooks.Book, at map[string]audiobooks.Place) {
	for i := 1; i < len(bs); i++ {
		for j := i; j > 0 && at[bs[j].Path].Updated.After(at[bs[j-1].Path].Updated); j-- {
			bs[j], bs[j-1] = bs[j-1], bs[j]
		}
	}
}

// subtitle is a book's author and how far through it is.
func subtitle(b audiobooks.Book, pl audiobooks.Place) string {
	var where string
	switch {
	case pl.Finished:
		where = "Finished"
	case pl.Part != "":
		where = hours(b.Left(pl)) + " left"
	default:
		where = hours(b.Duration)
	}
	if b.Author == "" {
		return where
	}
	return b.Author + " · " + where
}

// progress is how far through a book is, as a percentage, for one under way.
func progress(b audiobooks.Book, pl audiobooks.Place) string {
	if pl.Part == "" || pl.Finished || b.Duration <= 0 {
		return ""
	}
	done := b.Duration - b.Left(pl)
	return strconv.Itoa(int(100*done/b.Duration)) + "%"
}

// hours is a length in hours and minutes: "45 min", "9 hr 12 min".
func hours(d time.Duration) string {
	m := max(int((d+30*time.Second)/time.Minute), 1)
	if m < 60 {
		return strconv.Itoa(m) + " min"
	}
	h, m := m/60, m%60
	if m == 0 {
		return strconv.Itoa(h) + " hr"
	}
	return strconv.Itoa(h) + " hr " + strconv.Itoa(m) + " min"
}

// Handle plays a book from its place with A, starts it over with X, and
// rescans with Y.
func (t *Tab) Handle(ev input.Event, now time.Time) bool {
	if ev.Kind == input.Press {
		switch ev.Button {
		case input.Y:
			t.Rescan()
			t.nav.Toast(screens.Scanning)
			return true
		case input.A, input.X:
			if len(t.books) == 0 {
				return false
			}
			b := t.books[t.list.Focus()]
			pl := t.at[b.Path]
			if ev.Button == input.X {
				t.places.Forget(b.Path)
				pl = audiobooks.Place{}
				t.dirty = true
				t.nav.Toast("Starting “" + b.Title + "” over")
			}
			part, at := b.Start(pl)
			t.p.SetShuffle(false)
			t.p.PlayAt(b.Tracks(), part, at)
			return true
		}
	}
	return t.list.Move(ev.Button, now)
}

// Animating reports whether focus is moving, or a scan is under way: frames
// keep coming so its books show as soon as it ends.
func (t *Tab) Animating(now time.Time) bool {
	t.mu.Lock()
	scanning := t.scanning || t.gen != t.took
	t.mu.Unlock()
	return scanning || t.list.Animating(now)
}

var (
	bookHints  = []widgets.Hint{{Key: "A", Label: "Play"}, {Key: "X", Label: "Start over"}, {Key: "Y", Label: "Rescan"}, {Key: "L1/R1", Label: "Tabs"}}
	emptyHints = []widgets.Hint{{Key: "Y", Label: "Rescan"}, {Key: "L1/R1", Label: "Tabs"}}
)

// Draw renders the frame.
func (t *Tab) Draw(u *ui.Context) {
	t.refresh(u.Now)
	widgets.DrawBackground(u, widgets.TabGround)
	hints := bookHints
	if len(t.rows) == 0 {
		hints = emptyHints
		t.mu.Lock()
		scanning := t.gen == 0
		t.mu.Unlock()
		s := "No audiobooks yet. Put .m4b files, or a folder for each book, in " + t.dir + ", then press Y."
		if scanning {
			s = "Looking for audiobooks…"
		}
		screens.Message(u, &t.msg, s, gfx.Rect{X: screens.Side, Y: screens.ContentTop, W: t.list.CellW,
			H: screens.ContentBot - screens.ContentTop})
	} else {
		playing := t.p.State().Track.Path
		u.PushClip(screens.GlowArea)
		t.list.Draw(u.Now, screens.Side, screens.ContentTop, func(i int, x, y, k float32) {
			r := t.rows[i]
			r.Playing = playing != "" && audiobooks.BookOf(playing) == t.books[i].Path
			box := gfx.Rect{X: x, Y: y, W: t.list.CellW, H: widgets.RowHeight}
			widgets.RowGlow(u, box, widgets.RowRadius, k)
			u.PushClip(screens.Below(screens.ContentTop))
			widgets.ListRow(u, r, box, k)
			u.PopClip()
		})
		u.PopClip()
	}
	widgets.StatusBar(u)
	widgets.Tabs(u, widgets.TabAudiobooks)
	widgets.HintBar(u, hints)
}
