package runner

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/aalejandrofer/grubdrops/internal/platform"
)

type watchPulse struct {
	cancel context.CancelFunc
	done   chan struct{}
}

// Playback has its own cadence: a long inventory read or campaign scan must
// not leave gaps in the stream's playlist coverage. The minute beacon remains
// controlled exclusively by DefaultTick.
func (r *Runner) startWatchPulse(parent context.Context, handle *platform.WatchHandle) {
	pulser, ok := r.cfg.Backend.(platform.WatchPulser)
	if !ok {
		return
	}
	interval := pulser.WatchPulseInterval()
	if interval <= 0 {
		return
	}
	r.mu.Lock()
	if r.handle != handle || r.playbackPulse != nil {
		r.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(parent)
	p := &watchPulse{cancel: cancel, done: make(chan struct{})}
	r.playbackPulse = p
	h := *handle
	r.mu.Unlock()
	go func() {
		defer close(p.done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		var failures onceLog
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
			if ctx.Err() != nil {
				return
			}
			started := time.Now()
			err := safeWatchPulse(ctx, pulser, h)
			if ctx.Err() != nil {
				return
			}
			if err != nil {
				if failures.Changed(err.Error()) {
					slog.Warn("runner playback pulse failed", "kind", "error", "account", r.cfg.AccountID, "channel", h.Channel, "err", err)
				}
				continue
			}
			if failures.Reset() > 0 {
				slog.Info("runner playback pulse recovered", "kind", "state", "account", r.cfg.AccountID, "channel", h.Channel)
			}
			slog.Debug("runner playback pulse sent", "kind", "heartbeat", "account", r.cfg.AccountID, "channel", h.Channel, "elapsed", time.Since(started))
		}
	}()
}

func safeWatchPulse(ctx context.Context, p platform.WatchPulser, h platform.WatchHandle) (err error) {
	defer func() {
		if recover() != nil {
			err = errors.New("playback pulse panicked")
		}
	}()
	return p.WatchPulse(ctx, h)
}
