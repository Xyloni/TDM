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

type playbackBackend struct {
	*platformtest.Fake
	pulses, active, inventoryCalls atomic.Int32
	blockInventory, blockPulse     bool
	inventoryEntered, pulseEntered chan struct{}
}

func (*playbackBackend) WatchPulseInterval() time.Duration { return 5 * time.Millisecond }
func (b *playbackBackend) WatchPulse(ctx context.Context, _ platform.WatchHandle) error {
	b.pulses.Add(1)
	b.active.Add(1)
	defer b.active.Add(-1)
	select {
	case b.pulseEntered <- struct{}{}:
	default:
	}
	if b.blockPulse {
		<-ctx.Done()
		return ctx.Err()
	}
	return nil
}
func (b *playbackBackend) InventoryProgress(ctx context.Context, s platform.Session) ([]platform.Progress, error) {
	if b.inventoryCalls.Add(1) > 1 && b.blockInventory {
		select {
		case b.inventoryEntered <- struct{}{}:
		default:
		}
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return b.Fake.InventoryProgress(ctx, s)
}

func newPlaybackBackend() *playbackBackend {
	f := platformtest.NewFake()
	f.Campaigns = []platform.Campaign{camp("c1", "Rust", "d1")}
	f.Streams["c1"] = []platform.Stream{{Channel: "live"}}
	f.Progress["d1"] = &platform.Progress{BenefitID: "d1"}
	return &playbackBackend{Fake: f, inventoryEntered: make(chan struct{}, 1), pulseEntered: make(chan struct{}, 1)}
}

func TestPlaybackContinuesDuringBlockedInventoryWithoutExtraMinuteBeacons(t *testing.T) {
	b := newPlaybackBackend()
	b.blockInventory = true
	r := New(Config{AccountID: "acc", Platform: "twitch", Backend: b, Games: rustGames, TickInterval: time.Hour})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- r.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("runner did not stop")
		}
	})
	select {
	case <-b.inventoryEntered:
	case <-time.After(time.Second):
		t.Fatal("inventory was not entered")
	}
	require.Eventually(t, func() bool { return b.pulses.Load() >= 3 }, time.Second, time.Millisecond)
	require.Equal(t, 1, b.CallCount("Heartbeat:"), "playlist pulses must not send minute beacons")
}

func TestPoolPauseCancelsAndJoinsActivePlaybackPulse(t *testing.T) {
	b := newPlaybackBackend()
	b.blockPulse = true
	p := NewPool(nil)
	defer p.Stop()
	require.NoError(t, p.Reload(context.Background(), []Builder{func() Entry {
		return Entry{ID: "acc", Unit: New(Config{AccountID: "acc", Platform: "twitch", Backend: b, Games: rustGames})}
	}}))
	select {
	case <-b.pulseEntered:
	case <-time.After(time.Second):
		t.Fatal("pulse was not entered")
	}
	resume := p.Pause()
	require.Zero(t, b.active.Load(), "maintenance must wait for active playback requests")
	require.Equal(t, 1, b.CallCount("Stop:"))
	p.Stop()
	resume()
	require.Empty(t, p.Snapshots(), "stopped pool must not be revived by resume")
}

type panicPulser struct{}

func (panicPulser) WatchPulseInterval() time.Duration                      { return time.Second }
func (panicPulser) WatchPulse(context.Context, platform.WatchHandle) error { panic("failed pulse") }
func TestPlaybackPanicIsContained(t *testing.T) {
	require.ErrorContains(t, safeWatchPulse(context.Background(), panicPulser{}, platform.WatchHandle{}), "panicked")
}
