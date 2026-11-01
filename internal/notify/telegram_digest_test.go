package notify

import (
	"context"
	"errors"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestTelegramDigestEditsOneMessageAndRestoresItsID(t *testing.T) {
	srv, calls := tgServer(t, func(_ int, w http.ResponseWriter) { _, _ = io.WriteString(w, `{"ok":true,"result":{"message_id":42}}`) })
	bot := newTestBot(srv.URL, nil)
	var saved int64
	bot.SaveDigest = func(_ context.Context, id int64) error { saved = id; return nil }
	ctx := context.Background()
	fields := map[string]any{"title": "Health", "msg": "Healthy", "active": false}
	require.NoError(t, bot.Notify(ctx, EventFarmProblems, fields))
	require.Empty(t, calls())
	fields["msg"] = "Slow <account>"
	fields["active"] = true
	require.NoError(t, bot.Notify(ctx, EventFarmProblems, fields))
	require.Equal(t, int64(42), saved)
	require.NoError(t, bot.Notify(ctx, EventFarmProblems, fields))
	require.Len(t, calls(), 1, "identical text must not cause another request")
	fields["active"] = false
	fields["msg"] = "Recovered"
	require.NoError(t, bot.Notify(ctx, EventFarmProblems, fields))
	require.True(t, strings.HasSuffix(calls()[1].path, "/editMessageText"))
	newBot := newTestBot(srv.URL, nil)
	newBot.LoadDigest = func(context.Context) (int64, error) { return saved, nil }
	require.NoError(t, newBot.Notify(ctx, EventFarmProblems, fields))
	require.True(t, strings.HasSuffix(calls()[2].path, "/editMessageText"))
	require.Equal(t, float64(42), calls()[2].body["message_id"])
	require.Contains(t, calls()[0].body["text"], "&lt;account&gt;")
}

func TestTelegramDigestDoesNotDuplicateOnEditFailure(t *testing.T) {
	srv, calls := tgServer(t, func(_ int, w http.ResponseWriter) {
		w.WriteHeader(403)
		_, _ = io.WriteString(w, `{"ok":false,"description":"forbidden"}`)
	})
	bot := newTestBot(srv.URL, nil)
	bot.LoadDigest = func(context.Context) (int64, error) { return 42, nil }
	require.Error(t, bot.Notify(context.Background(), EventFarmProblems, map[string]any{"title": "Health", "active": true}))
	require.Len(t, calls(), 1)
	require.True(t, strings.HasSuffix(calls()[0].path, "/editMessageText"))
}

func TestTelegramCommandsRestrictedToConfiguredChatAndBot(t *testing.T) {
	bot := NewTelegramBot("test", "123", nil)
	now := time.Now()
	u := TelegramUpdate{Message: &TelegramMessage{Text: "/status@PumpBot", Date: now.Unix()}}
	u.Message.Chat.ID = 123
	u.Message.Entities = append(u.Message.Entities, struct {
		Type   string `json:"type"`
		Offset int    `json:"offset"`
	}{"bot_command", 0})
	m, cmd := bot.Command(u, "PumpBot", now)
	require.NotNil(t, m)
	require.Equal(t, "/status", cmd)
	u.Message.Chat.ID = 456
	m, _ = bot.Command(u, "PumpBot", now)
	require.Nil(t, m)
	u.Message.Chat.ID = 123
	m, _ = bot.Command(u, "OtherBot", now)
	require.Nil(t, m)
	u.Message.Date = now.Add(-10 * time.Minute).Unix()
	m, _ = bot.Command(u, "PumpBot", now)
	require.Nil(t, m)
	u.Message.Date = now.Unix()
	u.Message.From.IsBot = true
	m, _ = bot.Command(u, "PumpBot", now)
	require.Nil(t, m)
}

func TestTelegramDigestRetriesPersistenceWithoutResending(t *testing.T) {
	srv, calls := tgServer(t, func(_ int, w http.ResponseWriter) { _, _ = io.WriteString(w, `{"ok":true,"result":{"message_id":42}}`) })
	bot := newTestBot(srv.URL, nil)
	saves := 0
	bot.SaveDigest = func(context.Context, int64) error {
		saves++
		if saves == 1 {
			return errors.New("temporary DB error")
		}
		return nil
	}
	fields := map[string]any{"title": "Health", "active": true}
	require.Error(t, bot.Notify(context.Background(), EventFarmProblems, fields))
	require.NoError(t, bot.Notify(context.Background(), EventFarmProblems, fields))
	require.Len(t, calls(), 1)
	require.Equal(t, 2, saves)
}

func TestTelegramDigestReplacesDeletedMessageOnce(t *testing.T) {
	srv, calls := tgServer(t, func(n int, w http.ResponseWriter) {
		if n == 1 {
			w.WriteHeader(400)
			_, _ = io.WriteString(w, `{"ok":false,"description":"Bad Request: message to edit not found"}`)
			return
		}
		_, _ = io.WriteString(w, `{"ok":true,"result":{"message_id":43}}`)
	})
	bot := newTestBot(srv.URL, nil)
	bot.LoadDigest = func(context.Context) (int64, error) { return 42, nil }
	fields := map[string]any{"title": "Health", "active": true}
	require.NoError(t, bot.Notify(context.Background(), EventFarmProblems, fields))
	require.Len(t, calls(), 2)
	require.True(t, strings.HasSuffix(calls()[1].path, "/sendMessage"))
	require.NoError(t, bot.Notify(context.Background(), EventFarmProblems, fields))
	require.Len(t, calls(), 2)
}

func TestTelegramCommandTransportReadsUpdatesAndEscapesReplies(t *testing.T) {
	srv, calls := tgServer(t, func(n int, w http.ResponseWriter) {
		if n == 1 {
			_, _ = io.WriteString(w, `{"ok":true,"result":[{"update_id":123,"message":{"message_id":5,"date":0,"text":"/help","chat":{"id":123}}}]}`)
			return
		}
		_, _ = io.WriteString(w, `{"ok":true}`)
	})
	bot := newTestBot(srv.URL, nil)
	u, err := bot.Updates(context.Background(), 122)
	require.NoError(t, err)
	require.Len(t, u, 1)
	require.Equal(t, int64(123), u[0].ID)
	require.NoError(t, bot.Reply(context.Background(), u[0].Message, "Account <test>"))
	require.Equal(t, float64(122), calls()[0].body["offset"])
	require.Equal(t, "Account &lt;test&gt;", calls()[1].body["text"])
}
