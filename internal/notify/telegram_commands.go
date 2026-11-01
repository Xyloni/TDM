package notify

import (
	"context"
	"encoding/json"
	"html"
	"strconv"
	"strings"
	"time"
)

type TelegramUpdate struct {
	ID          int64            `json:"update_id"`
	Message     *TelegramMessage `json:"message"`
	ChannelPost *TelegramMessage `json:"channel_post"`
}
type TelegramMessage struct {
	ID   int64  `json:"message_id"`
	Date int64  `json:"date"`
	Text string `json:"text"`
	Chat struct {
		ID       int64  `json:"id"`
		Username string `json:"username"`
	} `json:"chat"`
	From struct {
		IsBot bool `json:"is_bot"`
	} `json:"from"`
	Entities []struct {
		Type   string `json:"type"`
		Offset int    `json:"offset"`
	} `json:"entities"`
}

func (t *TelegramBot) Username(ctx context.Context) (string, error) {
	resp, err := t.call(ctx, "getMe", map[string]any{})
	if err != nil {
		return "", err
	}
	var me struct {
		Username string `json:"username"`
	}
	err = json.Unmarshal(resp.Result, &me)
	return me.Username, err
}

func (t *TelegramBot) Updates(ctx context.Context, offset int64) ([]TelegramUpdate, error) {
	resp, err := t.call(ctx, "getUpdates", map[string]any{"offset": offset, "timeout": 5, "limit": 5, "allowed_updates": []string{"message", "channel_post"}})
	if err != nil {
		return nil, err
	}
	var updates []TelegramUpdate
	err = json.Unmarshal(resp.Result, &updates)
	return updates, err
}

// Command accepts explicit slash commands in the configured destination only.
// Private strangers, other bots, stale backlog and @OtherBot commands are silent.
func (t *TelegramBot) Command(u TelegramUpdate, username string, now time.Time) (*TelegramMessage, string) {
	m := u.Message
	if m == nil {
		m = u.ChannelPost
	}
	if m == nil || m.From.IsBot {
		return nil, ""
	}
	allowed := t.ChatID == strconv.FormatInt(m.Chat.ID, 10)
	if strings.HasPrefix(t.ChatID, "@") {
		allowed = strings.EqualFold(strings.TrimPrefix(t.ChatID, "@"), m.Chat.Username)
	}
	if !allowed || now.Unix()-m.Date > 300 || m.Date > now.Unix()+60 {
		return nil, ""
	}
	entity := false
	for _, e := range m.Entities {
		if e.Type == "bot_command" && e.Offset == 0 {
			entity = true
			break
		}
	}
	if !entity {
		return nil, ""
	}
	fields := strings.Fields(m.Text)
	if len(fields) == 0 {
		return nil, ""
	}
	name, target, tagged := strings.Cut(strings.ToLower(fields[0]), "@")
	if tagged && !strings.EqualFold(target, username) {
		return nil, ""
	}
	return m, name
}

func (t *TelegramBot) Reply(ctx context.Context, m *TelegramMessage, text string) error {
	return t.send(ctx, "sendMessage", map[string]any{"chat_id": m.Chat.ID, "text": html.EscapeString(text), "disable_web_page_preview": true, "parse_mode": "HTML"})
}
