package helper

import "testing"

// The Web API's answer: songs it can't play are left out, singles of more
// than three songs are EPs, and the cover is the smallest at least 300 wide.
func TestParseSearch(t *testing.T) {
	body := []byte(`{
	  "tracks": {"items": [
	    {"id": "t1", "name": "Carol of the Bells", "duration_ms": 201000, "is_playable": true,
	     "artists": [{"name": "Lindsey Stirling"}, {"name": "Egzod"}],
	     "album": {"name": "Holiday Favorites", "artists": [{"name": "Lindsey Stirling"}],
	       "images": [{"url": "big", "width": 640}, {"url": "mid", "width": 300}, {"url": "small", "width": 64}]},
	     "external_ids": {"isrc": "USX1"}},
	    {"id": "t2", "name": "Blocked", "is_playable": false, "artists": [], "album": {}},
	    null
	  ]},
	  "albums": {"items": [
	    {"uri": "spotify:album:a1", "name": "Holiday Favorites", "album_type": "album", "total_tracks": 24,
	     "artists": [{"name": "Lindsey Stirling"}], "images": [{"url": "a", "width": 640}]},
	    {"uri": "spotify:album:a2", "name": "Four", "album_type": "single", "total_tracks": 4, "artists": []}
	  ]},
	  "artists": {"items": [{"uri": "spotify:artist:r1", "name": "Lindsey Stirling", "images": [{"url": "r", "width": 160}]}]}
	}`)
	r, err := parseSearch(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Tracks) != 1 || r.Tracks[0].Cover != "mid" || r.Tracks[0].AlbumArtist != "Lindsey Stirling" ||
		len(r.Tracks[0].Artists) != 2 || r.Tracks[0].ISRC != "USX1" || r.Tracks[0].Duration != 201000 {
		t.Fatalf("tracks %+v", r.Tracks)
	}
	if len(r.Albums) != 2 || r.Albums[0].Type != "Album" || r.Albums[1].Type != "EP" || r.Albums[0].Tracks != 24 {
		t.Fatalf("albums %+v", r.Albums)
	}
	if len(r.Artists) != 1 || r.Artists[0].Cover != "r" {
		t.Fatalf("artists %+v", r.Artists)
	}
	if _, err := parseSearch([]byte("not json")); err == nil {
		t.Fatal("no error for a broken answer")
	}
}
