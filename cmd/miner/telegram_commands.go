package main

import (
	"context"
	"time"

	"github.com/aalejandrofer/grubdrops/internal/i18n"
	"github.com/aalejandrofer/grubdrops/internal/notify"
)

// A single polling loop avoids competing getUpdates consumers inside one daemon.
// Mining and notification delivery do not depend on incoming commands.
func (ro *roster) runTelegramCommands(configured func() *notify.TelegramBot) {
	var bot *notify.TelegramBot
	var username string
	var offset int64
	var lastError string
	wait := func() bool {
		timer := time.NewTimer(5 * time.Second)
		defer timer.Stop()
		select {
		case <-ro.ctx.Done():
			return false
		case <-timer.C:
			return true
		}
	}
	for ro.ctx.Err() == nil {
		next := configured()
		if next == nil {
			bot = nil
			if !wait() {
				return
			}
			continue
		}
		if bot == nil || bot.Token != next.Token || bot.ChatID != next.ChatID {
			bot = next
			username = ""
			offset = 0
			lastError = ""
		}
		ctx, cancel := context.WithTimeout(ro.ctx, 12*time.Second)
		var err error
		if username == "" {
			username, err = bot.Username(ctx)
		}
		if err == nil {
			var updates []notify.TelegramUpdate
			updates, err = bot.Updates(ctx, offset)
			if err == nil {
				for _, u := range updates {
					offset = max(offset, u.ID+1)
					m, command := bot.Command(u, username, time.Now())
					if m == nil {
						continue
					}
					text := ro.telegramCommandText(command)
					replyCtx, replyCancel := context.WithTimeout(ro.ctx, 10*time.Second)
					if sendErr := bot.Reply(replyCtx, m, text); sendErr != nil {
						ro.log().Warn("Telegram command reply failed", "err", sendErr)
					}
					replyCancel()
				}
			}
		}
		cancel()
		if err != nil {
			if ro.ctx.Err() != nil {
				return
			}
			if err.Error() != lastError {
				ro.log().Warn("Telegram command polling failed; check webhook or another polling instance", "err", err)
				lastError = err.Error()
			}
			if !wait() {
				return
			}
		} else {
			lastError = ""
		}
	}
}

func (ro *roster) telegramCommandText(command string) string {
	lang := ro.uiLanguage()
	if command == "/start" || command == "/help" {
		return i18n.T(lang, "farm.commands.help")
	}
	if command != "/status" && command != "/groups" && command != "/lagging" {
		return i18n.T(lang, "farm.commands.unknown")
	}
	report, err := ro.farmReport()
	if err != nil {
		return i18n.T(lang, "farm.load_error")
	}
	if command == "/groups" {
		return report.GroupText(lang)
	}
	if command == "/lagging" {
		return report.DigestText(lang)
	}
	return report.DigestText(lang) + "\n\n" + report.GroupText(lang)
}
