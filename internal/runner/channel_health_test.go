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

type eligibilityBackend struct {
	*platformtest.Fake
	stream    platform.Stream
	live      bool
	lookupErr error
	drops     map[string]struct{}
	dropErr   error
	listErr   error
	onList    func()
}

func (b *eligibilityBackend) ListEligibleChannels(ctx context.Context, session platform.Session, campaign platform.Campaign) ([]platform.Stream, error) {
	if b.onList != nil {
		b.onList()
	}
	if b.listErr != nil {
		return nil, b.listErr
	}
	return b.Fake.ListEligibleChannels(ctx, session, campaign)
}

func (b *eligibilityBackend) LookupChannel(context.Context, platform.Session, string) (platform.Stream, bool, error) {
	return b.stream, b.live, b.lookupErr
}
func (b *eligibilityBackend) AvailableDropIDs(context.Context, platform.Session, string) (map[string]struct{}, error) {
	return b.drops, b.dropErr
}

func TestWatchRechecksMissingChannel(t *testing.T) {
	for _, tc := range []struct {
		name                        string
		live                        bool
		game                        string
		lookupErr, dropErr, listErr error
		drops                       map[string]struct{}
		restricted                  bool
		swap                        bool
	}{
		{name: "directory omission", live: true, game: "Rust", drops: map[string]struct{}{"d1": {}}},
		{name: "list failure", listErr: errors.New("directory timeout")},
		{name: "lookup failure", lookupErr: errors.New("timeout")},
		{name: "drop lookup failure", live: true, game: "Rust", dropErr: errors.New("timeout")},
		{name: "offline", swap: true},
		{name: "category changed", live: true, game: "Other", swap: true},
		{name: "wrong reward", live: true, game: "Rust", drops: map[string]struct{}{"other": {}}, swap: true},
		{name: "unconfirmed restricted campaign", live: true, game: "Rust", restricted: true, swap: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := platformtest.NewFake()
			b := &eligibilityBackend{Fake: f, live: tc.live, stream: platform.Stream{Channel: "a", ChannelID: "ida", Game: tc.game}, lookupErr: tc.lookupErr, dropErr: tc.dropErr, drops: tc.drops}
			r := watching(t, f, func(c *Config) { c.Backend = b; c.Platform = "twitch"; c.TickInterval = liveCheckEvery })
			f.Streams["c1"] = []platform.Stream{{Channel: "b"}}
			b.listErr = tc.listErr
			if tc.restricted {
				r.current.Campaign.AllowedChannelCount = 1
			}
			steps(t, r, 1)
			if tc.swap {
				require.Equal(t, StateStarting, r.State())
				require.Equal(t, 1, f.CallCount("Stop:"))
			} else {
				require.Equal(t, StateWatching, r.State())
				require.Zero(t, f.CallCount("Stop:"))
				require.Equal(t, 1, f.CallCount("Inventory")-1)
			}
		})
	}
}

func TestNoStreamBackoffAndWaitingDuration(t *testing.T) {
	f := platformtest.NewFake()
	f.Campaigns = []platform.Campaign{camp("c1", "Rust", "d1")}
	r := newRunner(f, nil)
	steps(t, r, 2)
	require.Equal(t, StateSleeping, r.State())
	since := r.Snapshot().StreamWaitSince
	require.False(t, since.IsZero())
	previous := time.Duration(0)
	for i := 0; i < 6; i++ {
		r.noStreamAttempts = i
		delay := r.idleWaitLocked(StateSleeping, 0)
		if i < 5 {
			require.GreaterOrEqual(t, delay, previous)
		}
		require.LessOrEqual(t, delay, 5*time.Minute)
		if i == 0 {
			require.GreaterOrEqual(t, delay, 30*time.Second)
			require.LessOrEqual(t, delay, 60*time.Second)
		}
		previous = delay
	}
	// Failed searches retain the start of the same waiting episode.
	r.setState(context.Background(), StatePicking)
	steps(t, r, 2)
	require.Equal(t, since, r.Snapshot().StreamWaitSince)
	f.Streams["c1"] = []platform.Stream{{Channel: "live"}}
	r.setState(context.Background(), StatePicking)
	steps(t, r, 2)
	require.Equal(t, StateWatching, r.State())
	require.True(t, r.Snapshot().StreamWaitSince.IsZero())
	require.Zero(t, r.noStreamAttempts)
	// Account jitter is stable and spreads a fleet's first retries.
	delays := map[time.Duration]bool{}
	for _, id := range []string{"acc1", "acc2", "acc3", "acc4"} {
		r.cfg.AccountID = id
		r.sleepNoStream = true
		delays[r.idleWaitLocked(StateSleeping, 0)] = true
	}
	require.Greater(t, len(delays), 1)
}

