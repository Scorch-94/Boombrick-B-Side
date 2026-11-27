package settings

import (
	"path/filepath"
	"strconv"
	"time"

	"github.com/ujjwal313/boombrick/internal/library"
	"github.com/ujjwal313/boombrick/internal/platform/input"
	"github.com/ujjwal313/boombrick/internal/screens"
	"github.com/ujjwal313/boombrick/internal/theme"
	"github.com/ujjwal313/boombrick/internal/ui"
	"github.com/ujjwal313/boombrick/internal/ui/anim"
	"github.com/ujjwal313/boombrick/internal/ui/gfx"
	"github.com/ujjwal313/boombrick/internal/ui/widgets"
)

// Library is what the Library page manages: the library, and deleting songs
// from the card.
type Library interface {
	Index() *library.Index
	// Downloaded reports whether t is a file Boombrick downloaded.
	Downloaded(t *library.Track) bool
	// Delete deletes tracks' files from the card and scans the library
	// again, reporting how many it deleted.
	Delete(tracks []*library.Track) int
}

// The Library page's views, chosen with L2 and R2.
const (
	dupesView = iota
	albumsView
	songsView
	viewCount
)

var viewNames = []string{"Duplicates", "Albums", "Songs"}

// libraryPage is the Library page: the library's duplicate songs, its
// albums and its songs, each deleted from the card with A after a dialog.
// Duplicates starts with Delete all duplicate downloads: every song
// Boombrick downloaded that the library also has from elsewhere.
type libraryPage struct {
	lib    Library
	ix     *library.Index           // the index the items were made from
	gone   map[string]bool          // files deleted since, left out until the scan catches up
	items  [viewCount][]libraryItem // each view's rows
	counts []string
	view   int
	focus  [viewCount]int
	scroll [viewCount]anim.Value
	dialog *confirm
	doomed []*library.Track // what the open dialog deletes
}

// libraryItem is a row and the songs A deletes from it.
type libraryItem struct {
	row    widgets.Setting
	tracks []*library.Track
	title  string // what the dialog calls it
	all    bool   // Delete all duplicate downloads
}

func newLibraryPage(lib Library) *libraryPage {
	p := &libraryPage{lib: lib, gone: map[string]bool{}}
	p.read()
	return p
}

