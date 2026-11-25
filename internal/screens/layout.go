package screens

import (
	"github.com/ujjwal313/boombrick/internal/theme"
	"github.com/ujjwal313/boombrick/internal/ui"
	"github.com/ujjwal313/boombrick/internal/ui/gfx"
	"github.com/ujjwal313/boombrick/internal/ui/text"
	"github.com/ujjwal313/boombrick/internal/ui/views"
	"github.com/ujjwal313/boombrick/internal/ui/widgets"
)

// Layout shared by the tabs' pages, in design pixels.
const (
	ContentTop = widgets.StatusHeight + widgets.TabsHeight
	ContentBot = theme.ScreenH - widgets.HintsHeight
	Side       = 32
	Pad        = 14 // room above a list or grid for the focus ring
	// The album grid: three 212 px cards, 36 apart across and 28 down,
	// centred on the canvas.
	GridArt  = 212
	GridGapX = 36
	GridGapY = 28
	GridX    = (theme.ScreenW - 3*GridArt - 2*GridGapX) / 2
	// The Library's Albums view: four columns and two rows under the
	// filters, both whole with a focus ring's room (2 × card + gap + 2 × Pad
	// fits the view, so moving between them doesn't scroll). The covers are
	// small, so a caption runs the column's width.
	LibArt   = 92
	LibCellW = 176
	LibGapX  = 17
	LibGapY  = 18
	LibGridX = (theme.ScreenW - 4*LibCellW - 3*LibGapX) / 2
	// An album's page: its art in a column on the left with its details and
	// pills at the column's foot, the tracks on the right.
	PageArt   = 200
	PageTop   = ContentTop + Pad
	PageListX = Side + PageArt + Side
	PillGap   = 12
)

// GlowArea is where focus glows may reach. Lists and grids clip their items
// at their own tops, so scrolled ones don't show over what is above. Focus
// glows reach further, over everything between the status bar and the hint
// bar, so they fade out instead of ending at an edge: over a heading, the
// filters, or the tabs above an album page's first track.
var GlowArea = gfx.Rect{X: 0, Y: widgets.StatusHeight, W: theme.ScreenW, H: ContentBot - widgets.StatusHeight}

// Below is the part of the content from y down, where a list or grid
// starting at y draws its items.
func Below(y float32) gfx.Rect { return gfx.Rect{X: 0, Y: y, W: theme.ScreenW, H: ContentBot - y} }

// AlbumGrid is the grid of album cards; LayoutGrid fills in what depends on
// the fonts.
func AlbumGrid() views.Grid {
	return views.Grid{Cols: 3, CellW: GridArt, GapX: GridGapX, GapY: GridGapY, Pad: Pad}
}

// LibraryAlbumGrid is the Library's Albums view: four by two.
func LibraryAlbumGrid() views.Grid {
	return views.Grid{Cols: 4, CellW: LibCellW, GapX: LibGapX, GapY: LibGapY, Pad: Pad}
}

// LayoutLibraryGrid sizes the Library's album cards and view.
func LayoutLibraryGrid(u *ui.Context, g *views.Grid, viewH float32) {
	g.CellH, g.ViewH = widgets.CardHeight(u, LibArt), viewH
}

// LayoutGrid sizes g's cards and view.
func LayoutGrid(u *ui.Context, g *views.Grid, viewH float32) {
	g.CellH, g.ViewH = widgets.CardHeight(u, GridArt), viewH
}

// Heading draws a Heading on the left and a Meta count on the right, as
// Home's rows do.
func Heading(u *ui.Context, title, meta string, y float32) {
	u.Text(u.Fit(title, text.Heading, theme.ScreenW-2*Side-u.TextWidth(meta, text.Meta)-24), text.Heading, Side, y, theme.Paper)
	baseline := y + u.Ascent(text.Heading)
	u.Text(meta, text.Meta, theme.ScreenW-Side-u.TextWidth(meta, text.Meta), u.Baseline(text.Meta, baseline), theme.Ink3)
}

// Message draws s wrapped and centred in box, in ink2. The screen keeps the
// lines in c, so that the same message isn't wrapped every frame.
func Message(u *ui.Context, c *widgets.Wrapped, s string, box gfx.Rect) {
	lines := c.Lines(u, s, text.Body, box.W)
	lh := u.LineHeight(text.Body)
	y := box.Y + (box.H-float32(len(lines))*lh)/2
	for _, l := range lines {
		u.Text(l, text.Body, box.X+(box.W-u.TextWidth(l, text.Body))/2, y, theme.Ink2)
		y += lh
	}
}
