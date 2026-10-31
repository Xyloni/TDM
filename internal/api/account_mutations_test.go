package api

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aalejandrofer/grubdrops/internal/platform"
	"github.com/aalejandrofer/grubdrops/internal/platform/platformtest"
	"github.com/stretchr/testify/require"
)

type maintenancePool struct {
	snapPool
	paused  atomic.Bool
	resumes atomic.Int32
}

func (p *maintenancePool) Pause() func() {
	p.paused.Store(true)
	return func() { p.paused.Store(false); p.resumes.Add(1) }
}

type parallelAndroid struct {
	*androidBackend
	pool    *maintenancePool
	entered chan struct{}
	release chan struct{}
}

func (b *parallelAndroid) AndroidLogin(ctx context.Context, raw string) (platform.Session, string, error) {
	if !b.pool.paused.Load() {
		panic("login while miners are running")
	}
	b.entered <- struct{}{}
	select {
	case <-b.release:
	case <-ctx.Done():
		return platform.Session{}, "", ctx.Err()
	}
	return b.androidBackend.AndroidLogin(ctx, raw)
}

func TestBatchLoginParallelDeduplicatedAndPaused(t *testing.T) {
	p := &maintenancePool{}
	b := &parallelAndroid{androidBackend: &androidBackend{Fake: platformtest.NewFake()}, pool: p,
		entered: make(chan struct{}, 4), release: make(chan struct{})}
	e, _ := accountsEnv(t, b)
	e.s.d.Pool = p
	_, csrf := e.session()
	done := make(chan struct{})
	go func() {
		defer close(done)
		resp, body := e.do("POST", "/api/login/twitch/batch", csrf, map[string]any{
			"accounts": []map[string]string{{"url": "good"}, {"url": "other"}, {"url": "dead"}, {"url": "good"}},
		})
		if resp.StatusCode != 200 {
			t.Errorf("status %d: %s", resp.StatusCode, body)
			return
		}
		var out struct {
			Results []bool `json:"results"`
		}
		if err := json.Unmarshal(body, &out); err != nil {
			t.Error(err)
			return
		}
		if len(out.Results) != 4 || !out.Results[0] || !out.Results[1] || out.Results[2] || !out.Results[3] {
			t.Errorf("results: %v", out.Results)
		}
	}()
	// All validations must enter before any is released: this cannot pass
	// with sequential imports, and also checks the maintenance boundary.
	for i := 0; i < 4; i++ {
		select {
		case <-b.entered:
		case <-time.After(5 * time.Second):
			close(b.release)
			t.Fatal("validation did not run concurrently")
		}
	}
	require.True(t, p.paused.Load())
	close(b.release)
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("batch hung")
	}
	require.False(t, p.paused.Load())
	require.EqualValues(t, 1, p.resumes.Load())
	accounts, err := e.s.d.Q.ListAllAccounts(context.Background())
	require.NoError(t, err)
	require.Len(t, accounts, 2, "duplicate Twitch user must not create another account")
}

func TestDeleteAllAccountsTransactionAndChildren(t *testing.T) {
	e, _ := accountsEnv(t)
	a := e.createAccount("twitch", "a")
	e.createAccount("kick", "b")
	ctx := context.Background()
	for _, stmt := range []string{
		"INSERT INTO account_channels(account_id,channel,rank) VALUES (?, 'channel', 1)",
		"INSERT INTO account_force_channels(account_id,channel,rank,created_at) VALUES (?, 'channel', 1, 1)",
		"INSERT INTO account_scans(account_id) VALUES (?)",
	} {
		_, err := e.s.d.DB.ExecContext(ctx, stmt, a)
		require.NoError(t, err)
	}
	_, err := e.s.d.DB.ExecContext(ctx, "INSERT INTO kv(key,value) VALUES ('settings:account_group_queue','{}')")
	require.NoError(t, err)
	_, csrf := e.session()
	// An SQL failure must roll back children as well as account rows.
	_, err = e.s.d.DB.ExecContext(ctx, "CREATE TRIGGER fail_purge BEFORE DELETE ON accounts BEGIN SELECT RAISE(ABORT, 'blocked'); END")
	require.NoError(t, err)
	resp, body := e.do("DELETE", "/api/accounts", csrf, nil)
	require.Equal(t, 500, resp.StatusCode, string(body))
	var n int
	require.NoError(t, e.s.d.DB.QueryRowContext(ctx, "SELECT count(*) FROM account_channels").Scan(&n))
	require.Equal(t, 1, n)
	_, err = e.s.d.DB.ExecContext(ctx, "DROP TRIGGER fail_purge")
	require.NoError(t, err)
	resp, body = e.do("DELETE", "/api/accounts", csrf, nil)
	require.Equal(t, 204, resp.StatusCode, string(body))
	for _, table := range []string{"accounts", "sessions", "account_channels", "account_force_channels", "account_scans", "drop_states"} {
		require.NoError(t, e.s.d.DB.QueryRowContext(ctx, "SELECT count(*) FROM "+table).Scan(&n))
		require.Zero(t, n, table)
	}
	require.NoError(t, e.s.d.DB.QueryRowContext(ctx, "SELECT count(*) FROM kv WHERE key='settings:account_group_queue'").Scan(&n))
	require.Zero(t, n)
}
