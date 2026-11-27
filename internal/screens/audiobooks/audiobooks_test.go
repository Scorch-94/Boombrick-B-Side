package audiobooks

import (
	"testing"
	"time"

	"github.com/ujjwal313/boombrick/internal/audiobooks"
)

func TestOrder(t *testing.T) {
	at := time.Unix(1700000000, 0)
	books := []audiobooks.Book{{Path: "/a"}, {Path: "/b"}, {Path: "/c"}, {Path: "/d"}}
	places := map[string]audiobooks.Place{
		"/a": {Part: "/a", Finished: true, Updated: at.Add(time.Hour)},
		"/c": {Part: "/c", Updated: at},
		"/d": {Part: "/d", Updated: at.Add(time.Minute)},
	}
	got := order(books, places)
	want := []string{"/d", "/c", "/b", "/a"}
	for i, b := range got {
		if b.Path != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
}

func TestSubtitle(t *testing.T) {
	b := audiobooks.Book{Path: "/b", Author: "Frank Herbert", Duration: 21*time.Hour + 2*time.Minute,
		Parts: []audiobooks.Part{{Path: "/b", Duration: 21*time.Hour + 2*time.Minute}}}
	cases := []struct {
		pl   audiobooks.Place
		want string
	}{
		{audiobooks.Place{}, "Frank Herbert · 21 hr 2 min"},
		{audiobooks.Place{Part: "/b", Position: 21 * time.Hour}, "Frank Herbert · 2 min left"},
		{audiobooks.Place{Part: "/b", Finished: true}, "Frank Herbert · Finished"},
	}
	for _, c := range cases {
		if got := subtitle(b, c.pl); got != c.want {
			t.Errorf("subtitle(%+v) = %q, want %q", c.pl, got, c.want)
		}
	}
	if got := progress(b, audiobooks.Place{Part: "/b", Position: b.Duration / 2}); got != "50%" {
		t.Errorf("progress = %q", got)
	}
	if hours(3*time.Hour) != "3 hr" || hours(10*time.Second) != "1 min" {
		t.Error(hours(3*time.Hour), hours(10*time.Second))
	}
}
