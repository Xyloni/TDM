package notify

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
)

// updateDigest edits one durable message. Healthy fleets stay silent until
// the first incident; recovery edits the existing message instead of posting.
func (t *TelegramBot) updateDigest(ctx context.Context, text string, active bool) error {
	t.digestMu.Lock()
	defer t.digestMu.Unlock()
	if !t.digestLoaded {
		if t.LoadDigest != nil {
			id, err := t.LoadDigest(ctx)
			if err != nil {
				return err
			}
			t.digestID = id
		}
		t.digestLoaded = true
	}
	if t.digestDirty && t.SaveDigest != nil {
		if err := t.SaveDigest(ctx, t.digestID); err != nil {
			return err
		}
		t.digestDirty = false
	}
	if t.digestID == 0 && !active {
		return nil
	}
	if t.digestText == text {
		return nil
	}
	if t.digestID > 0 {
		err := t.send(ctx, "editMessageText", map[string]any{"chat_id": t.ChatID, "message_id": t.digestID, "text": text, "parse_mode": "HTML", "disable_web_page_preview": true})
		if err == nil {
			t.digestText = text
			return nil
		}
		var apiErr *telegramAPIError
		if !errors.As(err, &apiErr) || apiErr.status != 400 {
			return err
		}
		desc := strings.ToLower(apiErr.desc)
		if strings.Contains(desc, "message is not modified") {
			t.digestText = text
			return nil
		}
		// Only confirmed deletion permits a replacement; permission/network
		// failures must not produce a new message on every retry.
		if !strings.Contains(desc, "message to edit not found") {
			return err
		}
		t.digestID = 0
		if t.SaveDigest != nil {
			if err = t.SaveDigest(ctx, 0); err != nil {
				return err
			}
		}
		if !active {
			return nil
		}
	}
	resp, err := t.call(ctx, "sendMessage", map[string]any{"chat_id": t.ChatID, "text": text, "parse_mode": "HTML", "disable_web_page_preview": true})
	if err != nil {
		return err
	}
	var message struct {
		ID int64 `json:"message_id"`
	}
	if err = json.Unmarshal(resp.Result, &message); err != nil || message.ID <= 0 {
		return errors.New("telegram digest: missing message_id")
	}
	t.digestID = message.ID
	t.digestText = text
	if t.SaveDigest != nil {
		t.digestDirty = true
		if err := t.SaveDigest(ctx, t.digestID); err != nil {
			return err
		}
		t.digestDirty = false
	}
	return nil
}
