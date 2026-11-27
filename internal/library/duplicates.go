package library

import (
	"slices"
	"time"
)

// Duplicates is the songs the library has more than once: groups of copies
// of one song, each of two or more. Copies are one song when their titles
// match, their artists share one, and their lengths are within 3 seconds
// where both are known, so another recording, as a live one or a remix with
// its own title, isn't counted. Groups go by title, copies in the index's
// song order. The index works them out the first time it's asked.
func (ix *Index) Duplicates() [][]*Track {
	if ix == nil {
		return nil
	}
	ix.dupOnce.Do(func() { ix.dupes = duplicates(ix.Songs) })
	return ix.dupes
}

func duplicates(songs []*Track) [][]*Track {
	byTitle := map[string][]*Track{}
	var titles []string
	for _, t := range songs {
		k := foldText(t.Title)
		if k == "" {
			continue
		}
		if _, ok := byTitle[k]; !ok {
			titles = append(titles, k)
		}
		byTitle[k] = append(byTitle[k], t)
	}
	var out [][]*Track
	for _, k := range titles {
		ts := byTitle[k]
		if len(ts) < 2 {
			continue
		}
		var groups [][]*Track
	TRACKS:
		for _, t := range ts {
			for i, g := range groups {
				if sameSong(g[0], t) {
					groups[i] = append(g, t)
					continue TRACKS
				}
			}
			groups = append(groups, []*Track{t})
		}
		for _, g := range groups {
			if len(g) > 1 {
				out = append(out, g)
			}
		}
	}
	slices.SortStableFunc(out, func(a, b []*Track) int { return Compare(a[0].Title, b[0].Title) })
	return out
}

// sameSong reports whether a and b, of one title, are copies of one song:
// an artist in common, and lengths within 3 seconds when both are known.
func sameSong(a, b *Track) bool {
	if a.Duration > 0 && b.Duration > 0 {
		if d := a.Duration - b.Duration; d > 3*time.Second || d < -3*time.Second {
			return false
		}
	}
	_, as := splitArtists(a.Artist)
	_, bs := splitArtists(b.Artist)
	if len(as) == 0 || len(bs) == 0 {
		return len(as) == len(bs)
	}
	for _, x := range as {
		for _, y := range bs {
			if fx := foldText(x); fx != "" && fx == foldText(y) {
				return true
			}
		}
	}
	return false
}
