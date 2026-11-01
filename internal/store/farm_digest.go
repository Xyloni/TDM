package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// FarmDigestMessage isolates the editable message by bot and destination.
// Only a hash of the configuration is stored, never a second copy of the token.
func (s *Settings) FarmDigestMessage(ctx context.Context, token, chat string) (int64, error) {
	raw, err := s.getString(ctx, "settings:farm_digest_message")
	if err != nil || raw == "" {
		return 0, err
	}
	var v struct {
		Scope string
		ID    int64
	}
	if err = json.Unmarshal([]byte(raw), &v); err != nil {
		return 0, err
	}
	if v.Scope != digestScope(token, chat) {
		return 0, nil
	}
	return v.ID, nil
}

func (s *Settings) SetFarmDigestMessage(ctx context.Context, token, chat string, id int64) error {
	v := struct {
		Scope string
		ID    int64
	}{digestScope(token, chat), id}
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return s.setString(ctx, "settings:farm_digest_message", string(raw))
}

func digestScope(token, chat string) string {
	h := sha256.Sum256([]byte(token + "\x00" + chat))
	return hex.EncodeToString(h[:])
}

func (s *Settings) TelegramCommands(ctx context.Context) (bool, error) {
	raw, err := s.getString(ctx, "settings:telegram_commands")
	return raw != "0", err
}
func (s *Settings) SetTelegramCommands(ctx context.Context, on bool) error {
	return s.setBool(ctx, "settings:telegram_commands", on)
}
