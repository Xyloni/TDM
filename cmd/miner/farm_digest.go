package main

import (
	"context"
	"errors"
	"time"

	"github.com/aalejandrofer/grubdrops/internal/farming"
	"github.com/aalejandrofer/grubdrops/internal/i18n"
	"github.com/aalejandrofer/grubdrops/internal/notify"
	"github.com/aalejandrofer/grubdrops/internal/store"
)

func (ro *roster) farmReport() (farming.Report, error) {
	accounts, err := ro.q.ListAllAccounts(ro.ctx)
	if err != nil {
		return farming.Report{}, err
	}
	queue := store.AccountGroupQueue{}
	if ro.settings != nil {
		queue, err = ro.settings.AccountGroupsQueue(ro.ctx)
		if err != nil {
			return farming.Report{}, err
		}
	}
	return farming.Build(time.Now(), accounts, ro.pool.Snapshots(), queue), nil
}

func (ro *roster) runFarmDigest() {
	// Let imports settle and accumulate a full window before the first digest.
	timer := time.NewTimer(15 * time.Minute)
	defer timer.Stop()
	select {
	case <-ro.ctx.Done():
		return
	case <-timer.C:
	}
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		ro.sendFarmDigest()
		select {
		case <-ro.ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (ro *roster) sendFarmDigest() {
	if ro.notifier == nil {
		return
	}
	report, err := ro.farmReport()
	if err != nil {
		ro.log().Warn("farm digest snapshot failed", "err", err)
		return
	}
	lang := ro.uiLanguage()
	ctx, cancel := context.WithTimeout(ro.ctx, 25*time.Second)
	defer cancel()
	err = ro.notifier.Notify(notify.WithTarget(ctx, notify.TargetTelegram), notify.EventFarmProblems, map[string]any{
		"title": i18n.T(lang, "farm.digest.title"), "msg": report.DigestText(lang), "active": report.Problems > 0,
	})
	if err != nil && !errors.Is(err, notify.ErrNoTarget) {
		ro.log().Warn("farm Telegram digest failed", "err", err)
	}
}
