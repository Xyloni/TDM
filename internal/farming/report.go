// Package farming derives read-only fleet diagnostics from runner snapshots.
package farming

import (
	"math"
	"sort"
	"time"

	"github.com/aalejandrofer/grubdrops/internal/plan"
	"github.com/aalejandrofer/grubdrops/internal/runner"
	"github.com/aalejandrofer/grubdrops/internal/store"
	"github.com/aalejandrofer/grubdrops/internal/store/gen"
)

type Account struct {
	ID             string              `json:"id"`
	Name           string              `json:"name"`
	Platform       string              `json:"platform"`
	Group          int                 `json:"group"`
	Channel        string              `json:"channel"`
	Reward         string              `json:"reward"`
	Reason         string              `json:"reason"`
	Stats          runner.FarmingStats `json:"stats"`
	Remaining      int                 `json:"remaining"`
	ETAMinutes     *int                `json:"eta_minutes"`
	WaitingSeconds int                 `json:"waiting_seconds"`
	Complete       bool                `json:"complete"`
}

type Group struct {
	ID          int        `json:"id"`
	CampaignID  string     `json:"campaign_id"`
	Campaign    string     `json:"campaign"`
	Total       int        `json:"total"`
	Completed   int        `json:"completed"`
	Unknown     int        `json:"unknown"`
	ETAMinutes  *int       `json:"eta_minutes"`
	EstimatedAt *time.Time `json:"estimated_at"`
	Delayers    []Account  `json:"delayers"`
}

type Report struct {
	At       time.Time `json:"at"`
	Accounts []Account `json:"accounts"`
	Group    Group     `json:"group"`
	Observed int       `json:"observed"`
	Slow     int       `json:"slow"`
	Problems int       `json:"problems"`
	Rotated  int       `json:"rotated"`
}

func ignorable(b plan.Blocked) bool {
	return b.Reason == plan.ReasonSkipped || b.Reason == plan.ReasonExcluded || (b.Reason == plan.ReasonNotLinked && b.Minutes >= b.Benefit.RequiredMinutes)
}

