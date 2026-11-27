package helper

import (
	"context"
	"slices"
	"strings"
	"time"

	librespot "github.com/devgianlu/go-librespot"
	extmetadatapb "github.com/devgianlu/go-librespot/proto/spotify/extendedmetadata"
	metadatapb "github.com/devgianlu/go-librespot/proto/spotify/metadata"

	"github.com/ujjwal313/boombrick/internal/spotify"
)

const (
	// artistsSet is the collection set the user's followed artists are in.
	artistsSet = "artist"
	// releaseEnds is how many of an artist's albums, and of its singles, are
	// looked at from each end of its list: the newest are at one end, which
	// one the metadata doesn't say.
	releaseEnds = 3
	// releaseWindow is how far back a release counts as latest, and
	// maxReleases how many are listed.
	releaseWindow = 180 * 24 * time.Hour
	maxReleases   = 20
)

// Releases lists the latest albums, EPs and singles of the artists the user
// follows, newest first: released within releaseWindow and not yet to come.
func (s *Librespot) Releases(ctx context.Context) ([]spotify.Release, error) {
	sess, err := s.ready(ctx)
	if err != nil {
		return nil, err
	}
	artists, err := collection(ctx, sess, artistsSet)
	if err != nil {
		return nil, classify(err)
	}
	sp := sess.Spclient()
	ameta, err := metadata[metadatapb.Artist](ctx, sp, artists, extmetadatapb.ExtensionKind_ARTIST_V4)
	if err != nil {
		return nil, classify(err)
	}
	var albums []string
	seen := map[string]bool{}
	for _, a := range artists {
		m := ameta[a]
		if m == nil {
			continue
		}
		for _, groups := range [][]*metadatapb.AlbumGroup{m.GetAlbumGroup(), m.GetSingleGroup()} {
			for _, g := range ends(groups, releaseEnds) {
				for _, al := range g.GetAlbum() {
					if len(al.GetGid()) != 16 {
						continue
					}
					u := librespot.SpotifyIdFromGid(librespot.SpotifyIdTypeAlbum, al.GetGid()).Uri()
					if !seen[u] {
						seen[u] = true
						albums = append(albums, u)
					}
					break // a group's first album is the release; the rest are its other versions
				}
			}
		}
	}
	meta, err := metadata[metadatapb.Album](ctx, sp, albums, extmetadatapb.ExtensionKind_ALBUM_V4)
	if err != nil {
		return nil, classify(err)
	}
	out := make([]spotify.Release, 0, len(albums))
	for _, u := range albums {
		if m := meta[u]; m != nil {
			out = append(out, release(u, m))
		}
	}
	return latest(out, time.Now()), nil
}

// ends is the first n and the last n of gs, each once.
func ends[T any](gs []T, n int) []T {
	if len(gs) <= 2*n {
		return gs
	}
	return append(slices.Clip(gs[:n]), gs[len(gs)-n:]...)
}

// release is album u as a release.
func release(u string, m *metadatapb.Album) spotify.Release {
	var names []string
	for _, a := range m.GetArtist() {
		if n := a.GetName(); n != "" {
			names = append(names, n)
		}
	}
	tracks := 0
	for _, d := range m.GetDisc() {
		tracks += len(d.GetTrack())
	}
	img := cover(m.GetCoverGroup().GetImage())
	if img == "" {
		img = cover(m.GetCover())
	}
	r := spotify.Release{URI: u, Name: m.GetName(), Artist: strings.Join(names, ", "), Cover: img, Tracks: tracks,
		Type: releaseType(m, tracks)}
	if d := m.GetDate(); d.GetYear() > 0 {
		r.Released = time.Date(int(d.GetYear()), time.Month(max(d.GetMonth(), 1)), int(max(d.GetDay(), 1)), 0, 0, 0, 0,
			time.UTC).Unix()
	}
	return r
}

// releaseType says what a release is: "Album", "EP" or "Single".
func releaseType(m *metadatapb.Album, tracks int) string {
	switch m.GetType() {
	case metadatapb.Album_SINGLE:
		if tracks > 3 {
			return "EP"
		}
		return "Single"
	case metadatapb.Album_EP:
		return "EP"
	}
	return "Album"
}

// latest is rs released within releaseWindow before now and not after it,
// newest first, a release kept once though two artists followed are on it,
// and at most maxReleases of them.
func latest(rs []spotify.Release, now time.Time) []spotify.Release {
	from, to := now.Add(-releaseWindow).Unix(), now.Unix()
	var out []spotify.Release
	seen := map[string]bool{}
	for _, r := range rs {
		if r.Released < from || r.Released > to || seen[r.URI] {
			continue
		}
		seen[r.URI] = true
		out = append(out, r)
	}
	slices.SortStableFunc(out, func(a, b spotify.Release) int {
		if a.Released != b.Released {
			return int(b.Released - a.Released)
		}
		return strings.Compare(a.Name, b.Name)
	})
	if len(out) > maxReleases {
		out = out[:maxReleases]
	}
	return out
}
