package notify

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testToken = "123456:ABCdefGHIjklMNOpqrSTUvwxYZ0123456789"

type tgCall struct {
	path string
	body map[string]any
}

func tgServer(t *testing.T, handle func(n int, w http.ResponseWriter)) (*httptest.Server, func() []tgCall) {
	t.Helper()
	var mu sync.Mutex
	var calls []tgCall
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		require.NoError(t, json.Unmarshal(raw, &body))
		mu.Lock()
		calls = append(calls, tgCall{path: r.URL.Path, body: body})
		n := len(calls)
		mu.Unlock()
		handle(n, w)
	}))
	t.Cleanup(srv.Close)
	return srv, func() []tgCall {
		mu.Lock()
		defer mu.Unlock()
		return append([]tgCall(nil), calls...)
	}
}

func okHandler(_ int, w http.ResponseWriter) { _, _ = io.WriteString(w, `{"ok":true}`) }

func newTestBot(base string, filter *VerbosityFilter) *TelegramBot {
	b := NewTelegramBot(testToken, "-1001234567890", filter)
	b.APIBase = base
	return b
}

func TestTelegram_SendMessageFormat(t *testing.T) {
	srv, calls := tgServer(t, okHandler)
	bot := newTestBot(srv.URL, nil)
	require.NoError(t, bot.Notify(context.Background(), EventClaim, map[string]any{
		"platform": "twitch", "game": "Rust", "drop": "Helmet", "campaign": "Week 1",
		"account_label": "main", "channel": "streamer", "cur_min": 30, "req_min": 60,
	}))
	got := calls()
	require.Len(t, got, 1)
	assert.Equal(t, "/bot"+testToken+"/sendMessage", got[0].path)
	b := got[0].body
	assert.Equal(t, "-1001234567890", b["chat_id"])
	assert.Equal(t, "HTML", b["parse_mode"])
	assert.Equal(t, true, b["disable_web_page_preview"])
	assert.Equal(t, strings.Join([]string{
		"<b>Helmet</b>",
		"Rust · Twitch",
		"Account: main",
		`Channel: <a href="https://twitch.tv/streamer">streamer</a>`,
		"✅ Claimed <code>60/60 min</code>",
		"▰▰▰▰▰▰▰▰▰▰",
		"<i>GrubDrops • Week 1</i>",
	}, "\n"), b["text"])
}

func TestTelegramGroupSwitchSendsAndPinsOnlyItsOwnMessage(t *testing.T) {
	srv, calls := tgServer(t, func(n int, w http.ResponseWriter) {
		if n == 1 {
			_, _ = io.WriteString(w, `{"ok":true,"result":{"message_id":456}}`)
			return
		}
		_, _ = io.WriteString(w, `{"ok":true,"result":true}`)
	})
	bot := newTestBot(srv.URL, &VerbosityFilter{Allow: map[string]bool{EventGroupSwitch: true}})
	require.NoError(t, bot.Notify(context.Background(), EventGroupSwitch, map[string]any{
		"title": "Смена группы", "msg": "Группа 1 завершила <кампанию>. Группа 2 начала фарм.",
	}))
	got := calls()
	require.Len(t, got, 2)
	assert.True(t, strings.HasSuffix(got[0].path, "/sendMessage"))
	assert.Equal(t, "<b>Смена группы</b>\nГруппа 1 завершила &lt;кампанию&gt;. Группа 2 начала фарм.", got[0].body["text"])
	assert.True(t, strings.HasSuffix(got[1].path, "/pinChatMessage"))
	assert.Equal(t, got[0].body["chat_id"], got[1].body["chat_id"])
	assert.Equal(t, float64(456), got[1].body["message_id"])
	assert.Equal(t, true, got[1].body["disable_notification"])
}