func Build(now time.Time, accounts []gen.Account, snapshots []runner.Snapshot, queue store.AccountGroupQueue) Report {
	r := Report{At: now, Accounts: []Account{}, Group: Group{ID: queue.Active, CampaignID: queue.CampaignID, Delayers: []Account{}}}
	byID := map[string]runner.Snapshot{}
	for _, s := range snapshots {
		byID[s.AccountID] = s
	}
	// While a queue turn is being selected, use its first known campaign.
	if queue.Active > 0 && r.Group.CampaignID == "" {
		for _, a := range accounts {
			if queue.Groups[a.ID] != queue.Active || a.Enabled == 0 {
				continue
			}
			s := byID[a.ID]
			if s.Current != nil {
				r.Group.CampaignID = s.Current.Campaign.ID
				break
			}
			if len(s.UpNext) > 0 {
				r.Group.CampaignID = s.UpNext[0].Campaign.ID
				break
			}
		}
	}
	maxETA := 0
	for _, a := range accounts {
		s, exists := byID[a.ID]
		x := Account{ID: a.ID, Name: store.AccountName(a), Platform: a.Platform, Group: queue.Groups[a.ID], Channel: s.Channel, Stats: s.Farming}
		member := a.Enabled != 0 && (queue.Active == 0 || x.Group == queue.Active)
		focus := ""
		if queue.Active > 0 && x.Group == queue.Active {
			focus = r.Group.CampaignID
		}
		pending := false
		blocked := false
		seen := map[string]bool{}
		add := func(t plan.Target) {
			if focus != "" && t.Campaign.ID != focus {
				return
			}
			if seen[t.Benefit.ID] {
				return
			}
			seen[t.Benefit.ID] = true
			pending = true
			remaining := max(0, t.Benefit.RequiredMinutes-t.Minutes)
			if a.Platform == "kick" && focus != "" {
				x.Remaining = max(x.Remaining, remaining)
			} else {
				x.Remaining += remaining
			}
			if t.Campaign.ID == r.Group.CampaignID {
				r.Group.Campaign = t.Campaign.Name
			}
		}
		if s.Current != nil {
			x.Reward = s.Current.Benefit.Name
			add(*s.Current)
		}
		for _, t := range s.UpNext {
			add(t)
		}
		// Prerequisites whose unfinished rewards are in the plan can finish
		// naturally. Include their later rewards in the estimate, but do not
		// pretend an excluded or skipped prerequisite has been claimed.
		blockedIDs := map[string]bool{}
		for _, b := range s.Blocked {
			blockedIDs[b.Benefit.ID] = true
		}
		var later []plan.Blocked
		for _, b := range s.Blocked {
			if focus != "" && b.Campaign.ID != focus {
				continue
			}
			if b.Reason == plan.ReasonPrecondition {
				later = append(later, b)
			} else if !ignorable(b) {
				blocked = true
			}
		}
		for len(later) > 0 {
			var rest []plan.Blocked
			for _, b := range later {
				reachable := true
				for _, p := range b.Benefit.Preconditions {
					if blockedIDs[p] && !seen[p] {
						reachable = false
						break
					}
				}
				if reachable {
					add(b.Target)
				} else {
					rest = append(rest, b)
				}
			}
			if len(rest) == len(later) {
				blocked = true
				break
			}
			later = rest
		}
		x.Complete = exists && !s.LastScanAt.IsZero() && !pending && !blocked && s.State != runner.StateNeedsLogin && s.State != runner.StateNoGames
		switch {
		case a.Enabled == 0:
			x.Reason = "paused"
			x.Complete = false
		case queue.Active > 0 && x.Group > 0 && x.Group != queue.Active:
			x.Reason = "waiting_group"
			x.Complete = false
		case !exists:
			x.Reason = "waiting_scan"
		case s.State == runner.StateNeedsLogin:
			x.Reason = "needs_login"
		case s.State == runner.StateNoGames:
			x.Reason = "no_games"
		case s.State == runner.StateClaiming:
			x.Reason = "claiming"
		case x.Complete:
			x.Reason = "complete"
		case blocked && s.Current == nil:
			x.Reason = "blocked"
		case s.State == runner.StateSleeping || s.State == runner.StateAwaitingConnect:
			x.Reason = "waiting_stream"
		case s.State == runner.StateForceWatching:
			x.Reason = "force_watching"
		case s.State != runner.StateWatching:
			x.Reason = "starting"
		case (s.LastPollAt.IsZero() && now.Sub(s.StartedAt) > 150*time.Second) || (!s.LastPollAt.IsZero() && now.Sub(s.LastPollAt) > 150*time.Second):
			x.Reason = "stale"
		case !x.Stats.Minutes15.Ready:
			x.Reason = "warming"
		case x.Stats.Minutes15.Rate != nil && *x.Stats.Minutes15.Rate < .9:
			x.Reason = "slow"
		default:
			x.Reason = "healthy"
		}
		if (x.Reason == "waiting_stream" || x.Reason == "starting") && !s.StreamWaitSince.IsZero() {
			x.WaitingSeconds = max(0, int(now.Sub(s.StreamWaitSince).Seconds()))
		}
		if x.Complete {
			zero := 0
			x.ETAMinutes = &zero
		} else if !blocked && s.State == runner.StateWatching && x.Reason != "stale" && x.Reason != "paused" && x.Reason != "waiting_group" && x.Stats.Minutes15.Rate != nil && *x.Stats.Minutes15.Rate > 0 {
			// Inventory can arrive in batches; cap optimistic rates at realtime.
			eta := int(math.Ceil(float64(x.Remaining) / min(1, *x.Stats.Minutes15.Rate)))
			x.ETAMinutes = &eta
		}
		if member {
			r.Group.Total++
			if x.Complete {
				r.Group.Completed++
			} else {
				r.Group.Delayers = append(r.Group.Delayers, x)
				if x.ETAMinutes == nil {
					r.Group.Unknown++
				} else {
					maxETA = max(maxETA, *x.ETAMinutes)
				}
			}
			if x.Stats.Minutes15.Ready {
				r.Observed++
			}
			if x.Reason == "slow" {
				r.Slow++
			}
			if x.Reason == "slow" || x.Reason == "stale" || x.Reason == "needs_login" || x.Reason == "blocked" {
				r.Problems++
			}
			if x.Stats.Rotations15 > 0 {
				r.Rotated++
			}
		}
		r.Accounts = append(r.Accounts, x)
	}
	if r.Group.Total > 0 && r.Group.Unknown == 0 {
		r.Group.ETAMinutes = &maxETA
		at := now.Add(time.Duration(maxETA) * time.Minute)
		r.Group.EstimatedAt = &at
	}
	sort.SliceStable(r.Group.Delayers, func(i, j int) bool {
		a, b := r.Group.Delayers[i], r.Group.Delayers[j]
		if a.ETAMinutes == nil {
			return b.ETAMinutes != nil
		}
		if b.ETAMinutes == nil {
			return false
		}
		return *a.ETAMinutes > *b.ETAMinutes
	})
	return r
}