func TestPlayback404ConfirmationAndSuccessReset(t *testing.T) {
	now := time.Now()
	var h playbackHealth
	h.observe(now, platform.ErrPlaybackUnavailable)
	require.False(t, h.ready)
	h.observe(now.Add(20*time.Second), platform.ErrPlaybackUnavailable)
	require.False(t, h.ready)
	h.observe(now.Add(40*time.Second), platform.ErrPlaybackUnavailable)
	require.True(t, h.ready)
	h.observe(now.Add(time.Minute), nil)
	require.False(t, h.ready)
	require.Zero(t, h.failures)
	h.observe(now, errors.New("network timeout"))
	require.Zero(t, h.failures)
}

func failPlayback(r *Runner, now time.Time) {
	r.playbackHealth.observe(now.Add(-40*time.Second), platform.ErrPlaybackUnavailable)
	r.playbackHealth.observe(now.Add(-20*time.Second), platform.ErrPlaybackUnavailable)
	r.playbackHealth.observe(now, platform.ErrPlaybackUnavailable)
}
func TestPlaybackRecoveryReconnectsThenRotatesWithCooldown(t *testing.T) {
	f := platformtest.NewFake()
	r := watching(t, f, func(c *Config) { c.Platform = "twitch" })
	f.Streams["c1"] = []platform.Stream{{Channel: "a", ViewerCount: 10}, {Channel: "b", ViewerCount: 50}}
	now := time.Now()
	r.lastBeatAt = now
	failPlayback(r, now)
	require.True(t, r.recoverPlayback(context.Background(), *r.handle, *r.current, now))
	steps(t, r, 1)
	require.Equal(t, "a", r.Snapshot().Channel, "first recovery reconnects the same stream even with a more popular alternative")
	require.Equal(t, now, r.lastBeatAt)
	require.Greater(t, r.untilNextBeat(), 50*time.Second, "recovery must not send an extra minute beacon")
	failPlayback(r, now.Add(time.Minute))
	require.False(t, r.recoverPlayback(context.Background(), *r.handle, *r.current, now.Add(time.Minute)))
	later := now.Add(playbackRecoveryCooldown)
	failPlayback(r, later)
	require.True(t, r.recoverPlayback(context.Background(), *r.handle, *r.current, later))
	steps(t, r, 1)
	require.Equal(t, "b", r.Snapshot().Channel)
	require.Equal(t, 3, f.CallCount("Start:"))
	require.Zero(t, f.CallCount("Claim:"))
}

func TestCompletedRewardTakesPriorityOverPlaybackRecovery(t *testing.T) {
	f := platformtest.NewFake()
	f.Progress["d1"] = &platform.Progress{BenefitID: "d1", MinutesWatched: 59, InstanceID: "instance"}
	r := watching(t, f, func(c *Config) { c.Platform = "twitch" })
	failPlayback(r, time.Now())
	steps(t, r, 1)
	require.Equal(t, StateClaiming, r.State())
	require.Zero(t, f.CallCount("Stop:"))
	steps(t, r, 1)
	require.Equal(t, 1, f.CallCount("Claim:"))
}

func TestRepeatedPlayback404WithoutAlternativeReconnectsWithoutBlockingChannel(t *testing.T) {
	f := platformtest.NewFake()
	r := watching(t, f, func(c *Config) { c.Platform = "twitch" })
	now := time.Now()
	r.playbackHealth.lastChannel = "a"
	r.playbackHealth.lastAttempt = now.Add(-3 * time.Minute)
	failPlayback(r, now)
	require.True(t, r.recoverPlayback(context.Background(), *r.handle, *r.current, now))
	require.Empty(t, r.stalled)
	steps(t, r, 1)
	require.Equal(t, StateWatching, r.State())
	require.Equal(t, "a", r.Snapshot().Channel)
}

func TestPlaybackSuccessDuringAlternativeLookupCancelsRecovery(t *testing.T) {
	f := platformtest.NewFake()
	b := &eligibilityBackend{Fake: f}
	r := watching(t, f, func(c *Config) { c.Platform = "twitch"; c.Backend = b })
	now := time.Now()
	r.playbackHealth.lastChannel = "a"
	r.playbackHealth.lastAttempt = now.Add(-3 * time.Minute)
	failPlayback(r, now)
	b.onList = func() { r.mu.Lock(); r.playbackHealth.observe(now, nil); r.mu.Unlock() }
	require.False(t, r.recoverPlayback(context.Background(), *r.handle, *r.current, now))
	require.Equal(t, StateWatching, r.State())
	require.Zero(t, f.CallCount("Stop:"))
}

