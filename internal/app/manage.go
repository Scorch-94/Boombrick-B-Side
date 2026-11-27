package app

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/ujjwal313/boombrick/internal/library"
)

// libraryManager is the Library page's library: the index, and deleting
// songs from the card.
type libraryManager struct{ a *App }

func (m libraryManager) Index() *library.Index { return m.a.ix }

// Downloaded reports whether t is in the folder downloads are filed in.
func (m libraryManager) Downloaded(t *library.Track) bool {
	if m.a.opts.MusicDir == "" {
		return false
	}
	dir := filepath.Join(m.a.opts.MusicDir, "Boombrick") + string(filepath.Separator)
	return strings.HasPrefix(t.Path, dir)
}

// Delete deletes tracks' files, with their .lrc lyrics, from the card; a
// download is forgotten by the queue too. Folders left empty go, up to the
// music folder, and the library is scanned again.
func (m libraryManager) Delete(tracks []*library.Track) int {
	n := 0
	music := filepath.Clean(m.a.opts.MusicDir)
	for _, t := range tracks {
		if t == nil || t.Path == "" {
			continue
		}
		if err := os.Remove(t.Path); err != nil && !os.IsNotExist(err) {
			if m.a.opts.Log != nil {
				m.a.opts.Log.Warn("delete", "path", t.Path, "err", err)
			}
			continue
		}
		n++
		os.Remove(strings.TrimSuffix(t.Path, filepath.Ext(t.Path)) + ".lrc")
		if m.a.opts.Downloads != nil {
			m.a.opts.Downloads.ForgetFile(t.Path)
		}
		removeEmpty(filepath.Dir(t.Path), music)
	}
	if n > 0 {
		m.a.rescan()
	}
	return n
}

// removeEmpty removes dir and the folders above it while they're empty,
// stopping at stop. A folder with only its cover picture left counts as
// empty: the picture goes with it.
func removeEmpty(dir, stop string) {
	for dir != stop && strings.HasPrefix(dir, stop+string(filepath.Separator)) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, e := range entries {
			if e.IsDir() || !isPicture(e.Name()) {
				return
			}
		}
		for _, e := range entries {
			os.Remove(filepath.Join(dir, e.Name()))
		}
		if os.Remove(dir) != nil {
			return
		}
		dir = filepath.Dir(dir)
	}
}

func isPicture(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".jpg", ".jpeg", ".png", ".webp", ".bmp", ".gif":
		return true
	}
	return false
}
