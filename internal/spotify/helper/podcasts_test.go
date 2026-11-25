package helper

import (
	"context"
	"net/http"
	"net/url"
	"reflect"
	"slices"
	"testing"
	"time"

	collectionpb "github.com/devgianlu/go-librespot/proto/spotify/collection/v2"
	metadatapb "github.com/devgianlu/go-librespot/proto/spotify/metadata"
	resumptionpb "github.com/devgianlu/go-librespot/proto/spotify/resumption/v1"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/ujjwal313/boombrick/internal/spotify"
)

// The podcast routes answer from the session, and the client reads them
// back the same.
func TestPodcastRoutes(t *testing.T) {
	ep := spotify.Episode{ID: "4rOoJ6Egrf8K2IrywzwOMk", Title: "Ep 12", Show: "Night Owls", ShowURI: "spotify:show:1",
		Duration: 3_600_000, Published: 1_700_000_000, Position: 120_000}
	f := &fake{
		shows:    []spotify.Show{{URI: "spotify:show:1", Name: "Night Owls", Publisher: "Owl FM"}},
		episodes: map[string][]spotify.Episode{spotify.ContinueURI: {ep}, "spotify:show:1": {ep}},
	}
	srv := serve(t, f)
	var ss []spotify.Show
	getJSON(t, srv.URL+"/shows", &ss)
	if !reflect.DeepEqual(ss, f.shows) {
		t.Fatalf("shows %+v", ss)
	}
	for _, uri := range []string{spotify.ContinueURI, "spotify:show:1"} {
		var es []spotify.Episode
		getJSON(t, srv.URL+"/episodes?uri="+url.QueryEscape(uri), &es)
		if !reflect.DeepEqual(es, []spotify.Episode{ep}) {
			t.Fatalf("%s: episodes %+v", uri, es)
		}
	}
	resp, err := http.Post(srv.URL+"/position?id=abc&ms=61000&finished=0", "", nil)
	if err != nil || resp.StatusCode != http.StatusNoContent {
		t.Fatalf("position: %v %v", resp, err)
	}
	resp, _ = http.Post(srv.URL+"/position?id=abc&ms=0&finished=1", "", nil)
	if resp.StatusCode != http.StatusNoContent || !slices.Equal(f.saved, []string{"abc 61000 false", "abc 0 true"}) {
		t.Fatalf("saved %v", f.saved)
	}
	f.err = spotify.ErrExternal
	resp, _ = http.Get(srv.URL + "/stream/episode:abc")
	if got := spotify.ErrorOf(resp.StatusCode, "", ""); got != spotify.ErrExternal {
		t.Fatalf("an external episode's stream answers %v", got)
	}
	if f.asked.id != "episode:abc" {
		t.Fatalf("stream asked for %q", f.asked.id)
	}
}

// A collection page is read as Spotify's clients write it.
func TestParsePage(t *testing.T) {
	var b []byte
	for _, it := range []*collectionpb.CollectionItem{
		{Uri: "spotify:show:a", AddedAt: 10}, {Uri: "spotify:show:b", AddedAt: 20, IsRemoved: true}} {
		raw, _ := proto.Marshal(it)
		b = protowire.AppendTag(b, 1, protowire.BytesType)
		b = protowire.AppendBytes(b, raw)
	}
	b = protowire.AppendTag(b, 2, protowire.BytesType)
	b = protowire.AppendString(b, "next-token")
	b = protowire.AppendTag(b, 3, protowire.BytesType) // the sync token, skipped
	b = protowire.AppendString(b, "sync")
	b = protowire.AppendTag(b, 9, protowire.VarintType) // an unknown field, skipped
	b = protowire.AppendVarint(b, 7)
	items, next, err := parsePage(b)
	if err != nil || next != "next-token" || len(items) != 2 || items[0].GetUri() != "spotify:show:a" ||
		!items[1].GetIsRemoved() || items[1].GetAddedAt() != 20 {
		t.Fatalf("items %v, next %q, err %v", items, next, err)
	}
	if _, _, err := parsePage([]byte{0x0a, 0x05, 1}); err == nil {
		t.Fatal("a cut-off page should fail")
	}
}

func rev(at time.Time, p *resumptionpb.ResumePoint) *resumptionpb.ResumePointRevision {
	return &resumptionpb.ResumePointRevision{Snapshot: p, UpdateTime: timestamppb.New(at)}
}

