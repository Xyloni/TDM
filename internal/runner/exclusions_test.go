package runner

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aalejandrofer/grubdrops/internal/platform"
	"github.com/aalejandrofer/grubdrops/internal/platform/platformtest"
	"github.com/stretchr/testify/require"
)

func TestExcludingCurrentDropSwitchesBeforeNextMinute(t *testing.T) {
	f := platformtest.NewFake()
	f.Campaigns = []platform.Campaign{camp("c1", "Rust", "badge", "reward")}
	f.Streams["c1"] = []platform.Stream{{Channel: "live"}}
	f.Progress["badge"] = &platform.Progress{BenefitID: "badge"}
	f.Progress["reward"] = &platform.Progress{BenefitID: "reward"}
	var excluded atomic.Bool
	r := newRunner(f, func(c *Config) {
		c.ExcludedDrops = func(context.Context) (map[string]bool, error) { return map[string]bool{"badge": excluded.Load()}, nil }
	})
	p := NewPool(nil)
	defer p.Stop()
	require.NoError(t, p.Reload(context.Background(), []Builder{func() Entry { return Entry{ID: "acc", Unit: r} }}))
	require.Eventually(t, func() bool {
		s := r.Snapshot()
		return s.State == StateWatching && s.Current != nil && s.Current.Benefit.ID == "badge"
	}, time.Second, time.Millisecond)
	excluded.Store(true)
	p.DropExclusionsChanged("kick", "badge")
	require.Eventually(t, func() bool {
		s := r.Snapshot()
		return s.State == StateWatching && s.Current != nil && s.Current.Benefit.ID == "reward"
	}, time.Second, time.Millisecond)
	require.GreaterOrEqual(t, f.CallCount("Stop:"), 1)
}
