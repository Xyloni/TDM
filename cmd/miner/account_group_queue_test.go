package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aalejandrofer/grubdrops/internal/notify"
	"github.com/aalejandrofer/grubdrops/internal/plan"
	"github.com/aalejandrofer/grubdrops/internal/platform"
	"github.com/aalejandrofer/grubdrops/internal/runner"
	"github.com/aalejandrofer/grubdrops/internal/store"
	"github.com/aalejandrofer/grubdrops/internal/store/gen"
)

type groupNotificationFunc func(context.Context, string, map[string]any) error

func (f groupNotificationFunc) Notify(ctx context.Context, event string, fields map[string]any) error {
	return f(ctx, event, fields)
}

type groupSnapshotUnit struct{ snapshot runner.Snapshot }

func (u groupSnapshotUnit) Snapshot() runner.Snapshot     { return u.snapshot }
func (u groupSnapshotUnit) Run(ctx context.Context) error { <-ctx.Done(); return ctx.Err() }

func TestGroupHandoffNotifiesOnceAfterCommitAndKeepsRunningOnDeliveryFailure(t *testing.T) {
	for _, deliveryErr := range []error{nil, errors.New("pin permission denied")} {
		t.Run(fmtDeliveryCase(deliveryErr), func(t *testing.T) {
			e := newRosterEnv(t)
			e.account(t, "a1", 1, false)
			e.account(t, "a2", 1, false)
			e.ro.settings = store.NewSettings(e.q)
			require.NoError(t, e.ro.settings.SetAccountGroup(e.ctx, "a1", 1))
			require.NoError(t, e.ro.settings.SetAccountGroup(e.ctx, "a2", 2))
			_, err := e.ro.settings.SetAccountGroupCampaign(e.ctx, 1, "campaign")
			require.NoError(t, err)
			snapshot := runner.Snapshot{AccountID: "a1", State: runner.StateSleeping, LastScanAt: time.Now(),
				UpNext: []plan.Target{{Campaign: platformCampaign("campaign"), Benefit: planBenefit("drop")}}}
			builder := func() runner.Entry { return runner.Entry{ID: "a1", Unit: groupSnapshotUnit{snapshot: snapshot}} }
			require.NoError(t, e.ro.pool.Reload(e.ctx, []runner.Builder{builder}))
			calls := 0
			e.ro.notifier = groupNotificationFunc(func(ctx context.Context, event string, fields map[string]any) error {
				calls++
				assert.Equal(t, notify.TargetTelegram, notify.TargetFrom(ctx))
				assert.Equal(t, notify.EventGroupSwitch, event)
				_, hasDeadline := ctx.Deadline()
				assert.True(t, hasDeadline)
				state, err := e.ro.settings.AccountGroupsQueue(ctx)
				require.NoError(t, err)
				assert.Equal(t, 2, state.Active, "notification must follow committed handoff")
				assert.Contains(t, fields["msg"], "Группа 1")
				assert.Contains(t, fields["msg"], "группа 2")
				return deliveryErr
			})
			e.ro.checkAccountGroupQueue()
			assert.Zero(t, calls, "incomplete group must not notify")
			snapshot.UpNext = nil
			e.ro.pool.ReloadAccount("a1", builder)
			e.ro.checkAccountGroupQueue()
			assert.Equal(t, 1, calls)
			state, err := e.ro.settings.AccountGroupsQueue(e.ctx)
			require.NoError(t, err)
			assert.Equal(t, 2, state.Active)
			e.ro.checkAccountGroupQueue()
			assert.Equal(t, 1, calls, "the next queue check must not repeat the handoff")
		})
	}
}

func fmtDeliveryCase(err error) string {
	if err != nil {
		return "delivery_failed"
	}
	return "delivered"
}

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
