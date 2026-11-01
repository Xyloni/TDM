package farming

import (
	"fmt"
	"strings"

	"github.com/aalejandrofer/grubdrops/internal/i18n"
)

func (r Report) DigestText(lang string) string {
	text := fmt.Sprintf(i18n.T(lang, "farm.digest.body"), r.Slow, r.Group.Total, r.Rotated, r.Problems)
	if r.Problems == 0 {
		text = i18n.T(lang, "farm.digest.recovered")
	}
	text += "\n" + fmt.Sprintf(i18n.T(lang, "farm.digest.observed"), r.Observed, r.Group.Total)
	var names []string
	for _, a := range r.Accounts {
		if r.Group.ID > 0 && a.Group != r.Group.ID {
			continue
		}
		if a.Reason == "slow" || a.Reason == "needs_login" || a.Reason == "stale" || a.Reason == "blocked" {
			names = append(names, a.Name)
			if len(names) == 5 {
				break
			}
		}
	}
	if len(names) > 0 {
		text += "\n" + strings.Join(names, ", ")
	}
	return text
}

func (r Report) GroupText(lang string) string {
	text := fmt.Sprintf(i18n.T(lang, "farm.group.message"), r.Group.ID, r.Group.Completed, r.Group.Total)
	if r.Group.ID == 0 {
		text = fmt.Sprintf(i18n.T(lang, "farm.group.all_message"), r.Group.Completed, r.Group.Total)
	}
	if r.Group.ETAMinutes != nil {
		text += "\n" + fmt.Sprintf(i18n.T(lang, "farm.group.eta_message"), *r.Group.ETAMinutes)
	} else {
		text += "\n" + fmt.Sprintf(i18n.T(lang, "farm.group.unknown_message"), r.Group.Unknown)
	}
	for i, a := range r.Group.Delayers {
		if i == 5 {
			break
		}
		eta := i18n.T(lang, "farm.unknown")
		if a.ETAMinutes != nil {
			eta = fmt.Sprintf(i18n.T(lang, "farm.minutes_value"), *a.ETAMinutes)
		}
		text += "\n" + a.Name + ": " + eta
	}
	return text
}
