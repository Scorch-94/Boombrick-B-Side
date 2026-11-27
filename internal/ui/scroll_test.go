package ui

import (
	"testing"
	"time"
)

// A focused line too long waits ScrollDelay, slides through, rests, and
// slides again; while it doesn't slide it says when it next will.
func TestScrollOffset(t *testing.T) {
	const tw = 200
	if off, moving, next := scrollOffset(tw, 100*time.Millisecond); off != 0 || moving || next != ScrollDelay {
		t.Fatalf("waiting: %v %v %v", off, moving, next)
	}
	off, moving, _ := scrollOffset(tw, ScrollDelay+time.Second)
	if !moving || off < scrollSpeed-1 || off > scrollSpeed+1 {
		t.Fatalf("a second in: %v %v", off, moving)
	}
	slide := time.Duration(float64(tw+scrollGap) / scrollSpeed * float64(time.Second))
	off, moving, next := scrollOffset(tw, ScrollDelay+slide+time.Second)
	if off != 0 || moving || next != ScrollDelay+slide+scrollRest {
		t.Fatalf("resting: %v %v %v", off, moving, next)
	}
	if _, moving, _ := scrollOffset(tw, ScrollDelay+slide+scrollRest+time.Second); !moving {
		t.Fatal("sliding again after the rest")
	}
}
