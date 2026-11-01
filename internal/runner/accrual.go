package runner

import (
	"context"
	"log/slog"
	"time"

	"github.com/aalejandrofer/grubdrops/internal/platform"
)

const (
	slowAccrualWindow   = 15 * time.Minute
	slowAccrualConfirm  = time.Minute
	slowAccrualCooldown = 20 * time.Minute
	// Do not interpret a long gap in inventory observations as under-credit.
	slowAccrualMaxGap = 150 * time.Second
)

type accrualSample struct {
	at      time.Time
	minutes int
}

type accrualReading struct {
	elapsed time.Duration
	gained  int
}

// accrualWindow uses polled platform progress, never local heartbeat counts or
// PubSub pushes. The first successful poll establishes the baseline, so an
// account's pre-existing minutes cannot influence its measured rate.
type accrualWindow struct {
	benefit string
	samples []accrualSample
	lowAt   time.Time
}

func (w *accrualWindow) observe(now time.Time, benefit string, minutes int) (accrualReading, bool) {
	if n := len(w.samples); n > 0 {
		last := w.samples[n-1]
		if benefit != w.benefit || !now.After(last.at) || now.Sub(last.at) > slowAccrualMaxGap || minutes < last.minutes {
			*w = accrualWindow{}
		}
	}
	w.benefit = benefit
	w.samples = append(w.samples, accrualSample{now, minutes})
	cutoff := now.Add(-slowAccrualWindow)
	// Retain the nearest sample at/before the window boundary plus newer ones.
	// Compact in place to keep memory bounded over long-running watches.
	i := 0
	for i+1 < len(w.samples) && !w.samples[i+1].at.After(cutoff) {
		i++
	}
	if i > 0 {
		copy(w.samples, w.samples[i:])
		w.samples = w.samples[:len(w.samples)-i]
	}
	first := w.samples[0]
	reading := accrualReading{now.Sub(first.at), minutes - first.minutes}
	if reading.elapsed < slowAccrualWindow {
		return reading, false
	}
	// Strictly below 90%; comparing durations avoids rounded percentage edges.
	if time.Duration(reading.gained)*time.Minute >= reading.elapsed*9/10 {
		w.lowAt = time.Time{}
		return reading, false
	}
	if w.lowAt.IsZero() {
		w.lowAt = now
		return reading, false
	}
	return reading, now.Sub(w.lowAt) >= slowAccrualConfirm
}

func (r *Runner) checkSlowAccrual(ctx context.Context, handle platform.WatchHandle, benefit string, minutes int, now time.Time) (bool, error) {
	r.mu.Lock()
	if r.handle == nil || r.handle.Channel != handle.Channel || r.current == nil || r.current.Benefit.ID != benefit {
		r.mu.Unlock()
		return false, nil
	}
	reading, low := r.accrual.observe(now, benefit, minutes)
	if !low || now.Before(r.nextSlowAttempt) {
		r.mu.Unlock()
		return false, nil
	}
	r.nextSlowAttempt = now.Add(slowAccrualCooldown)
	retryAfter := r.nextSlowAttempt
	campaign := r.current.Campaign
	r.mu.Unlock()

	// Never abandon a usable stream solely for slowness if no alternative is
	// available. A failed lookup also leaves current playback intact.
	streams, err := r.cfg.Backend.ListEligibleChannels(ctx, r.cfg.Session, campaign)
	if err != nil {
		slog.Warn("runner slow accrual alternatives lookup failed", "account", r.cfg.AccountID, "channel", handle.Channel, "err", err)
		return false, ctx.Err()
	}
	r.mu.Lock()
	streams = r.unstalledLocked(streams, now)
	alternative := false
	for _, s := range streams {
		if s.Channel != "" && s.Channel != handle.Channel {
			alternative = true
			break
		}
	}
	valid := r.handle != nil && r.handle.Channel == handle.Channel && r.current != nil && r.current.Benefit.ID == benefit
	r.mu.Unlock()
	if !valid || ctx.Err() != nil {
		return false, ctx.Err()
	}
	if !alternative {
		slog.Info("runner slow accrual; no alternative channel", "account", r.cfg.AccountID, "benefit", benefit, "channel", handle.Channel, "window_minutes", reading.elapsed.Minutes(), "gained_minutes", reading.gained)
		return false, nil
	}
	r.stopWatch(ctx)
	r.mu.Lock()
	r.stalled[handle.Channel] = now.Add(stalledChannelCooldown)
	r.mu.Unlock()
	r.setState(ctx, StateStarting)
	err = r.start(ctx)
	r.mu.Lock()
	newChannel := r.channel
	r.mu.Unlock()
	slog.Info("runner slow accrual channel rotation", "kind", "state", "account", r.cfg.AccountID, "benefit", benefit,
		"old_channel", handle.Channel, "new_channel", newChannel, "window_minutes", reading.elapsed.Minutes(),
		"gained_minutes", reading.gained, "rate_pct", float64(reading.gained)/reading.elapsed.Minutes()*100,
		"retry_after", retryAfter, "err", err)
	return true, err
}