func TestTelegramGroupPinFailureDoesNotResendMessage(t *testing.T) {
	srv, calls := tgServer(t, func(n int, w http.ResponseWriter) {
		if n == 1 {
			_, _ = io.WriteString(w, `{"ok":true,"result":{"message_id":789}}`)
			return
		}
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, `{"ok":false,"description":"not enough rights to pin"}`)
	})
	err := newTestBot(srv.URL, nil).Notify(context.Background(), EventGroupSwitch, map[string]any{"title": "Group changed"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pinChatMessage")
	assert.NotContains(t, err.Error(), testToken)
	assert.Len(t, calls(), 2, "pin failure must not resend the delivered notification")
}

func TestTelegramGroupSwitchDoesNotPinMissingMessageID(t *testing.T) {
	srv, calls := tgServer(t, okHandler)
	err := newTestBot(srv.URL, nil).Notify(context.Background(), EventGroupSwitch, nil)
	require.ErrorContains(t, err, "message_id")
	assert.Len(t, calls(), 1)
}

func TestTelegram_EscapesOutsideText(t *testing.T) {
	srv, calls := tgServer(t, okHandler)
	bot := newTestBot(srv.URL, nil)
	require.NoError(t, bot.Notify(context.Background(), EventProgress, map[string]any{
		"platform": "kick", "game": `<script>&"x"`, "drop": "<b>Bold</b>",
		"campaign": "A & B", "channel": `a"b<c>`, "msg": "1 < 2",
	}))
	text := calls()[0].body["text"].(string)
	assert.Contains(t, text, "<b>&lt;b&gt;Bold&lt;/b&gt;</b>")
	assert.Contains(t, text, "&lt;script&gt;&amp;&#34;x&#34; · Kick")
	assert.Contains(t, text, `<a href="https://kick.com/a%22b%3Cc%3E">a&#34;b&lt;c&gt;</a>`)
	assert.Contains(t, text, "Detail: 1 &lt; 2")
	assert.Contains(t, text, "<i>GrubDrops • A &amp; B</i>")
	assert.NotContains(t, text, "<script>")
}

func TestTelegram_SummaryAndFallback(t *testing.T) {
	srv, calls := tgServer(t, okHandler)
	bot := newTestBot(srv.URL, nil)
	ctx := context.Background()
	require.NoError(t, bot.Notify(ctx, EventSummary, map[string]any{"title": "Daily <summary>", "msg": "60 min"}))
	require.NoError(t, bot.Notify(ctx, EventCanary, map[string]any{"platform": "kick", "detail": "no <credit>"}))
	got := calls()
	require.Len(t, got, 2)
	assert.Equal(t, "<b>Daily &lt;summary&gt;</b>\n60 min", got[0].body["text"])
	assert.Equal(t, "<b>Heartbeat health check failed</b>\ndetail: no &lt;credit&gt;\nplatform: kick", got[1].body["text"])
}

func TestTelegram_FilterBlocks(t *testing.T) {
	srv, calls := tgServer(t, okHandler)
	bot := newTestBot(srv.URL, &VerbosityFilter{Allow: map[string]bool{EventClaim: true}})
	require.NoError(t, bot.Notify(context.Background(), EventProgress, map[string]any{"drop": "x"}))
	assert.Empty(t, calls())
}

func TestTelegram_PhotoWithCaption(t *testing.T) {
	srv, calls := tgServer(t, okHandler)
	bot := newTestBot(srv.URL, nil)
	require.NoError(t, bot.Notify(context.Background(), EventClaim, map[string]any{
		"drop": "Helmet", "image": "https://example.com/i.png",
	}))
	got := calls()
	require.Len(t, got, 1)
	assert.Equal(t, "/bot"+testToken+"/sendPhoto", got[0].path)
	assert.Equal(t, "https://example.com/i.png", got[0].body["photo"])
	assert.Contains(t, got[0].body["caption"], "<b>Helmet</b>")
}

func TestTelegram_PhotoRejectedFallsBackToText(t *testing.T) {
	srv, calls := tgServer(t, func(n int, w http.ResponseWriter) {
		if n == 1 {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, `{"ok":false,"description":"Bad Request: wrong file"}`)
			return
		}
		okHandler(n, w)
	})
	bot := newTestBot(srv.URL, nil)
	require.NoError(t, bot.Notify(context.Background(), EventClaim, map[string]any{
		"drop": "Helmet", "image": "https://example.com/i.png",
	}))
	got := calls()
	require.Len(t, got, 2)
	assert.True(t, strings.HasSuffix(got[1].path, "/sendMessage"))
}

func TestTelegram_429RetriesAfter(t *testing.T) {
	srv, calls := tgServer(t, func(n int, w http.ResponseWriter) {
		if n == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = io.WriteString(w, `{"ok":false,"description":"Too Many Requests: retry after 1","parameters":{"retry_after":1}}`)
			return
		}
		okHandler(n, w)
	})
	bot := newTestBot(srv.URL, nil)
	require.NoError(t, bot.Notify(context.Background(), EventTest, map[string]any{"drop": "x"}))
	assert.Len(t, calls(), 2)
}

func TestTelegram_429TooLongFails(t *testing.T) {
	srv, calls := tgServer(t, func(_ int, w http.ResponseWriter) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = io.WriteString(w, `{"ok":false,"parameters":{"retry_after":3600}}`)
	})
	bot := newTestBot(srv.URL, nil)
	err := bot.Notify(context.Background(), EventTest, map[string]any{"drop": "x"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "429")
	assert.Len(t, calls(), 1)
}

func TestTelegram_ErrorsRedactToken(t *testing.T) {
	// A closed server makes client.Do fail with a *url.Error holding the URL.
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	base := srv.URL
	srv.Close()
	bot := newTestBot(base, nil)
	err := bot.Notify(context.Background(), EventTest, map[string]any{"drop": "x"})
	require.Error(t, err)
	assert.NotContains(t, err.Error(), testToken)
	assert.NotContains(t, err.Error(), "ABCdefGHI")
	assert.Contains(t, err.Error(), "REDACTED")

	srv2, _ := tgServer(t, func(_ int, w http.ResponseWriter) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"ok":false,"description":"Unauthorized"}`)
	})
	bot = newTestBot(srv2.URL, nil)
	err = bot.Notify(context.Background(), EventTest, map[string]any{"drop": "x"})
	require.Error(t, err)
	assert.NotContains(t, err.Error(), testToken)
	var apiErr *telegramAPIError
	require.True(t, errors.As(err, &apiErr))
	assert.Equal(t, http.StatusUnauthorized, apiErr.status)
}
