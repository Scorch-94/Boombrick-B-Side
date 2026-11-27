package helper

import (
	"slices"
	"testing"
	"time"

	metadatapb "github.com/devgianlu/go-librespot/proto/spotify/metadata"
	"google.golang.org/protobuf/proto"

	"github.com/ujjwal313/boombrick/internal/spotify"
)

// An artist's newest releases are at one end of its list: both ends are
// looked at, each release once.
func TestEnds(t *testing.T) {
	xs := []int{1, 2, 3, 4, 5, 6, 7, 8, 9}
	if got := ends(xs, 3); !slices.Equal(got, []int{1, 2, 3, 7, 8, 9}) {
		t.Fatalf("ends %v", got)
	}
	if got := ends(xs[:5], 3); !slices.Equal(got, xs[:5]) {
		t.Fatalf("a short list: %v", got)
	}
	if !slices.Equal(xs, []int{1, 2, 3, 4, 5, 6, 7, 8, 9}) {
		t.Fatal("ends wrote over its list")
	}
}

// Latest releases are those of the last half year, not yet-to-come ones,
// newest first, each once, at most maxReleases.
func TestLatest(t *testing.T) {
	now := time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)
	day := func(d int) int64 { return now.AddDate(0, 0, -d).Unix() }
	rs := []spotify.Release{
		{URI: "a", Name: "Old", Released: day(400)},
		{URI: "b", Name: "Two days", Released: day(2)},
		{URI: "c", Name: "Today", Released: day(0)},
		{URI: "b", Name: "Two days", Released: day(2)}, // two artists followed are on it
		{URI: "d", Name: "Next week", Released: day(-7)},
		{URI: "e", Name: "Month", Released: day(30)},
	}
	got := latest(rs, now)
	var names []string
	for _, r := range got {
		names = append(names, r.Name)
	}
	if !slices.Equal(names, []string{"Today", "Two days", "Month"}) {
		t.Fatalf("latest %v", names)
	}
	var many []spotify.Release
	for i := range 30 {
		many = append(many, spotify.Release{URI: string(rune('A' + i)), Released: day(i)})
	}
	if n := len(latest(many, now)); n != maxReleases {
		t.Fatalf("%d releases", n)
	}
}

// A release's name, artists, kind, date and songs come from its metadata.
func TestRelease(t *testing.T) {
	m := &metadatapb.Album{
		Name:   proto.String("Holiday Favorites"),
		Artist: []*metadatapb.Artist{{Name: proto.String("Lindsey Stirling")}, {Name: proto.String("Egzod")}},
		Type:   metadatapb.Album_SINGLE.Enum(),
		Date:   &metadatapb.Date{Year: proto.Int32(2026), Month: proto.Int32(11), Day: proto.Int32(26)},
		Disc:   []*metadatapb.Disc{{Track: []*metadatapb.Track{{}, {}}}},
	}
	r := release("spotify:album:x", m)
	if r.Name != "Holiday Favorites" || r.Artist != "Lindsey Stirling, Egzod" || r.Type != "Single" || r.Tracks != 2 ||
		r.Released != time.Date(2026, 11, 26, 0, 0, 0, 0, time.UTC).Unix() {
		t.Fatalf("release %+v", r)
	}
	m.Disc[0].Track = make([]*metadatapb.Track, 5)
	if r := release("u", m); r.Type != "EP" {
		t.Fatalf("a five-song single is %q", r.Type)
	}
	m.Type = metadatapb.Album_ALBUM.Enum()
	m.Date = &metadatapb.Date{Year: proto.Int32(2025)}
	if r := release("u", m); r.Type != "Album" || r.Released != time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).Unix() {
		t.Fatalf("an album with only a year: %+v", r)
	}
}
