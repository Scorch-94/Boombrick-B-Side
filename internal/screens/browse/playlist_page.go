package browse

import (
	"slices"
	"time"

	"github.com/ujjwal313/boombrick/internal/library"
	"github.com/ujjwal313/boombrick/internal/platform/input"
	"github.com/ujjwal313/boombrick/internal/screens"
	"github.com/ujjwal313/boombrick/internal/theme"
	"github.com/ujjwal313/boombrick/internal/ui"
	"github.com/ujjwal313/boombrick/internal/ui/gfx"
	"github.com/ujjwal313/boombrick/internal/ui/text"
	"github.com/ujjwal313/boombrick/internal/ui/widgets"
)

// PlaylistPage is a playlist's songs, laid out as an album's, and, with
// playlists editable, Select edits it: A picks a song up to move it, X
// takes it out, Y renames the playlist and Start deletes it. Each change is
// saved to the file at once.
type PlaylistPage struct {
	*AlbumPage
	pl       *library.Playlist
	editing  bool
	moving   bool
	before   []library.PlaylistEntry // the order before a move, for B to put back
	from     int                     // where the song moving was
	deleting bool                    // the delete dialog is open
}

// NewPlaylistPage opens playlist pl.
func NewPlaylistPage(nav screens.Nav, p Player, pl *library.Playlist) *PlaylistPage {
	return &PlaylistPage{AlbumPage: playlistPage(nav, p, pl), pl: pl}
}

var (
	editHints   = []widgets.Hint{{Key: "A", Label: "Move"}, {Key: "X", Label: "Remove"}, {Key: "Y", Label: "Rename"}, {Key: "Start", Label: "Delete"}, {Key: "Select", Label: "Done"}}
	movingHints = []widgets.Hint{{Key: "▲ ▼", Label: "Move"}, {Key: "A", Label: "Drop here"}, {Key: "B", Label: "Put back"}}
	deleteHints = []widgets.Hint{{Key: "A", Label: "Delete"}, {Key: "B", Label: "Cancel"}}
	listHints   = []widgets.Hint{{Key: "A", Label: "Play"}, {Key: "Y", Label: "Queue"}, {Key: "X", Label: "Add to playlist"}, {Key: "Select", Label: "Edit"}}
)

// editable reports whether the page can edit its playlist.
func (pg *PlaylistPage) editable() bool { return Edits != nil && pg.pl.Editable() }

// Claims Select, to edit, and while editing Start, to delete.
func (pg *PlaylistPage) Claims(b input.Button) bool {
	return pg.editable() && (b == input.Select || pg.editing && b == input.Start)
}

// Back puts a moved song back, closes the delete dialog or ends editing.
func (pg *PlaylistPage) Back() bool {
	switch {
	case pg.deleting:
		pg.deleting = false
	case pg.moving:
		pg.moving = false
		pg.setEntries(pg.before, pg.from)
	case pg.editing:
		pg.editing = false
	default:
		return false
	}
	return true
}

// Handle edits the playlist while editing, and plays, queues and adds its
// songs to other playlists otherwise.
func (pg *PlaylistPage) Handle(ev input.Event, now time.Time) bool {
	if ev.Kind == input.Release {
		return false
	}
	press := ev.Kind == input.Press
	if pg.deleting {
		if press && ev.Button == input.A {
			pg.delete()
		}
		return true
	}
	if !pg.editing {
		if press && ev.Button == input.Select && pg.editable() {
			pg.editing = true
			return true
		}
		return pg.AlbumPage.Handle(ev, now)
	}
	f := pg.list.Focus()
	if pg.moving {
		switch ev.Button {
		case input.Up, input.Down:
			to := f + 1
			if ev.Button == input.Up {
				to = f - 1
			}
			if to < 0 || to >= len(pg.pl.Entries) {
				return false
			}
			es := slices.Clone(pg.pl.Entries)
			es[f], es[to] = es[to], es[f]
			pg.setEntries(es, to)
			return true
		case input.A, input.Select:
			if press {
				pg.moving = false
				pg.save()
			}
			return true
		}
		return true
	}
	if !press {
		return pg.list.Move(ev.Button, now)
	}
	switch ev.Button {
	case input.Select:
		pg.editing = false
	case input.A:
		if len(pg.pl.Entries) > 0 {
			pg.moving, pg.before, pg.from = true, slices.Clone(pg.pl.Entries), f
		}
	case input.X:
		if len(pg.pl.Entries) > 0 {
			gone := pg.rows[f].Title
			pg.setEntries(slices.Delete(slices.Clone(pg.pl.Entries), f, f+1), f)
			if pg.save() {
				pg.nav.Toast("Removed " + gone)
			}
		}
	case input.Y:
		pg.nav.Push(newNameEntry(pg.nav, "Rename playlist", pg.pl.Name, func(name string) bool {
			if err := pg.pl.Rename(Edits.Music(), name); err != nil {
				pg.nav.Toast("Couldn’t rename the playlist")
				return false
			}
			Edits.Saved()
			pg.title = pg.pl.Name
			pg.nav.Pop()
			return true
		}))
	case input.Start:
		pg.deleting = true
	default:
		return pg.list.Move(ev.Button, now)
	}
	return true
}

