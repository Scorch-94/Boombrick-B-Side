package helper

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	librespot "github.com/devgianlu/go-librespot"
	collectionpb "github.com/devgianlu/go-librespot/proto/spotify/collection/v2"
	connectpb "github.com/devgianlu/go-librespot/proto/spotify/connectstate"
	extmetadatapb "github.com/devgianlu/go-librespot/proto/spotify/extendedmetadata"
	metadatapb "github.com/devgianlu/go-librespot/proto/spotify/metadata"
	resumptionpb "github.com/devgianlu/go-librespot/proto/spotify/resumption/v1"
	"github.com/devgianlu/go-librespot/session"
	"github.com/devgianlu/go-librespot/spclient"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"

	"github.com/ujjwal313/boombrick/internal/spotify"
)

const (
	// maxEpisodes caps a show's episodes listed: the newest are what's wanted.
	maxEpisodes = 200
	// maxContinue caps the episodes Continue listening lists.
	maxContinue = 50
	// statesPage is how many resume points are asked for at once, and
	// statesPages how many pages at most.
	statesPage, statesPages = 100, 5
	// collectionPage is how many of a collection's items come at once.
	collectionPage = 300
)

// The collection sets the user's podcasts are kept in.
const (
	showsSet        = "show"
	yourEpisodesSet = "listenlater"
)

// Shows lists the podcasts the user follows, the latest followed first.
func (s *Librespot) Shows(ctx context.Context) ([]spotify.Show, error) {
	sess, err := s.ready(ctx)
	if err != nil {
		return nil, err
	}
	uris, err := collection(ctx, sess, showsSet)
	if err != nil {
		return nil, classify(err)
	}
	meta, err := metadata[metadatapb.Show](ctx, sess.Spclient(), uris, extmetadatapb.ExtensionKind_SHOW_V4)
	if err != nil {
		return nil, classify(err)
	}
	out := make([]spotify.Show, 0, len(uris))
	for _, u := range uris {
		if m := meta[u]; m != nil {
			out = append(out, spotify.Show{URI: u, Name: m.GetName(), Publisher: m.GetPublisher(),
				Cover: cover(m.GetCoverImage().GetImage())})
		}
	}
	return out, nil
}

// Episodes lists a show's episodes, newest first, Continue listening's, the
// latest played first, or Your Episodes', the latest saved first; each with
// the user's place in it.
func (s *Librespot) Episodes(ctx context.Context, uri string) ([]spotify.Episode, error) {
	sess, err := s.ready(ctx)
	if err != nil {
		return nil, err
	}
	states, err := resumeStates(ctx, sess)
	if err != nil {
		// Without the places the episodes still list, from the start.
		s.log.Warn("resume points", "err", err)
	}
	var uris []string
	switch uri {
	case spotify.ContinueURI:
		uris = inProgress(states, maxContinue)
	case spotify.YourEpisodesURI:
		uris, err = collection(ctx, sess, yourEpisodesSet)
	default:
		if !strings.HasPrefix(uri, "spotify:show:") {
			return nil, fmt.Errorf("%w: not a show: %s", spotify.ErrUnavailable, uri)
		}
		uris, err = showEpisodes(ctx, s, sess, uri)
	}
	if err != nil {
		return nil, classify(err)
	}
	meta, err := metadata[metadatapb.Episode](ctx, sess.Spclient(), uris, extmetadatapb.ExtensionKind_EPISODE_V4)
	if err != nil {
		return nil, classify(err)
	}
	out := make([]spotify.Episode, 0, len(uris))
	for _, u := range uris {
		m := meta[u]
		if m == nil {
			continue
		}
		e := episode(strings.TrimPrefix(u, "spotify:episode:"), m)
		if st, ok := states[u]; ok {
			e.Position, e.Finished = st.position, st.finished
		}
		out = append(out, e)
	}
	if uri != spotify.ContinueURI && uri != spotify.YourEpisodesURI {
		// A show's context lists its episodes in the show's order; newest
		// first is how they're browsed.
		slices.SortStableFunc(out, func(a, b spotify.Episode) int { return int(b.Published - a.Published) })
	}
	return out, nil
}

// SavePosition records how far into episode id the user got, or that they
// played it through.
func (s *Librespot) SavePosition(ctx context.Context, id string, ms int64, finished bool) error {
	sess, err := s.ready(ctx)
	if err != nil {
		return err
	}
	sid, err := librespot.SpotifyIdFromBase62(librespot.SpotifyIdTypeEpisode, id)
	if err != nil {
		return fmt.Errorf("%w: %w", spotify.ErrUnavailable, err)
	}
	if finished {
		err = sess.Spclient().SetResumeFinished(ctx, *sid)
	} else {
		err = sess.Spclient().SetResumePositionMs(ctx, *sid, max(ms, 0))
	}
	return classify(err)
}