// read makes the rows from the library as it is now, leaving out files
// deleted since it was scanned; when the scan has caught up, a new index
// comes and they are forgotten.
func (p *libraryPage) read() {
	ix := p.lib.Index()
	if ix != p.ix {
		p.ix, p.gone = ix, map[string]bool{}
	}
	here := func(ts []*library.Track) []*library.Track {
		var out []*library.Track
		for _, t := range ts {
			if !p.gone[t.Path] {
				out = append(out, t)
			}
		}
		return out
	}
	// where says a copy is a download, or names its folder when that isn't
	// just its album's name, to tell copies apart.
	where := func(t *library.Track) string {
		if p.lib.Downloaded(t) {
			return "Downloaded"
		}
		if dir := filepath.Base(filepath.Dir(t.Path)); dir != t.Album {
			return dir
		}
		return ""
	}

	// Duplicates: every copy of each song the library has twice or more,
	// the downloads last in each.
	var dupes, extra []libraryItem
	var spare []*library.Track
	dupPaths := map[string]bool{}
	if ix != nil {
		for _, g := range ix.Duplicates() {
			g = here(g)
			if len(g) < 2 {
				continue
			}
			var mine, got []*library.Track
			for _, t := range g {
				if p.lib.Downloaded(t) {
					got = append(got, t)
				} else {
					mine = append(mine, t)
				}
			}
			if len(mine) > 0 {
				spare = append(spare, got...)
			}
			for i, t := range append(mine, got...) {
				dupPaths[t.Path] = true
				dupes = append(dupes, libraryItem{
					row: widgets.Setting{Icon: "music", Label: t.Title,
						Detail: join(t.Artist, t.Album, where(t)),
						Value:  strconv.Itoa(i+1) + " of " + strconv.Itoa(len(g))},
					tracks: []*library.Track{t}, title: "“" + t.Title + "”"})
			}
		}
	}
	if len(spare) > 0 {
		extra = append(extra, libraryItem{
			row: widgets.Setting{Icon: "trash", Label: "Delete all duplicate downloads",
				Detail: "Downloads you also have elsewhere in your library", Value: songs(len(spare))},
			tracks: spare, all: true})
	}
	p.items[dupesView] = append(extra, dupes...)

	// Albums, saying which are duplicates.
	p.items[albumsView] = p.items[albumsView][:0]
	p.items[songsView] = p.items[songsView][:0]
	if ix != nil {
		for _, a := range ix.Albums {
			var ts []*library.Track
			for i := range a.Tracks {
				ts = append(ts, &a.Tracks[i])
			}
			ts = here(ts)
			if len(ts) == 0 {
				continue
			}
			d := 0
			for _, t := range ts {
				if dupPaths[t.Path] {
					d++
				}
			}
			value := ""
			switch {
			case d == len(ts):
				value = "Duplicate"
			case d > 0:
				value = strconv.Itoa(d) + " duplicate"
				if d > 1 {
					value += "s"
				}
			}
			p.items[albumsView] = append(p.items[albumsView], libraryItem{
				row:    widgets.Setting{Icon: "folder", Label: a.Title, Detail: join(a.Artist, songs(len(ts))), Value: value},
				tracks: ts, title: "“" + a.Title + "”"})
		}
		for _, t := range here(ix.Songs) {
			value := screens.Clock(t.Duration)
			if dupPaths[t.Path] {
				value = "Duplicate"
			}
			p.items[songsView] = append(p.items[songsView], libraryItem{
				row:    widgets.Setting{Icon: "music", Label: t.Title, Detail: join(t.Artist, t.Album, where(t)), Value: value},
				tracks: []*library.Track{t}, title: "“" + t.Title + "”"})
		}
	}
	nd := len(dupes)
	p.counts = []string{screens.Count(nd), screens.Count(len(p.items[albumsView])), screens.Count(len(p.items[songsView]))}
	for v := range p.items {
		p.focus[v] = min(p.focus[v], max(len(p.items[v])-1, 0))
	}
}

// join is the parts that aren't empty, with " · " between.
func join(parts ...string) string {
	out := ""
	for _, s := range parts {
		if s == "" {
			continue
		}
		if out != "" {
			out += " · "
		}
		out += s
	}
	return out
}

func songs(n int) string {
	if n == 1 {
		return "1 song"
	}
	return screens.Count(n) + " songs"
}

// handle moves, changes view, and opens the dialog to delete what A is on,
// passing input to the dialog while it's open.
func (p *libraryPage) handle(s *Screen, ev input.Event, now time.Time) bool {
	if p.dialog != nil {
		if p.dialog.handle(ev, now) && p.dialog.confirmed {
			p.delete(s)
		}
		return true
	}
	if ev.Kind == input.Release {
		return false
	}
	items := p.items[p.view]
	switch ev.Button {
	case input.L2, input.R2:
		if ev.Kind != input.Press {
			return false
		}
		next := p.view + 1
		if ev.Button == input.L2 {
			next = p.view - 1
		}
		if next < 0 || next >= viewCount {
			return false
		}
		p.view = next
		return true
	case input.Up, input.Down:
		next := p.focus[p.view] + 1
		if ev.Button == input.Up {
			next = p.focus[p.view] - 1
		}
		if next < 0 || next >= len(items) {
			return false
		}
		p.focus[p.view] = next
		return true
	case input.A:
		if ev.Kind != input.Press || len(items) == 0 {
			return false
		}
		it := items[p.focus[p.view]]
		p.doomed = it.tracks
		p.dialog = newConfirm(deleteDialog(it), now)
		p.dialog.choose(1, now) // Cancel first: a delete can't be undone
		return true
	}
	return false
}