// setEntries shows es as the playlist's songs, focus on the one at f.
func (pg *PlaylistPage) setEntries(es []library.PlaylistEntry, f int) {
	pg.pl.SetEntries(es)
	fresh := playlistPage(pg.nav, pg.p, pg.pl)
	pg.tracks, pg.rows, pg.length, pg.who = fresh.tracks, fresh.rows, fresh.length, fresh.who
	pg.list.SetCountFocus(len(pg.rows), min(f, max(len(pg.rows)-1, 0)))
}

// save writes the playlist, saying so when it can't.
func (pg *PlaylistPage) save() bool {
	if err := pg.pl.Save(Edits.Music()); err != nil {
		pg.nav.Toast("Couldn’t save the playlist")
		return false
	}
	Edits.Saved()
	return true
}

func (pg *PlaylistPage) delete() {
	if err := pg.pl.Delete(Edits.Music()); err != nil {
		pg.deleting = false
		pg.nav.Toast("Couldn’t delete the playlist")
		return
	}
	Edits.Saved()
	pg.nav.Pop()
	pg.nav.Toast("Deleted " + pg.pl.Name)
}

// Draw renders the page; while editing, the moving song with a grip, and
// the delete dialog over it.
func (pg *PlaylistPage) Draw(u *ui.Context) {
	pg.bar = nil
	switch {
	case pg.deleting:
		pg.bar = deleteHints
	case pg.moving:
		pg.bar = movingHints
	case pg.editing:
		pg.bar = editHints
	case pg.editable():
		pg.bar = listHints
	}
	for i := range pg.rows {
		pg.rows[i].Grip = pg.moving && i == pg.list.Focus()
	}
	pg.AlbumPage.Draw(u)
	if pg.editing && !pg.deleting {
		u.Text("Editing", text.Meta, side, float32(albumTop+albumArt+pad)+u.LineHeight(text.Heading)+2*u.LineHeight(text.Meta), u.Accent.Color)
	}
	if pg.deleting {
		pg.drawDelete(u)
	}
}

func (pg *PlaylistPage) drawDelete(u *ui.Context) {
	h := float32(36 + 50 + 12 + 30 + 36)
	p := widgets.BeginPanel(u, h, 1)
	x, y := p.Box.X+40, p.Box.Y+36
	u.Text(u.Fit("Delete “"+pg.pl.Name+"”?", text.Heading, p.Box.W-80), text.Heading, x, y, theme.Paper)
	y += u.LineHeight(text.Heading) + 12
	about := "The playlist goes from the SD card. Its songs stay."
	if pg.pl.Path != "" && len(pg.pl.Path) > 4 && pg.pl.Path[len(pg.pl.Path)-4:] == ".csv" {
		about = "Its songs go from the playlist file. The songs stay."
	}
	u.Text(u.Fit(about, text.Meta, p.Box.W-80), text.Meta, x, y, theme.Ink2)
	p.End(u)
	// The dialog's hints over the scrim, as the quick menu's.
	u.Fill(gfx.Rect{Y: theme.ScreenH - widgets.HintsHeight, W: theme.ScreenW, H: widgets.HintsHeight}, theme.Ground)
	widgets.HintBar(u, deleteHints)
}