// episode is an episode's metadata as the helper lists it.
func episode(id string, m *metadatapb.Episode) spotify.Episode {
	e := spotify.Episode{ID: id, Title: m.GetName(), Show: m.GetShow().GetName(), Publisher: m.GetShow().GetPublisher(),
		Duration: int64(m.GetDuration())}
	if g := m.GetShow().GetGid(); len(g) == 16 {
		e.ShowURI = librespot.SpotifyIdFromGid(librespot.SpotifyIdTypeShow, g).Uri()
	}
	e.Cover = cover(m.GetCoverImage().GetImage())
	if e.Cover == "" {
		e.Cover = cover(m.GetShow().GetCoverImage().GetImage())
	}
	if d := m.GetPublishTime(); d != nil && d.GetYear() > 0 {
		e.Published = time.Date(int(d.GetYear()), time.Month(max(d.GetMonth(), 1)), int(max(d.GetDay(), 1)),
			int(d.GetHour()), int(d.GetMinute()), 0, 0, time.UTC).Unix()
	}
	e.External = len(m.GetAudio()) == 0 && m.GetExternalUrl() != ""
	return e
}

// state is where the user is in an episode, by its resume points.
type state struct {
	position int64 // ms
	finished bool
	at       time.Time // when it was last updated
}

// resumeStates is the user's resume points, by URI: the latest revision of
// each, as Spotify's own clients read them.
func resumeStates(ctx context.Context, sess *session.Session) (map[string]state, error) {
	out := map[string]state{}
	token := ""
	for range statesPages {
		resp, err := sess.Spclient().ListCurrentStates(ctx, &resumptionpb.ListCurrentStatesRequest{
			PageSize: statesPage, PageToken: token})
		if err != nil {
			return out, err
		}
		for _, cs := range resp.GetCurrentStates() {
			if st, ok := latestState(cs); ok {
				out[cs.GetUri()] = st
			}
		}
		token = resp.GetNextPageToken()
		if token == "" {
			break
		}
	}
	return out, nil
}

// latestState reads a current state's most recently updated revision.
func latestState(cs *resumptionpb.CurrentState) (state, bool) {
	var latest *resumptionpb.ResumePointRevision
	for _, rev := range cs.GetResumePointRevisions() {
		if latest == nil || rev.GetUpdateTime().AsTime().After(latest.GetUpdateTime().AsTime()) {
			latest = rev
		}
	}
	if latest == nil {
		return state{}, false
	}
	st := state{at: latest.GetUpdateTime().AsTime()}
	if st.at.IsZero() || st.at.Unix() == 0 {
		st.at = latest.GetCreateTime().AsTime()
	}
	switch p := latest.GetSnapshot().GetResumePoint().(type) {
	case *resumptionpb.ResumePoint_Position:
		st.position = p.Position.AsDuration().Milliseconds()
	case *resumptionpb.ResumePoint_RelativePosition:
		st.position = p.RelativePosition.GetStartOffset().AsDuration().Milliseconds()
	case *resumptionpb.ResumePoint_Finished, *resumptionpb.ResumePoint_MarkedAsFinished:
		st.finished = true
	}
	return st, true
}

// inProgress is the episodes part way through, the latest played first, up
// to n.
func inProgress(states map[string]state, n int) []string {
	var uris []string
	for u, st := range states {
		if strings.HasPrefix(u, "spotify:episode:") && !st.finished && st.position > 0 {
			uris = append(uris, u)
		}
	}
	slices.SortFunc(uris, func(a, b string) int {
		if c := states[b].at.Compare(states[a].at); c != 0 {
			return c
		}
		return strings.Compare(a, b)
	})
	if len(uris) > n {
		uris = uris[:n]
	}
	return uris
}

// showEpisodes is a show's episode URIs, from its context, up to maxEpisodes.
func showEpisodes(ctx context.Context, s *Librespot, sess *session.Session, uri string) ([]string, error) {
	r, err := spclient.NewContextResolver(ctx, s.lg, sess.Spclient(), &connectpb.Context{Uri: uri})
	if err != nil {
		return nil, err
	}
	var uris []string
	for page := 0; page < maxPages && len(uris) < maxEpisodes; page++ {
		items, err := r.Page(ctx, page)
		if errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, err
		}
		for _, it := range items {
			u := it.GetUri()
			if u == "" && len(it.GetGid()) == 16 {
				u = librespot.SpotifyIdFromGid(librespot.SpotifyIdTypeEpisode, it.GetGid()).Uri()
			}
			if strings.HasPrefix(u, "spotify:episode:") {
				uris = append(uris, u)
			}
		}
	}
	if len(uris) > maxEpisodes {
		uris = uris[:maxEpisodes]
	}
	return uris, nil
}

