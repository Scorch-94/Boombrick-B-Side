//go:build !linux

package remote

import "context"

// runKeys has no input devices to read off Linux.
func runKeys(ctx context.Context, r *Remote) { <-ctx.Done() }
