package main

import (
	"io"
	"log/slog"
	"testing"
	"time"
)

func TestWaitAtMost(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	done := make(chan struct{})
	close(done)
	start := time.Now()
	waitAtMost(done, time.Minute, "done", log)
	waitAtMost(make(chan struct{}), 50*time.Millisecond, "stuck", log)
	if d := time.Since(start); d > time.Second {
		t.Fatalf("took %v", d)
	}
}
