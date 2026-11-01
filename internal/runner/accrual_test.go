package runner

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aalejandrofer/grubdrops/internal/platform"
	"github.com/aalejandrofer/grubdrops/internal/platform/platformtest"
	"github.com/stretchr/testify/require"
)

func TestAccrualWindowDetectsSlowButAdvancingProgress(t *testing.T) {
	var w accrualWindow
	base := time.Unix(100000, 0)
	for minute := 0; minute <= 16; minute++ {
		reading, confirmed := w.observe(base.Add(time.Duration(minute)*time.Minute), "drop", 133+minute*4/5)
		require.Equal(t, minute == 16, confirmed, "minute %d: %+v", minute, reading)
	}
}

func TestAccrualWindowHealthyAndDelayedBatchUpdates(t *testing.T) {
	for _, batch := range []int{1, 2, 3} {
		var w accrualWindow
		base := time.Unix(100000, 0)
		for minute := 0; minute < 180; minute++ {
			_, low := w.observe(base.Add(time.Duration(minute)*time.Minute), "drop", 120+minute/batch*batch)
			require.False(t, low, "batch %d minute %d", batch, minute)
		}
		require.LessOrEqual(t, len(w.samples), 17)
	}
}

func TestAccrualWindowResetsConfirmationOnRecovery(t *testing.T) {
	var w accrualWindow
	base := time.Unix(100000, 0)
	for minute := 0; minute <= 15; minute++ {
		_, low := w.observe(base.Add(time.Duration(minute)*time.Minute), "drop", minute*4/5)
		require.False(t, low)
	}
	require.False(t, w.lowAt.IsZero())
	_, low := w.observe(base.Add(16*time.Minute), "drop", 16)
	require.False(t, low)
	require.True(t, w.lowAt.IsZero())
}

func TestAccrualWindowResetsForDifferentBenefitGapAndRegression(t *testing.T) {
	base := time.Unix(100000, 0)
	for _, tc := range []struct {
		name, benefit string
		offset        time.Duration
		minutes       int
	}{
		{"benefit", "next", time.Minute, 114},
		{"gap", "drop", 3 * time.Minute, 114},
		{"regression", "drop", time.Minute, 1},
		{"same timestamp", "drop", 0, 114},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var w accrualWindow
			for m := 0; m <= 16; m++ {
				w.observe(base.Add(time.Duration(m)*time.Minute), "drop", 100+m*4/5)
			}
			_, low := w.observe(base.Add(16*time.Minute+tc.offset), tc.benefit, tc.minutes)
			require.False(t, low)
			require.Len(t, w.samples, 1)
			require.True(t, w.lowAt.IsZero())
		})
	}
}

func TestSlowAccrualRotatesChannelAndHonorsAccountCooldown(t *testing.T) {
	f := platformtest.NewFake()
	r := watching(t, f, func(c *Config) { c.Platform = "twitch" })
	f.Streams["c1"] = []platform.Stream{{Channel: "a", ViewerCount: 10}, {Channel: "b", ViewerCount: 5}}
	base := time.Now()
	h := *r.handle
	for m := 0; m <= 16; m++ {
		rotated, err := r.checkSlowAccrual(context.Background(), h, "d1", m*4/5, base.Add(time.Duration(m)*time.Minute))
		require.NoError(t, err)
		require.Equal(t, m == 16, rotated)
	}
	require.Equal(t, "b", r.Snapshot().Channel)
	require.True(t, hasCall(f, "Stop:a"))
	require.True(t, hasCall(f, "Start:b"))
	require.Empty(t, r.accrual.samples, "new playback needs a fresh window")
	h = *r.handle
	for m := 0; m <= 18; m++ {
		rotated, err := r.checkSlowAccrual(context.Background(), h, "d1", m*4/5, base.Add(time.Duration(17+m)*time.Minute))
		require.NoError(t, err)
		require.False(t, rotated, "cooldown at minute %d", 17+m)
	}
	r.stopWatch(context.Background())
	require.Empty(t, r.accrual.samples)
	require.False(t, r.nextSlowAttempt.IsZero(), "stopping must preserve the account cooldown")
}

func TestSlowAccrualKeepsWatchingWithoutAlternative(t *testing.T) {
	f := platformtest.NewFake()
	r := watching(t, f, func(c *Config) { c.Platform = "twitch" })
	h := *r.handle
	base := time.Now()
	for m := 0; m <= 18; m++ {
		rotated, err := r.checkSlowAccrual(context.Background(), h, "d1", m*4/5, base.Add(time.Duration(m)*time.Minute))
		require.NoError(t, err)
		require.False(t, rotated)
	}
	require.False(t, hasCall(f, "Stop:a"))
	require.Equal(t, "a", r.Snapshot().Channel)
	require.False(t, r.nextSlowAttempt.IsZero(), "no-alternative attempts must be paced too")
}

func TestWatchCompletedRewardWinsOverSlowAccrual(t *testing.T) {
	f := platformtest.NewFake()
	r := watching(t, f, func(c *Config) { c.Platform = "twitch" })
	now := time.Now()
	r.accrual = accrualWindow{benefit: "d1", samples: []accrualSample{{now.Add(-16 * time.Minute), 59}, {now.Add(-time.Minute), 59}}, lowAt: now.Add(-time.Minute)}
	f.Progress["d1"] = &platform.Progress{BenefitID: "d1", MinutesWatched: 60}
	require.NoError(t, r.tick(context.Background()))
	require.Equal(t, StateClaiming, r.State())
	require.False(t, hasCall(f, "Stop:a"))
}

func TestWatchSlowAccrualIsIntegratedWithInventoryPoll(t *testing.T) {
	f := platformtest.NewFake()
	r := watching(t, f, func(c *Config) { c.Platform = "twitch" })
	f.Streams["c1"] = []platform.Stream{{Channel: "a", ViewerCount: 10}, {Channel: "b", ViewerCount: 5}}
	now := time.Now()
	r.accrual = accrualWindow{benefit: "d1", samples: []accrualSample{{now.Add(-15 * time.Minute), 0}, {now.Add(-time.Minute), 12}}, lowAt: now.Add(-time.Minute)}
	f.Progress["d1"] = &platform.Progress{BenefitID: "d1", MinutesWatched: 12}
	require.NoError(t, r.tick(context.Background()))
	require.Equal(t, "b", r.Snapshot().Channel)
}

func TestWatchUnreliableInventoryResetsSlowAccrual(t *testing.T) {
	for _, failed := range []bool{true, false} {
		f := platformtest.NewFake()
		r := watching(t, f, func(c *Config) { c.Platform = "twitch" })
		r.accrual = accrualWindow{benefit: "d1", samples: []accrualSample{{time.Now(), 0}}}
		if failed {
			f.InventoryErr = errors.New("unavailable")
		}
		require.NoError(t, r.tick(context.Background()))
		require.Empty(t, r.accrual.samples)
		require.Equal(t, "a", r.Snapshot().Channel)
	}
}