func TestAdvisoryMismatchRequiresFreshGainFromSameWatch(t *testing.T) {
	for _, tc := range []struct {
		name                             string
		baselineBenefit                  string
		age                              time.Duration
		minutes                          int
		missing                          bool
		inventoryErr                     error
		offline, wrongGame, newBroadcast bool
		keep                             bool
	}{
		{name: "fresh gain keeps viewing", baselineBenefit: "d1", age: time.Minute, minutes: 11, keep: true},
		{name: "flat minutes still switch", baselineBenefit: "d1", age: time.Minute, minutes: 10},
		{name: "regressed minutes still switch", baselineBenefit: "d1", age: time.Minute, minutes: 9},
		{name: "missing reward still switches", baselineBenefit: "d1", age: time.Minute, missing: true},
		{name: "another reward cannot establish gain", baselineBenefit: "other", age: time.Minute, minutes: 11},
		{name: "stale baseline cannot establish gain", baselineBenefit: "d1", age: 3 * time.Minute, minutes: 11},
		{name: "previous watch baseline cleared", baselineBenefit: "d1", minutes: 11},
		{name: "failed inventory check keeps viewing", baselineBenefit: "d1", age: time.Minute, inventoryErr: errors.New("timeout"), keep: true},
		{name: "offline overrides old gain", baselineBenefit: "d1", age: time.Minute, minutes: 11, offline: true},
		{name: "changed category overrides gain", baselineBenefit: "d1", age: time.Minute, minutes: 11, wrongGame: true},
		{name: "new broadcast overrides gain", baselineBenefit: "d1", age: time.Minute, minutes: 11, newBroadcast: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := platformtest.NewFake()
			b := &eligibilityBackend{Fake: f, live: !tc.offline, stream: platform.Stream{Channel: "a", ChannelID: "ida", Game: "Rust", BroadcastID: "old"}, drops: map[string]struct{}{"other": {}}}
			r := watching(t, f, func(c *Config) { c.Backend = b; c.Platform = "twitch" })
			f.Streams["c1"] = []platform.Stream{{Channel: "b"}}
			if !tc.missing {
				f.Progress["d1"] = &platform.Progress{BenefitID: "d1", MinutesWatched: tc.minutes}
			}
			f.InventoryErr = tc.inventoryErr
			r.stream.BroadcastID = "old"
			r.farm.benefit = tc.baselineBenefit
			r.farm.minutes = 10
			if tc.age > 0 {
				r.farm.last = time.Now().Add(-tc.age)
			}
			if tc.wrongGame {
				b.stream.Game = "Other"
			}
			if tc.newBroadcast {
				b.stream.BroadcastID = "new"
			}
			shouldSwitch := r.checkWatchChannel(context.Background(), *r.handle, *r.current)
			require.Equal(t, !tc.keep, shouldSwitch)
			require.Zero(t, f.CallCount("Claim:"))
		})
	}
}

func TestAdvisoryMismatchDoesNotBlockNormalClaimOrFreezeSafeguard(t *testing.T) {
	for _, complete := range []bool{false, true} {
		t.Run(map[bool]string{false: "flat progress rotates", true: "complete reward claims once"}[complete], func(t *testing.T) {
			f := platformtest.NewFake()
			b := &eligibilityBackend{Fake: f, live: true, stream: platform.Stream{Channel: "a", ChannelID: "ida", Game: "Rust"}, drops: map[string]struct{}{"other": {}}}
			r := watching(t, f, func(c *Config) { c.Backend = b; c.Platform = "twitch"; c.TickInterval = liveCheckEvery })
			r.farm.observe(time.Now().Add(-time.Minute), "d1", 59)
			r.lastPollMin = 59
			f.Streams["c1"] = []platform.Stream{{Channel: "b"}}
			f.Progress["d1"] = &platform.Progress{BenefitID: "d1", MinutesWatched: 59, InstanceID: "instance"}
			if !complete {
				f.OnHeartbeat = func(*platformtest.Fake) {}
			}
			steps(t, r, 1)
			if complete {
				require.Equal(t, StateClaiming, r.State())
				require.Zero(t, f.CallCount("Stop:"))
				steps(t, r, 1)
				require.Equal(t, 1, f.CallCount("Claim:"))
			} else {
				require.Equal(t, StateStarting, r.State())
				require.Equal(t, 1, f.CallCount("Stop:"))
				require.Zero(t, f.CallCount("Claim:"))
			}
		})
	}
}
