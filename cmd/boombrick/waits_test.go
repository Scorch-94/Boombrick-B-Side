package main

import (
	"io"
	"log/slog"
	"strings"
	"sync"
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

// The watchdog logs the stacks and exits once its time is up.
func TestWatchdog(t *testing.T) {
	var b strings.Builder
	log := slog.New(slog.NewTextHandler(&lockedWriter{w: &b}, nil))
	exited := make(chan struct{})
	startWatchdog(20*time.Millisecond, log, func() { close(exited) })
	select {
	case <-exited:
	case <-time.After(5 * time.Second):
		t.Fatal("the watchdog didn't exit")
	}
	if !strings.Contains(b.String(), "TestWatchdog") {
		t.Fatalf("no stacks logged: %.200s", b.String())
	}
}

type lockedWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (l *lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}
