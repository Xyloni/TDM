package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/aalejandrofer/grubdrops/internal/plan"
	"github.com/aalejandrofer/grubdrops/internal/platform"
	"github.com/aalejandrofer/grubdrops/internal/runner"
	"github.com/aalejandrofer/grubdrops/internal/store/gen"
)

func TestAccountGroupCampaignComplete(t *testing.T) {
	lastScan := time.Now()
	member := gen.Account{ID: "a1", Enabled: 1}
	snapshot := runner.Snapshot{
		AccountID: "a1", State: runner.StateSleeping, LastScanAt: lastScan,
		UpNext: []plan.Target{{Campaign: platformCampaign("c1"), Benefit: planBenefit("b1")}},
	}
	assert.False(t, accountGroupCampaignComplete([]gen.Account{member}, map[string]runner.Snapshot{"a1": snapshot}, "c1"))

	snapshot.UpNext = nil
	assert.True(t, accountGroupCampaignComplete([]gen.Account{member}, map[string]runner.Snapshot{"a1": snapshot}, "c1"))

	snapshot.Blocked = []plan.Blocked{{Target: plan.Target{Campaign: platformCampaign("c1"), Benefit: platform.DropBenefit{RequiredMinutes: 15}, Minutes: 14}, Reason: plan.ReasonNotLinked}}
	assert.False(t, accountGroupCampaignComplete([]gen.Account{member}, map[string]runner.Snapshot{"a1": snapshot}, "c1"))
	snapshot.Blocked[0].Minutes = 15
	assert.True(t, accountGroupCampaignComplete([]gen.Account{member}, map[string]runner.Snapshot{"a1": snapshot}, "c1"))
	snapshot.Blocked[0].Reason = plan.ReasonSkipped
	assert.True(t, accountGroupCampaignComplete([]gen.Account{member}, map[string]runner.Snapshot{"a1": snapshot}, "c1"))

	snapshot.State = runner.StateNeedsLogin
	assert.False(t, accountGroupCampaignComplete([]gen.Account{member}, map[string]runner.Snapshot{"a1": snapshot}, "c1"))
}

func platformCampaign(id string) platform.Campaign { return platform.Campaign{ID: id} }

func planBenefit(id string) platform.DropBenefit { return platform.DropBenefit{ID: id} }
