package browse

import (
	"fmt"
	"testing"

	"github.com/ujjwal313/boombrick/internal/library"
	"github.com/ujjwal313/boombrick/internal/platform/input"
	"github.com/ujjwal313/boombrick/internal/screens"
	"github.com/ujjwal313/boombrick/internal/theme"
	"github.com/ujjwal313/boombrick/internal/ui/anim"
	"github.com/ujjwal313/boombrick/internal/ui/uitest"
)

// The Library's albums show four across and two down, both rows whole with
// room for the focus ring, so moving onto the second row doesn't scroll.
func TestLibraryAlbumGridFourByTwo(t *testing.T) {
	var albums []library.Album
	for i := range 10 {
		albums = append(albums, library.Album{Artist: fmt.Sprintf("Artist %d", i), Title: fmt.Sprintf("Album %02d", i),
			Tracks: []library.Track{{Path: fmt.Sprintf("/m/%d/1.flac", i), Title: "One"}}})
	}
	l := New(&fakeNav{}, &fakePlayer{}, "/m", nil)
	l.SetIndex(library.NewIndex(albums))
	render(t, l, t0)
	g := &l.views[albumsFilter]
	if g.Cols != 4 {
		t.Fatalf("%d columns", g.Cols)
	}
	if need := 2*g.CellH + g.GapY + 2*g.Pad; need > g.ViewH {
		t.Fatalf("two rows need %.1f, the view is %.1f", need, g.ViewH)
	}
	if w := float32(4*screens.LibCellW + 3*screens.LibGapX); w > float32(theme.ScreenW-2*screens.Side) {
		t.Fatalf("four columns are %v wide", w)
	}
	l.Handle(press(input.Down), t0)
	if g.Focus() != 4 {
		t.Fatalf("focus %d after Down", g.Focus())
	}
	uitest.Golden(t, "library-albums-4x2", render(t, l, t0.Add(anim.Focus)))
}
