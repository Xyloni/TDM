package runner

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/aalejandrofer/grubdrops/internal/plan"
	"github.com/aalejandrofer/grubdrops/internal/platform"
)

const playbackRecoveryCooldown = 2 * time.Minute

type playbackHealth struct {
	first404    time.Time
	failures    int
	ready       bool
	nextAttempt time.Time
	lastChannel string
	lastAttempt time.Time
}

func (h *playbackHealth) resetWatch() { h.first404 = time.Time{}; h.failures = 0; h.ready = false }
func (h *playbackHealth) observe(now time.Time, err error) {
	if !errors.Is(err, platform.ErrPlaybackUnavailable) {
		h.resetWatch()
		return
	}
	if h.first404.IsZero() {
		h.first404 = now
	}
	h.failures++
	h.ready = h.failures >= 3 && now.Sub(h.first404) >= 40*time.Second
}

// Run exclusively on the main runner loop, after inventory and completion
// checks. The pulse goroutine reports health but never stops or starts watches.
func (r *Runner) recoverPlayback(ctx context.Context, handle platform.WatchHandle, target plan.Target, now time.Time) bool {
	if r.cfg.Platform != "twitch" {
		return false
	}
	r.mu.Lock()
	health := r.playbackHealth
	r.mu.Unlock()
	if !health.ready || now.Before(health.nextAttempt) {
		return false
	}
	channel := handle.Channel
	repeated := health.lastChannel == channel && now.Sub(health.lastAttempt) < 10*time.Minute
	action := "reconnect"
	if repeated {
		lookupCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
		streams, err := r.cfg.Backend.ListEligibleChannels(lookupCtx, r.cfg.Session, target.Campaign)
		cancel()
		if err == nil {
			r.mu.Lock()
			streams = r.unstalledLocked(streams, now)
			r.mu.Unlock()
			for _, stream := range streams {
				if stream.Channel != handle.Channel {
					channel = stream.Channel
					action = "switch_channel"
					break
				}
			}
		}
	}
	if ctx.Err() != nil {
		return false
	}
	r.mu.Lock()
	// A successful pulse during the alternatives lookup cancels stale recovery.
	if !r.playbackHealth.ready {
		r.mu.Unlock()
		return false
	}
	r.playbackHealth.nextAttempt = now.Add(playbackRecoveryCooldown)
	r.playbackHealth.lastAttempt = now
	r.playbackHealth.lastChannel = channel
	r.reconnectChannel = channel
	r.reconnectBeatAt = r.lastBeatAt
	if channel != handle.Channel {
		r.stalled[handle.Channel] = now.Add(playbackRecoveryCooldown)
		r.farm.rotations = append(r.farm.rotations, now)
	}
	r.mu.Unlock()
	slog.Info("runner recovering persistent HLS 404", "kind", "state", "account", r.cfg.AccountID, "channel", handle.Channel, "next_channel", channel, "action", action, "consecutive_failures", health.failures, "retry_after", now.Add(playbackRecoveryCooldown))
	r.stopWatch(ctx)
	r.setState(ctx, StateStarting)
	return true
}
