package main

import (
	"log/slog"
	"time"
)

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