// deleteDialog asks whether to delete it's songs.
func deleteDialog(it libraryItem) widgets.Dialog {
	d := widgets.Dialog{Icon: "trash", IconColor: theme.GoldSoft,
		Pills: []widgets.DialogPill{{Label: "Delete"}, {Label: "Cancel"}}}
	n := len(it.tracks)
	switch {
	case it.all:
		d.Title = "Delete " + songs(n) + "?"
		d.Body = "Deletes the songs Boombrick downloaded that you also have elsewhere in your library. Your other copies stay. This can't be undone."
	case n == 1:
		d.Title = "Delete " + it.title + "?"
		d.Body = "Deletes " + filepath.Base(it.tracks[0].Path) + " from the SD card. This can't be undone."
	default:
		d.Title = "Delete " + it.title + "?"
		d.Body = "Deletes its " + songs(n) + " from the SD card. This can't be undone."
	}
	return d
}

// delete deletes what the dialog asked about, says how it went, and leaves
// those files out of the rows until the scan has caught up.
func (p *libraryPage) delete(s *Screen) {
	n := p.lib.Delete(p.doomed)
	for _, t := range p.doomed {
		p.gone[t.Path] = true
	}
	if s.opts.Toast != nil {
		switch {
		case n == len(p.doomed):
			s.opts.Toast("Deleted " + songs(n))
		case n == 0:
			s.opts.Toast("Couldn’t delete " + songs(len(p.doomed)))
		default:
			s.opts.Toast("Deleted " + songs(n) + "; " + strconv.Itoa(len(p.doomed)-n) + " couldn’t be")
		}
	}
	p.doomed = nil
	p.read()
}

// back closes the dialog, or the page.
func (p *libraryPage) back(s *Screen, now time.Time) bool {
	if p.dialog != nil {
		p.dialog.close(now)
		return true
	}
	s.library = nil
	return true
}

func (p *libraryPage) animating(now time.Time) bool {
	return !p.scroll[p.view].Done(now) || p.dialog != nil && p.dialog.animating(now)
}

var (
	libraryHints = []widgets.Hint{{Key: "A", Label: "Delete"}, {Key: "L2/R2", Label: "View"}, {Key: "B", Label: "Back"}}
	emptyHints   = []widgets.Hint{{Key: "L2/R2", Label: "View"}, {Key: "B", Label: "Back"}}
)

// emptyText is what a view with no rows says.
var emptyText = [viewCount]string{"No duplicates: every song is in your library once.", "No albums.", "No songs."}

func (p *libraryPage) draw(s *Screen, u *ui.Context) {
	if p.lib.Index() != p.ix {
		p.read()
	}
	widgets.StatusBar(u)
	header(u, "Library")
	top := listTop(u)
	widgets.Filters(u, viewNames, p.counts, p.view, side, top)
	top += widgets.FiltersHeight + 8
	view := gfx.Rect{X: 0, Y: top, W: theme.ScreenW, H: screens.ContentBot - top}
	items := p.items[p.view]
	if len(items) == 0 {
		u.Text(emptyText[p.view], labelText, side, top+16, theme.Ink2)
	}
	h := float64(widgets.SettingDetailHeight)
	f := p.focus[p.view]
	sc := &p.scroll[p.view]
	to := max(sc.Target(), float64(f+1)*h-float64(view.H))
	to = min(to, float64(f)*h)
	sc.To(to, u.Now, anim.Focus, anim.EaseOutCubic)
	y := top - float32(sc.At(u.Now))
	first := max(int((float64(top-y))/h)-1, 0)
	u.PushClip(view)
	for i := first; i < len(items); i++ {
		box := gfx.Rect{X: side - rowInset, Y: y + float32(i)*float32(h), W: theme.ScreenW - 2*(side-rowInset), H: widgets.SettingDetailHeight}
		if box.Y > view.Y+view.H {
			break
		}
		k := float32(0)
		if i == f {
			k = 1
		}
		widgets.DrawSetting(u, items[i].row, box, k, i == len(items)-1)
	}
	u.PopClip()
	if p.dialog != nil {
		if p.dialog.gone(u.Now) {
			p.dialog = nil
		} else {
			p.dialog.draw(u)
			widgets.HintBar(u, p.dialog.hints())
			return
		}
	}
	if len(items) == 0 {
		widgets.HintBar(u, emptyHints)
		return
	}
	widgets.HintBar(u, libraryHints)
}
