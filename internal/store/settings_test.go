package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aalejandrofer/grubdrops/internal/store/gen"
)

func TestSettings_RoundTrip(t *testing.T) {
	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "t.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	s := NewSettings(gen.New(db))

	url, err := s.GlobalDiscordWebhook(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "", url)

	require.NoError(t, s.SetGlobalDiscordWebhook(context.Background(), "https://discord.example/wh/abc"))
	url, err = s.GlobalDiscordWebhook(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "https://discord.example/wh/abc", url)
}

func TestSettings_AutoSkipUnlinkedDefaultsOnAndPersists(t *testing.T) {
	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "t.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	s := NewSettings(gen.New(db))
	ctx := context.Background()
	on, err := s.AutoSkipUnlinked(ctx)
	require.NoError(t, err)
	assert.True(t, on)
	require.NoError(t, s.SetAutoSkipUnlinked(ctx, false))
	on, err = s.AutoSkipUnlinked(ctx)
	require.NoError(t, err)
	assert.False(t, on)
}

func TestSettings_Canary(t *testing.T) {
	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "t.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	s := NewSettings(gen.New(db))
	ctx := context.Background()

	tw, err := s.CanaryTwitchChannel(ctx)
	require.NoError(t, err)
	assert.Equal(t, "alveussanctuary", tw)

	kk, err := s.CanaryKickChannel(ctx)
	require.NoError(t, err)
	assert.Equal(t, "", kk)

	iv, err := s.CanaryIntervalSec(ctx)
	require.NoError(t, err)
	assert.Equal(t, 6*3600, iv)

	require.NoError(t, s.SetCanaryTwitchChannel(ctx, "somechannel"))
	tw, _ = s.CanaryTwitchChannel(ctx)
	assert.Equal(t, "somechannel", tw)

	require.NoError(t, s.SetCanaryKickChannel(ctx, "kickchan"))
	kk, _ = s.CanaryKickChannel(ctx)
	assert.Equal(t, "kickchan", kk)

	require.NoError(t, s.SetCanaryIntervalSec(ctx, 1800))
	iv, _ = s.CanaryIntervalSec(ctx)
	assert.Equal(t, 1800, iv)

	// Explicit 0 disables the canary (Runner.Run treats <=0 as disabled).
	require.NoError(t, s.SetCanaryIntervalSec(ctx, 0))
	iv, err = s.CanaryIntervalSec(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, iv, "explicit 0 must disable (not default to 6h)")
}

func TestSettings_AccountGroupQueue(t *testing.T) {
	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "groups.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	s := NewSettings(gen.New(db))
	ctx := context.Background()

	state, err := s.AccountGroupsQueue(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, state.Active)
	require.NoError(t, s.SetAccountGroup(ctx, "a1", 1))
	require.NoError(t, s.SetAccountGroup(ctx, "a2", 2))
	state, err = s.AccountGroupsQueue(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, state.Active)
	assert.Equal(t, map[string]int{"a1": 1, "a2": 2}, state.Groups)

	set, err := s.SetAccountGroupCampaign(ctx, 1, "campaign")
	require.NoError(t, err)
	assert.True(t, set)
	next, advanced, err := s.AdvanceAccountGroup(ctx, 1, "campaign")
	require.NoError(t, err)
	assert.True(t, advanced)
	assert.Equal(t, 2, next.Active)
	assert.Equal(t, "campaign", next.CampaignID)

	next, advanced, err = s.AdvanceAccountGroup(ctx, 2, "campaign")
	require.NoError(t, err)
	assert.True(t, advanced)
	assert.Equal(t, 1, next.Active)
	assert.Empty(t, next.CampaignID)

	require.NoError(t, s.SetAccountGroup(ctx, "a1", 0))
	state, err = s.AccountGroupsQueue(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, state.Active)
}

