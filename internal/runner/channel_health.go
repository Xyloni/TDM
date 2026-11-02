package runner

import (
	"context"
	"hash/fnv"
	"log/slog"
	"strings"
	"time"

	"github.com/aalejandrofer/grubdrops/internal/plan"
	"github.com/aalejandrofer/grubdrops/internal/platform"
)

// A directory page is a limited, changing selection, not proof of liveness.
// Return true only when a switch is justified. Lookup failures preserve playback.
func (r *Runner) checkWatchChannel(ctx context.Context, handle platform.WatchHandle, target plan.Target) bool {
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	streams, err := r.cfg.Backend.ListEligibleChannels(ctx, r.cfg.Session, target.Campaign)
	if err != nil {
		slog.Warn("runner channel check inconclusive; keeping watch", "account", r.cfg.AccountID, "channel", handle.Channel, "reason", "list_error", "err", err)
		return false
	}
	for _, s := range streams {
		if strings.EqualFold(s.Channel, handle.Channel) {
			return false
		}
	}
	reason := "missing_from_eligible_list"
	if looker, ok := r.cfg.Backend.(platform.ChannelLooker); ok && r.cfg.Platform == "twitch" {
		stream, live, err := looker.LookupChannel(ctx, r.cfg.Session, handle.Channel)
		if err != nil {
			slog.Warn("runner channel check inconclusive; keeping watch", "account", r.cfg.AccountID, "channel", handle.Channel, "reason", "lookup_error", "err", err)
			return false
		}
		switch {
		case !live:
			reason = "confirmed_offline"
		case stream.GameID != "" && target.Campaign.GameID != "" && stream.GameID != target.Campaign.GameID:
			reason = "category_changed"
		case stream.Game != "" && target.Campaign.Game != "" && !strings.EqualFold(strings.TrimSpace(stream.Game), strings.TrimSpace(target.Campaign.Game)):
			reason = "category_changed"
		default:
			// A restricted campaign's incomplete allow-list must not be bypassed.
			allowed := target.Campaign.AllowedChannelCount == 0
			for _, login := range target.Campaign.AllowedChannels {
				if strings.EqualFold(login, handle.Channel) {
					allowed = true
				}
			}
			if !allowed {
				reason = "campaign_channel_not_confirmed"
				break
			}
			if stream.Game == "" && stream.GameID == "" {
				slog.Info("runner channel check inconclusive; keeping watch", "account", r.cfg.AccountID, "channel", handle.Channel, "reason", "category_unknown")
				return false
			}
			if checker, ok := r.cfg.Backend.(platform.AvailableDropsChecker); ok && stream.ChannelID != "" {
				drops, err := checker.AvailableDropIDs(ctx, r.cfg.Session, stream.ChannelID)
				if err != nil {
					slog.Warn("runner channel check inconclusive; keeping watch", "account", r.cfg.AccountID, "channel", handle.Channel, "reason", "drop_check_error", "err", err)
					return false
				}
				if len(drops) > 0 {
					if _, ok := drops[target.Benefit.ID]; !ok {
						reason = "reward_not_available"
						break
					}
				}
			}
			r.mu.Lock()
			changedBroadcast := r.stream != nil && r.stream.BroadcastID != "" && stream.BroadcastID != "" && r.stream.BroadcastID != stream.BroadcastID
			r.mu.Unlock()
			if changedBroadcast {
				reason = "broadcast_changed"
				break
			}
			slog.Info("runner channel absent from directory but still eligible; keeping watch", "account", r.cfg.AccountID, "channel", handle.Channel, "reason", "directory_omission")
			return false
		}
	}
	slog.Info("runner channel no longer eligible; swapping", "kind", "state", "account", r.cfg.AccountID, "channel", handle.Channel, "reason", reason, "alternatives", len(streams))
	return true
}

// Caller holds mu. Deterministic jitter spreads accounts without flaky clocks
// or random state. Snapshot reads never advance the retry sequence.
func (r *Runner) idleWaitLocked(state State, pace time.Duration) time.Duration {
	if state != StateSleeping || !r.sleepNoStream {
		return idleWait(state, r.sleepNoStream, pace)
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(r.cfg.AccountID))
	base := 30 * time.Second * time.Duration(1<<min(r.noStreamAttempts, 4))
	base = min(base, 270*time.Second)
	jitter := time.Duration((h.Sum32()+uint32(r.noStreamAttempts)*7919)%31) * time.Second
	return min(base+jitter, recheckInterval)
}
