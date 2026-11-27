package main

import (
	"log/slog"
	"runtime"
	"time"
)

// quitLimit is how long quitting may take before the watchdog ends it.
const quitLimit = 10 * time.Second

// startWatchdog waits d on a goroutine of its own, then logs every
// goroutine's stack, to see what quitting was stuck on, and calls exit.
// Exiting normally first ends it with the program.
func startWatchdog(d time.Duration, log *slog.Logger, exit func()) {
	go func() {
		time.Sleep(d)
		buf := make([]byte, 1<<20)
		buf = buf[:runtime.Stack(buf, true)]
		log.Error("quitting took too long; exiting", "after", d, "stacks", string(buf))
		exit()
	}()
}

// waitAtMost waits for done, but no longer than d: quitting, a part stuck in
// the network or a device mustn't keep the program from exiting, which would
// leave the Brick on a frozen screen. It logs when it gives up.
func waitAtMost(done <-chan struct{}, d time.Duration, what string, log *slog.Logger) {
	select {
	case <-done:
	case <-time.After(d):
		log.Warn("gave up waiting to stop", "what", what, "after", d)
	}
}