func TestSettings_KickWatchMode(t *testing.T) {
	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "t.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	s := NewSettings(gen.New(db))
	ctx := context.Background()

	// Default is auto (WS first, Chrome fallback): WS needs no Docker so a fresh
	// install mines Kick on any Pi out of the box, and auto falls back to the
	// Chrome sidecar if WS stops accruing.
	mode, err := s.KickWatchMode(ctx)
	require.NoError(t, err)
	assert.Equal(t, KickWatchModeAuto, mode)

	// Round-trips the explicit browser path.
	require.NoError(t, s.SetKickWatchMode(ctx, KickWatchModeBrowser))
	mode, err = s.KickWatchMode(ctx)
	require.NoError(t, err)
	assert.Equal(t, KickWatchModeBrowser, mode)

	// Round-trips the experimental WS-only path.
	require.NoError(t, s.SetKickWatchMode(ctx, KickWatchModeWS))
	mode, err = s.KickWatchMode(ctx)
	require.NoError(t, err)
	assert.Equal(t, KickWatchModeWS, mode)

	// Unknown values are coerced back to the auto default on write…
	require.NoError(t, s.SetKickWatchMode(ctx, "garbage"))
	mode, err = s.KickWatchMode(ctx)
	require.NoError(t, err)
	assert.Equal(t, KickWatchModeAuto, mode)
}

func TestSettings_UIDefaults(t *testing.T) {
	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "t.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	s := NewSettings(gen.New(db))
	ctx := context.Background()

	lang, _ := s.UILanguage(ctx)
	tf, _ := s.UITimeFormat(ctx)
	th, _ := s.UITheme(ctx)
	sum, _ := s.NotifyDailySummary(ctx)
	auto, _ := s.UpdateAuto(ctx)
	assert.Equal(t, "ru", lang)
	assert.Equal(t, "24h", tf)
	assert.Equal(t, "dark", th)
	assert.False(t, sum)
	assert.True(t, auto)

	require.NoError(t, s.SetUILanguage(ctx, "zh-CN"))
	require.NoError(t, s.SetUITimeFormat(ctx, "12h"))
	require.NoError(t, s.SetUITheme(ctx, "light"))
	require.NoError(t, s.SetNotifyDailySummary(ctx, true))
	require.NoError(t, s.SetUpdateAuto(ctx, false))
	lang, _ = s.UILanguage(ctx)
	tf, _ = s.UITimeFormat(ctx)
	th, _ = s.UITheme(ctx)
	sum, _ = s.NotifyDailySummary(ctx)
	auto, _ = s.UpdateAuto(ctx)
	assert.Equal(t, "zh-CN", lang)
	assert.Equal(t, "12h", tf)
	assert.Equal(t, "light", th)
	assert.True(t, sum)
	assert.False(t, auto)

	assert.Error(t, s.SetUILanguage(ctx, "fr"))
	assert.Error(t, s.SetUITheme(ctx, "blue"))
}

func TestSettings_ScanAndPreempt(t *testing.T) {
	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "t.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	s := NewSettings(gen.New(db))
	ctx := context.Background()

	n, _ := s.ScanIntervalMin(ctx)
	on, _ := s.Preempt(ctx)
	assert.Equal(t, 60, n)
	assert.True(t, on)

	require.NoError(t, s.SetScanIntervalMin(ctx, 15))
	require.NoError(t, s.SetPreempt(ctx, false))
	n, _ = s.ScanIntervalMin(ctx)
	on, _ = s.Preempt(ctx)
	assert.Equal(t, 15, n)
	assert.False(t, on)

	assert.Error(t, s.SetScanIntervalMin(ctx, 5))
	require.NoError(t, s.setString(ctx, keyScanIntervalMin, "7"))
	n, _ = s.ScanIntervalMin(ctx)
	assert.Equal(t, 60, n)
}

func TestSettings_AccountGroupWaveSize(t *testing.T) {
	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "t.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	s := NewSettings(gen.New(db))
	ctx := context.Background()
	n, err := s.AccountGroupWaveSize(ctx)
	require.NoError(t, err)
	assert.Equal(t, 20, n)
	require.NoError(t, s.SetAccountGroupWaveSize(ctx, 35))
	n, err = s.AccountGroupWaveSize(ctx)
	require.NoError(t, err)
	assert.Equal(t, 35, n)
	assert.Error(t, s.SetAccountGroupWaveSize(ctx, 0))
}