func pos(d time.Duration) *resumptionpb.ResumePoint {
	return &resumptionpb.ResumePoint{ResumePoint: &resumptionpb.ResumePoint_Position{Position: durationpb.New(d)}}
}

// The latest revision of each resume point says where the user is; Continue
// listening is those part way through, the latest played first.
func TestResumeStates(t *testing.T) {
	t0 := time.Date(2026, 11, 20, 8, 0, 0, 0, time.UTC)
	finished := &resumptionpb.ResumePoint{ResumePoint: &resumptionpb.ResumePoint_Finished{Finished: &emptypb.Empty{}}}
	states := map[string]state{}
	for _, cs := range []*resumptionpb.CurrentState{
		{Uri: "spotify:episode:old", ResumePointRevisions: []*resumptionpb.ResumePointRevision{rev(t0, pos(5*time.Minute))}},
		{Uri: "spotify:episode:new", ResumePointRevisions: []*resumptionpb.ResumePointRevision{
			rev(t0.Add(time.Hour), pos(10*time.Minute)), rev(t0.Add(2*time.Hour), pos(12*time.Minute))}},
		{Uri: "spotify:episode:done", ResumePointRevisions: []*resumptionpb.ResumePointRevision{
			rev(t0.Add(3*time.Hour), pos(time.Minute)), rev(t0.Add(4*time.Hour), finished)}},
		{Uri: "spotify:episode:fresh", ResumePointRevisions: []*resumptionpb.ResumePointRevision{rev(t0.Add(5*time.Hour), pos(0))}},
		{Uri: "spotify:chapter:book", ResumePointRevisions: []*resumptionpb.ResumePointRevision{rev(t0.Add(6*time.Hour), pos(time.Minute))}},
		{Uri: "spotify:episode:empty"},
	} {
		if st, ok := latestState(cs); ok {
			states[cs.GetUri()] = st
		}
	}
	if st := states["spotify:episode:new"]; st.position != 720_000 || st.finished {
		t.Fatalf("new: %+v, want the later revision's 12 minutes", st)
	}
	if !states["spotify:episode:done"].finished {
		t.Fatal("done should be finished, its latest revision says so")
	}
	if _, ok := states["spotify:episode:empty"]; ok {
		t.Fatal("a state with no revisions says nothing")
	}
	if got := inProgress(states, 10); !slices.Equal(got, []string{"spotify:episode:new", "spotify:episode:old"}) {
		t.Fatalf("in progress %v", got)
	}
	if got := inProgress(states, 1); len(got) != 1 {
		t.Fatalf("capped at 1: %v", got)
	}
}

// An episode's metadata becomes the listed episode: its show, date, cover
// and whether Spotify hosts it.
func TestEpisodeMetadata(t *testing.T) {
	str := func(s string) *string { return &s }
	i32 := func(n int32) *int32 { return &n }
	gid := make([]byte, 16)
	gid[15] = 1
	m := &metadatapb.Episode{Name: str("Ep 12"), Duration: i32(3_600_000),
		PublishTime: &metadatapb.Date{Year: i32(2026), Month: i32(11), Day: i32(20)},
		Show: &metadatapb.Show{Gid: gid, Name: str("Night Owls"), Publisher: str("Owl FM"),
			CoverImage: &metadatapb.ImageGroup{Image: []*metadatapb.Image{{FileId: []byte{0xab}}}}},
		Audio: []*metadatapb.AudioFile{{FileId: []byte{1}}}}
	e := episode("abc", m)
	if e.Title != "Ep 12" || e.Show != "Night Owls" || e.Publisher != "Owl FM" || e.Duration != 3_600_000 ||
		e.Published != time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC).Unix() || e.Cover != imageURL+"ab" ||
		e.External || e.ShowURI == "" {
		t.Fatalf("episode %+v", e)
	}
	m.Audio, m.ExternalUrl = nil, str("https://example.com/ep12.mp3")
	if !episode("abc", m).External {
		t.Fatal("an episode with only a link is external")
	}
}

// SavePosition and Episodes ask for a session first, like every call.
func TestPodcastsWaitForTheSession(t *testing.T) {
	s := &Librespot{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.Episodes(ctx, spotify.ContinueURI); err == nil {
		t.Fatal("no session, no episodes")
	}
}