// collection is the URIs in one of the user's collection sets, the latest
// added first. Spotify's clients page through it with PageRequest, which
// go-librespot has no message for, so it is written out here: username 1,
// set 2, pagination_token 3 and limit 4; the PageResponse has the items 1
// and next_page_token 2.
func collection(ctx context.Context, sess *session.Session, set string) ([]string, error) {
	type item struct {
		uri string
		at  int64
	}
	var items []item
	token := ""
	for range maxPages {
		var req []byte
		req = protowire.AppendTag(req, 1, protowire.BytesType)
		req = protowire.AppendString(req, sess.Username())
		req = protowire.AppendTag(req, 2, protowire.BytesType)
		req = protowire.AppendString(req, set)
		if token != "" {
			req = protowire.AppendTag(req, 3, protowire.BytesType)
			req = protowire.AppendString(req, token)
		}
		req = protowire.AppendTag(req, 4, protowire.VarintType)
		req = protowire.AppendVarint(req, collectionPage)
		h := http.Header{}
		h.Set("Content-Type", "application/vnd.collection-v2.spotify.proto")
		h.Set("Accept", "application/vnd.collection-v2.spotify.proto")
		resp, err := sess.Spclient().Request(ctx, http.MethodPost, "/collection/v2/paging", nil, h, req)
		if err != nil {
			return nil, err
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("collection %s: %s", set, resp.Status)
		}
		page, next, err := parsePage(body)
		if err != nil {
			return nil, fmt.Errorf("collection %s: %w", set, err)
		}
		for _, it := range page {
			if !it.GetIsRemoved() && it.GetUri() != "" {
				items = append(items, item{it.GetUri(), it.GetAddedAt()})
			}
		}
		token = next
		if token == "" || len(page) == 0 {
			break
		}
	}
	slices.SortStableFunc(items, func(a, b item) int { return int(b.at - a.at) })
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.uri
	}
	return slices.Compact(out), nil
}

// parsePage reads a PageResponse: its items and its next page's token.
func parsePage(b []byte) ([]*collectionpb.CollectionItem, string, error) {
	var items []*collectionpb.CollectionItem
	var next string
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return nil, "", protowire.ParseError(n)
		}
		b = b[n:]
		if typ != protowire.BytesType {
			n = protowire.ConsumeFieldValue(num, typ, b)
			if n < 0 {
				return nil, "", protowire.ParseError(n)
			}
			b = b[n:]
			continue
		}
		v, n := protowire.ConsumeBytes(b)
		if n < 0 {
			return nil, "", protowire.ParseError(n)
		}
		b = b[n:]
		switch num {
		case 1:
			it := &collectionpb.CollectionItem{}
			if err := proto.Unmarshal(v, it); err != nil {
				return nil, "", err
			}
			items = append(items, it)
		case 2:
			next = string(bytes.Clone(v))
		}
	}
	return items, next, nil
}

// metadata fetches the extension kind of each URI, in batches, by URI.
// Those Spotify has none for are left out.
func metadata[T any, PT interface {
	*T
	proto.Message
}](ctx context.Context, sp *spclient.Spclient, uris []string, kind extmetadatapb.ExtensionKind) (map[string]*T, error) {
	out := make(map[string]*T, len(uris))
	for batch := range slices.Chunk(uris, metaBatch) {
		req := &extmetadatapb.BatchedEntityRequest{}
		for _, u := range batch {
			req.EntityRequest = append(req.EntityRequest, &extmetadatapb.EntityRequest{
				EntityUri: u, Query: []*extmetadatapb.ExtensionQuery{{ExtensionKind: kind}}})
		}
		resp, err := sp.ExtendedMetadata(ctx, req)
		if err != nil {
			return nil, err
		}
		for _, item := range resp.GetExtendedMetadata() {
			for _, d := range item.GetExtensionData() {
				if d.GetHeader().GetStatusCode() != 200 || d.GetExtensionData() == nil {
					continue
				}
				m := PT(new(T))
				if d.GetExtensionData().UnmarshalTo(m) == nil {
					out[d.GetEntityUri()] = (*T)(m)
				}
			}
		}
	}
	return out, nil
}
