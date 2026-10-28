package main

import (
	"time"

	"github.com/aalejandrofer/grubdrops/internal/api"
	"github.com/aalejandrofer/grubdrops/internal/plan"
	"github.com/aalejandrofer/grubdrops/internal/runner"
	"github.com/aalejandrofer/grubdrops/internal/store/gen"
)

const accountGroupQueueCheck = 10 * time.Second

// runAccountGroupQueue advances imported account groups serially. A group
// finishes once each enabled member has scanned and has no remaining target
// in the campaign currently being processed.
func (ro *roster) runAccountGroupQueue() {
	ticker := time.NewTicker(accountGroupQueueCheck)
	defer ticker.Stop()
	for {
		select {
		case <-ro.ctx.Done():
			return
		case <-ticker.C:
			ro.checkAccountGroupQueue()
		}
	}
}

func (ro *roster) checkAccountGroupQueue() {
	if ro.settings == nil {
		return
	}
	ctx := ro.ctx
	state, err := ro.settings.AccountGroupsQueue(ctx)
	if err != nil || state.Active == 0 || len(state.Groups) == 0 {
		if err != nil {
			ro.log().Warn("read account group queue failed", "kind", "error", "err", err)
		}
		return
	}
	accounts, err := ro.q.ListAllAccounts(ctx)
	if err != nil {
		ro.log().Warn("list accounts for group queue failed", "kind", "error", "err", err)
		return
	}
	snapshots := make(map[string]runner.Snapshot)
	for _, snapshot := range ro.pool.Snapshots() {
		snapshots[snapshot.AccountID] = snapshot
	}
	var members []gen.Account
	for _, account := range accounts {
		if state.Groups[account.ID] == state.Active {
			members = append(members, account)
		}
	}
	if len(members) == 0 {
		return
	}

	focus := state.CampaignID
	if focus == "" {
		focus = firstSnapshotCampaign(members, snapshots)
		if focus == "" {
			return
		}
		changed, err := ro.settings.SetAccountGroupCampaign(ctx, state.Active, focus)
		if err != nil {
			ro.log().Warn("save account group campaign failed", "kind", "error", "group", state.Active, "err", err)
			return
		}
		if !changed {
			return
		}
		state.CampaignID = focus
		for _, account := range members {
			ro.pool.RescanAccount(account.ID)
		}
	}
	if !accountGroupCampaignComplete(members, snapshots, focus) {
		return
	}

	previous := state.Active
	next, advanced, err := ro.settings.AdvanceAccountGroup(ctx, previous, focus)
	if err != nil {
		ro.log().Error("advance account group queue failed", "kind", "error", "group", previous, "err", err)
		return
	}
	if !advanced {
		return
	}
	ro.log().Info("account group queue advanced", "kind", "state", "from_group", previous, "to_group", next.Active, "campaign", focus, "next_campaign", next.CampaignID == "")
	for _, account := range accounts {
		group := state.Groups[account.ID]
		if group == previous || group == next.Active {
			ro.reloadAccount(account.ID)
		}
	}
	if ro.hub != nil {
		ro.hub.Publish(api.Event{Name: api.EventAccounts, Data: struct{}{}})
	}
}

func firstSnapshotCampaign(members []gen.Account, snapshots map[string]runner.Snapshot) string {
	for _, account := range members {
		snapshot, ok := snapshots[account.ID]
		if !ok {
			continue
		}
		if snapshot.Current != nil && snapshot.Current.Campaign.ID != "" {
			return snapshot.Current.Campaign.ID
		}
		for _, target := range snapshot.UpNext {
			if target.Campaign.ID != "" {
				return target.Campaign.ID
			}
		}
	}
	return ""
}

func accountGroupCampaignComplete(members []gen.Account, snapshots map[string]runner.Snapshot, campaignID string) bool {
	participants := 0
	for _, account := range members {
		if account.Enabled == 0 {
			continue
		}
		participants++
		snapshot, ok := snapshots[account.ID]
		if !ok || snapshot.State == runner.StateNeedsLogin || snapshot.State == runner.StateNoGames || snapshot.LastScanAt.IsZero() {
			return false
		}
		if snapshot.Current != nil && snapshot.Current.Campaign.ID == campaignID {
			return false
		}
		for _, target := range snapshot.UpNext {
			if target.Campaign.ID == campaignID {
				return false
			}
		}
		for _, blocked := range snapshot.Blocked {
			if blocked.Campaign.ID != campaignID || blocked.Reason == plan.ReasonSkipped {
				continue
			}
			// A Twitch drop can reach its full watch time but remain unclaimable
			// until the user links the game's account. That link blocker should
			// not hold the account-group queue forever once all required minutes
			// have been watched.
			if blocked.Reason == plan.ReasonNotLinked && blocked.Minutes >= blocked.Benefit.RequiredMinutes {
				continue
			}
			return false
		}
	}
	// A fully paused group is not an active queue turn.
	return participants > 0
}
