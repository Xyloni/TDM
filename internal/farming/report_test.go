package farming

import (
	"github.com/aalejandrofer/grubdrops/internal/plan"
	"github.com/aalejandrofer/grubdrops/internal/platform"
	"github.com/aalejandrofer/grubdrops/internal/runner"
	"github.com/aalejandrofer/grubdrops/internal/store"
	"github.com/aalejandrofer/grubdrops/internal/store/gen"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func target(id string, minutes, required int) plan.Target {
	return plan.Target{Campaign: platform.Campaign{ID: "campaign", Name: "Test"}, Benefit: platform.DropBenefit{ID: id, Name: id, RequiredMinutes: required}, Minutes: minutes}
}
func observing(now time.Time, id string, rate float64) runner.Snapshot {
	t := target("one", 10, 30)
	return runner.Snapshot{AccountID: id, State: runner.StateWatching, Current: &t, Channel: "live", LastScanAt: now, LastPollAt: now, StartedAt: now,
		Farming: runner.FarmingStats{Minutes15: runner.FarmingWindow{Rate: &rate, Ready: true, ObservationMinutes: 15}}}
}

func TestGroupForecastUsesSlowestMemberAndExcludesOtherGroups(t *testing.T) {
	now := time.Now()
	accounts := []gen.Account{{ID: "a", Enabled: 1}, {ID: "b", Enabled: 1}, {ID: "c", Enabled: 1}, {ID: "paused", Enabled: 0}}
	a := observing(now, "a", 1)
	b := observing(now, "b", .8)
	b.Farming.Rotations15 = 1
	q := store.AccountGroupQueue{Active: 1, CampaignID: "campaign", Groups: map[string]int{"a": 1, "b": 1, "c": 2, "paused": 1}}
	r := Build(now, accounts, []runner.Snapshot{a, b}, q)
	require.Equal(t, 2, r.Group.Total)
	require.Equal(t, 25, *r.Group.ETAMinutes)
	require.Equal(t, "b", r.Group.Delayers[0].ID)
	require.Equal(t, 1, r.Slow)
	require.Equal(t, 1, r.Rotated)
	require.Equal(t, "waiting_group", r.Accounts[2].Reason)
}

func TestGroupForecastUnknownLoginDoesNotLookComplete(t *testing.T) {
	now := time.Now()
	r := Build(now, []gen.Account{{ID: "a", Enabled: 1}}, []runner.Snapshot{{AccountID: "a", State: runner.StateNeedsLogin, LastScanAt: now}}, store.AccountGroupQueue{})
	require.Nil(t, r.Group.ETAMinutes)
	require.Equal(t, 1, r.Group.Unknown)
	require.Zero(t, r.Group.Completed)
	require.Equal(t, 1, r.Problems)
}

func TestGroupForecastIncludesReachableRewardChainButNotExcludedPrerequisites(t *testing.T) {
	now := time.Now()
	s := observing(now, "a", 1)
	two := target("two", 0, 40)
	two.Benefit.Preconditions = []string{"one"}
	three := target("three", 0, 50)
	three.Benefit.Preconditions = []string{"two"}
	s.Blocked = []plan.Blocked{{Target: three, Reason: plan.ReasonPrecondition}, {Target: two, Reason: plan.ReasonPrecondition}}
	accounts := []gen.Account{{ID: "a", Enabled: 1, Platform: "twitch"}}
	r := Build(now, accounts, []runner.Snapshot{s}, store.AccountGroupQueue{})
	require.Equal(t, 110, *r.Group.ETAMinutes)
	s.Current = nil
	s.Blocked = append(s.Blocked, plan.Blocked{Target: target("one", 0, 30), Reason: plan.ReasonExcluded})
	r = Build(now, accounts, []runner.Snapshot{s}, store.AccountGroupQueue{})
	require.Nil(t, r.Group.ETAMinutes)
	require.False(t, r.Accounts[0].Complete)
}

func TestGroupForecastCompletedAndStaleAccounts(t *testing.T) {
	now := time.Now()
	s := observing(now, "a", 1)
	s.LastPollAt = now.Add(-3 * time.Minute)
	accounts := []gen.Account{{ID: "a", Enabled: 1}}
	r := Build(now, accounts, []runner.Snapshot{s}, store.AccountGroupQueue{})
	require.Equal(t, "stale", r.Accounts[0].Reason)
	require.Nil(t, r.Group.ETAMinutes)
	s.Current = nil
	s.State = runner.StateSleeping
	r = Build(now, accounts, []runner.Snapshot{s}, store.AccountGroupQueue{})
	require.Equal(t, 1, r.Group.Completed)
	require.Zero(t, *r.Group.ETAMinutes)
}
