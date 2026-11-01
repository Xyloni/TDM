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

func unlinkedWatch(t *testing.T) (*Runner, *platformtest.Fake, *spyNotifier, *fakeClaims) {
	t.Helper()
	f := platformtest.NewFake()
	r, n, cl, _ := claimSetup(f, 60, 120)
	r.cfg.Platform = "twitch"
	r.cfg.AutoSkipUnlinked = func() bool { return true }
	r.cfg.TickInterval = time.Nanosecond
	f.Campaigns[0].Platform = "twitch"
	f.Campaigns[0].AccountLinked = false
	f.Campaigns[0].AccountLinkURL = "https://game.example/connect"
	f.Campaigns[0].Benefits[1].Preconditions = []string{"d1"}
	f.Progress["d1"].MinutesWatched = 59
	steps(t, r, 2)
	require.Equal(t, StateWatching, r.State())
	r.Rescan()
	return r, f, n, cl
}

func TestRescanDoesNotClaimTheRewardOwnedByActiveWatch(t *testing.T) {
	r, f, n, cl := unlinkedWatch(t)
	steps(t, r, 1) // heartbeat reaches 60; rescan sees the same completed reward
	require.Equal(t, StateClaiming, r.State())
	require.Zero(t, f.CallCount("Claim:d1"), "rescan must leave the active reward to the normal claim flow")
	steps(t, r, 1)
	require.Equal(t, 1, f.CallCount("Claim:d1"))
	require.Len(t, n.of("claim"), 1)
	require.Equal(t, []string{"acc:d1:"}, cl.recorded())
	steps(t, r, 2)
	require.Equal(t, StateWatching, r.State())
	require.Equal(t, "d2", r.Snapshot().Current.Benefit.ID, "the next reward must still unlock")
	require.Equal(t, 1, f.CallCount("Claim:d1"))
}

func TestRescanGuardPreservesRetryAfterFailedClaim(t *testing.T) {
	r, f, n, cl := unlinkedWatch(t)
	f.ClaimErr = errors.New("temporary transport failure")
	steps(t, r, 1)
	require.Zero(t, f.CallCount("Claim:d1"))
	require.Error(t, r.claim(context.Background()))
	require.Equal(t, 1, f.CallCount("Claim:d1"))
	require.Empty(t, n.of("claim"))
	require.Empty(t, cl.recorded())
	f.ClaimErr = nil
	require.NoError(t, r.claim(context.Background()))
	require.Equal(t, 2, f.CallCount("Claim:d1"))
	require.Len(t, n.of("claim"), 1)
	require.Len(t, cl.recorded(), 1)
}

func TestRescanStillClaimsOtherCompletedUnlinkedRewards(t *testing.T) {
	r, f, n, cl := unlinkedWatch(t)
	f.Progress["d1"].MinutesWatched = 10
	f.Campaigns[0].Benefits = append(f.Campaigns[0].Benefits, platform.DropBenefit{ID: "other", CampaignID: "c1", RequiredMinutes: 30})
	f.Progress["other"] = &platform.Progress{BenefitID: "other", MinutesWatched: 30}
	steps(t, r, 1)
	require.Equal(t, StateWatching, r.State())
	require.Equal(t, "d1", r.Snapshot().Current.Benefit.ID)
	require.Equal(t, 1, f.CallCount("Claim:other"))
	require.Zero(t, f.CallCount("Claim:d1"))
	require.Len(t, n.of("claim"), 1)
	require.Equal(t, []string{"acc:other:"}, cl.recorded())
}
