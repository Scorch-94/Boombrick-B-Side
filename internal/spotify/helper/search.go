package helper

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/ujjwal313/boombrick/internal/spotify"
)

// searchURL is Spotify's Web API search, which the session's own token can
// ask. searchLimit is how many of each kind come back.
var searchURL = "https://api.spotify.com/v1/search"

const searchLimit = 15

// Search finds songs, albums and artists on Spotify matching q.
func (s *Librespot) Search(ctx context.Context, q string) (spotify.Results, error) {
	sess, err := s.ready(ctx)
	if err != nil {
		return spotify.Results{}, err
	}
	sp := sess.Spclient()
	ask := func(force bool) (*http.Response, error) {
		token, err := sp.GetAccessToken(ctx, force)
		if err != nil {
			return nil, err
		}
		u := searchURL + "?" + url.Values{"q": {q}, "type": {"track,album,artist"},
			"limit": {fmt.Sprint(searchLimit)}}.Encode()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		return http.DefaultClient.Do(req)
	}
	resp, err := ask(false)
	if err == nil && resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close()
		resp, err = ask(true) // a token gone stale: once more with a new one
	}
	if err != nil {
		return spotify.Results{}, classify(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return spotify.Results{}, classify(err)
	}
	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		return spotify.Results{}, fmt.Errorf("%w: Spotify is limiting searches; try again in a while", spotify.ErrUnavailable)
	case resp.StatusCode != http.StatusOK:
		return spotify.Results{}, fmt.Errorf("%w: search answered %d: %.200s", spotify.ErrUnavailable, resp.StatusCode, body)
	}
	return parseSearch(body)
}

// The Web API's search answer, as much of it as is used.
type webImage struct {
	URL   string `json:"url"`
	Width int    `json:"width"`
}

type webArtist struct {
	URI    string     `json:"uri"`
	Name   string     `json:"name"`
	Images []webImage `json:"images"`
}

type webAlbum struct {
	URI         string      `json:"uri"`
	Name        string      `json:"name"`
	AlbumType   string      `json:"album_type"`
	TotalTracks int         `json:"total_tracks"`
	ReleaseDate string      `json:"release_date"`
	Images      []webImage  `json:"images"`
	Artists     []webArtist `json:"artists"`
}

type webTrack struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	DurationMs  int64       `json:"duration_ms"`
	Artists     []webArtist `json:"artists"`
	Album       webAlbum    `json:"album"`
	IsPlayable  *bool       `json:"is_playable"`
	ExternalIDs struct {
		ISRC string `json:"isrc"`
	} `json:"external_ids"`
}

type webSearch struct {
	Tracks  struct{ Items []*webTrack }  `json:"tracks"`
	Albums  struct{ Items []*webAlbum }  `json:"albums"`
	Artists struct{ Items []*webArtist } `json:"artists"`
}

// parseSearch reads the Web API's answer.
func parseSearch(body []byte) (spotify.Results, error) {
	var w webSearch
	if err := json.Unmarshal(body, &w); err != nil {
		return spotify.Results{}, fmt.Errorf("%w: search: %v", spotify.ErrUnavailable, err)
	}
	var r spotify.Results
	for _, t := range w.Tracks.Items {
		if t == nil || t.ID == "" || t.IsPlayable != nil && !*t.IsPlayable {
			continue
		}
		r.Tracks = append(r.Tracks, spotify.Track{ID: t.ID, Title: t.Name, Artists: names(t.Artists), Album: t.Album.Name,
			AlbumArtist: strings.Join(names(t.Album.Artists), ", "), Duration: t.DurationMs,
			Cover: image(t.Album.Images), ISRC: t.ExternalIDs.ISRC})
	}
	for _, a := range w.Albums.Items {
		if a == nil || a.URI == "" {
			continue
		}
		typ := "Album"
		switch {
		case a.AlbumType == "single" && a.TotalTracks > 3:
			typ = "EP"
		case a.AlbumType == "single":
			typ = "Single"
		case a.AlbumType == "compilation":
			typ = "Compilation"
		}
		r.Albums = append(r.Albums, spotify.Release{URI: a.URI, Name: a.Name, Artist: strings.Join(names(a.Artists), ", "),
			Cover: image(a.Images), Type: typ, Tracks: a.TotalTracks})
	}
	for _, a := range w.Artists.Items {
		if a == nil || a.URI == "" {
			continue
		}
		r.Artists = append(r.Artists, spotify.Artist{URI: a.URI, Name: a.Name, Cover: image(a.Images)})
	}
	return r, nil
}

func names(as []webArtist) []string {
	out := make([]string, 0, len(as))
	for _, a := range as {
		out = append(out, a.Name)
	}
	return out
}

// image is the smallest of images at least 300 wide, or the largest.
func image(ims []webImage) string {
	best := -1
	for i, im := range ims {
		switch {
		case best < 0:
			best = i
		case im.Width >= 300 && (ims[best].Width < 300 || im.Width < ims[best].Width):
			best = i
		case ims[best].Width < 300 && im.Width > ims[best].Width:
			best = i
		}
	}
	if best < 0 {
		return ""
	}
	return ims[best].URL
}
