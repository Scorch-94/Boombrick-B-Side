package settings

import (
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"

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

	// The search, which X opens: the keyboard on the left, the view's rows
	// that match on the right.
	searching bool
	kb        widgets.Keyboard
	query     string
	onHits    bool                // focus is on the matches, not the keyboard
	keys      [viewCount][]string // each row's folded words, to match
	hits      []int               // the shown view's rows that match, in order
	hitFocus  int
	hitScroll anim.Value
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
		p.keys[v] = make([]string, len(p.items[v]))
		for i, it := range p.items[v] {
			if !it.all {
				p.keys[v][i] = fold(it.row.Label + "\x00" + it.row.Detail)
			}
		}
	}
	p.find()
}

// fold lowers s and strips its accents, so "beyonce" finds "Beyoncé".
func fold(s string) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(s) {
		if !unicode.Is(unicode.Mn, r) {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}

// find lists the shown view's rows matching every word typed.
func (p *libraryPage) find() {
	p.hits = p.hits[:0]
	words := strings.Fields(fold(p.query))
	if len(words) > 0 {
	ROWS:
		for i, k := range p.keys[p.view] {
			if k == "" {
				continue
			}
			for _, w := range words {
				if !strings.Contains(k, w) {
					continue ROWS
				}
			}
			p.hits = append(p.hits, i)
		}
	}
	p.hitFocus = min(p.hitFocus, max(len(p.hits)-1, 0))
	if len(p.hits) == 0 && p.onHits {
		p.toKeyboard(time.Time{})
	}
}

func (p *libraryPage) toKeyboard(now time.Time) {
	p.onHits = false
	p.kb.SetActive(true, now)
}

func (p *libraryPage) toHits(now time.Time) bool {
	if len(p.hits) == 0 {
		return false
	}
	p.onHits = true
	p.kb.SetActive(false, now)
	return true
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
	if p.searching {
		return p.handleSearch(ev, now)
	}
	items := p.items[p.view]
	switch ev.Button {
	case input.X:
		if ev.Kind != input.Press {
			return false
		}
		p.searching, p.query, p.onHits = true, "", false
		p.kb = widgets.NewKeyboard()
		p.find()
		return true
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
		p.find()
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
		p.ask(items[p.focus[p.view]], now)
		return true
	}
	return false
}

// ask opens the dialog to delete it's songs, on Cancel: a delete can't be
// undone.
func (p *libraryPage) ask(it libraryItem, now time.Time) {
	p.doomed = it.tracks
	p.dialog = newConfirm(deleteDialog(it), now)
	p.dialog.choose(1, now)
}

// handleSearch types on the keyboard, changes view, and on the matches moves
// and deletes with A.
func (p *libraryPage) handleSearch(ev input.Event, now time.Time) bool {
	if ev.Kind == input.Press && (ev.Button == input.L2 || ev.Button == input.R2) {
		next := p.view + 1
		if ev.Button == input.L2 {
			next = p.view - 1
		}
		if next < 0 || next >= viewCount {
			return false
		}
		p.view, p.hitFocus = next, 0
		p.find()
		return true
	}
	if p.onHits {
		switch ev.Button {
		case input.A:
			if ev.Kind == input.Press {
				p.ask(p.items[p.view][p.hits[p.hitFocus]], now)
				return true
			}
		case input.Up, input.Down:
			next := p.hitFocus + 1
			if ev.Button == input.Up {
				next = p.hitFocus - 1
			}
			if next < 0 || next >= len(p.hits) {
				return false
			}
			p.hitFocus = next
			return true
		case input.Left:
			p.toKeyboard(now)
			return true
		}
		return false
	}
	if ev.Kind == input.Press {
		switch ev.Button {
		case input.A:
			switch act, r := p.kb.Press(); act {
			case widgets.TypeKey:
				p.typeRune(r)
			case widgets.SpaceKey:
				p.space()
			case widgets.BackspaceKey:
				p.backspace()
			case widgets.SearchKey:
				p.toHits(now)
			}
			return true
		case input.X:
			return p.space()
		case input.Y:
			return p.toHits(now)
		}
	}
	if p.kb.Move(ev.Button, now) {
		return true
	}
	return ev.Button == input.Right && p.toHits(now)
}

func (p *libraryPage) typeRune(r rune) {
	if utf8.RuneCountInString(p.query) < 80 {
		p.query += string(r)
		p.find()
	}
}

// space adds a space between words: never a leading one or two together.
func (p *libraryPage) space() bool {
	if p.query == "" || strings.HasSuffix(p.query, " ") {
		return false
	}
	p.typeRune(' ')
	return true
}

func (p *libraryPage) backspace() bool {
	if p.query == "" {
		return false
	}
	_, n := utf8.DecodeLastRuneInString(p.query)
	p.query = p.query[:len(p.query)-n]
	p.find()
	return true
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

// back closes the dialog; in the search, goes from the matches to the
// keyboard, deletes what's typed, and with nothing typed closes the search;
// and otherwise closes the page.
func (p *libraryPage) back(s *Screen, now time.Time) bool {
	switch {
	case p.dialog != nil:
		p.dialog.close(now)
	case p.searching && p.onHits:
		p.toKeyboard(now)
	case p.searching && p.backspace():
	case p.searching:
		p.searching = false
	default:
		s.library = nil
	}
	return true
}

func (p *libraryPage) animating(now time.Time) bool {
	return !p.scroll[p.view].Done(now) || !p.hitScroll.Done(now) || p.searching && p.kb.Animating(now) ||
		p.dialog != nil && p.dialog.animating(now)
}

var (
	libraryHints = []widgets.Hint{{Key: "A", Label: "Delete"}, {Key: "X", Label: "Search"}, {Key: "L2/R2", Label: "View"},
		{Key: "B", Label: "Back"}}
	emptyHints  = []widgets.Hint{{Key: "X", Label: "Search"}, {Key: "L2/R2", Label: "View"}, {Key: "B", Label: "Back"}}
	typingHints = []widgets.Hint{{Key: "A", Label: "Type"}, {Key: "B", Label: "Delete"}, {Key: "X", Label: "Space"},
		{Key: "Y", Label: "To results"}}
	aloneHints = []widgets.Hint{{Key: "A", Label: "Type"}, {Key: "B", Label: "Delete"}, {Key: "X", Label: "Space"},
		{Key: "L2/R2", Label: "View"}}
	hitHints = []widgets.Hint{{Key: "A", Label: "Delete"}, {Key: "B", Label: "Keyboard"}, {Key: "L2/R2", Label: "View"}}
)

// emptyText is what a view with no rows says.
var emptyText = [viewCount]string{"No duplicates: every song is in your library once.", "No albums.", "No songs."}

func (p *libraryPage) draw(s *Screen, u *ui.Context) {
	if p.lib.Index() != p.ix {
		p.read()
	}
	widgets.StatusBar(u)
	if p.searching {
		// The keyboard needs the room: the filters go up where the title was.
		widgets.Filters(u, viewNames, p.counts, p.view, side, headTop)
		p.drawSearch(u, headTop+widgets.FiltersHeight+8)
		return
	}
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

// drawSearch draws the keyboard and, beside it, the view's rows that match.
func (p *libraryPage) drawSearch(u *ui.Context, top float32) {
	p.kb.Draw(u, side, top, p.query)
	x := float32(side + widgets.KeyboardW + 20)
	w := float32(theme.ScreenW-side+rowInset) - x
	view := gfx.Rect{X: x, Y: top, W: w, H: screens.ContentBot - top}
	if len(p.hits) == 0 {
		msg := "Type to find " + strings.ToLower(viewNames[p.view]) + "."
		if q := strings.TrimSpace(p.query); q != "" {
			msg = "Nothing matches “" + q + "”"
		}
		u.Text(u.Fit(msg, labelText, w-20), labelText, x+20, top+16, theme.Ink2)
	}
	h := float64(widgets.SettingDetailHeight)
	f := p.hitFocus
	to := max(p.hitScroll.Target(), float64(f+1)*h-float64(view.H))
	to = min(to, float64(f)*h)
	p.hitScroll.To(to, u.Now, anim.Focus, anim.EaseOutCubic)
	y := top - float32(p.hitScroll.At(u.Now))
	u.PushClip(view)
	for j, i := range p.hits {
		box := gfx.Rect{X: x, Y: y + float32(j)*float32(h), W: w, H: widgets.SettingDetailHeight}
		if box.Y+box.H < view.Y {
			continue
		}
		if box.Y > view.Y+view.H {
			break
		}
		k := float32(0)
		if p.onHits && j == f {
			k = 1
		}
		widgets.DrawSetting(u, p.items[p.view][i].row, box, k, j == len(p.hits)-1)
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
	switch {
	case p.onHits:
		widgets.HintBar(u, hitHints)
	case len(p.hits) > 0:
		widgets.HintBar(u, typingHints)
	default:
		widgets.HintBar(u, aloneHints)
	}
}
