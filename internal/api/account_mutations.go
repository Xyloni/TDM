package api

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/aalejandrofer/grubdrops/internal/store"
	"io"
	"net/http"
	"sync"
)

// Account mutations share one maintenance window, including all batch workers.
// Pool reloads from other services are queued while workers are drained.
func (s *server) accountMutation(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mutationMu.Lock()
		defer s.mutationMu.Unlock()
		if r.Context().Err() != nil {
			return
		}
		if p, ok := s.d.Pool.(interface{ Pause() func() }); ok {
			resume := p.Pause()
			defer func() {
				resume()
				if s.d.Hub != nil {
					s.d.Hub.Publish(Event{Name: EventAccounts, Data: struct{}{}})
				}
			}()
		}
		next.ServeHTTP(w, r)
	})
}

type batchUsersKey struct{}

// loginMu guards this index together with lookup/create/session persistence.
type batchUsers map[string]string

type batchLoginInput struct {
	URL   string `json:"url"`
	Group int    `json:"group,omitempty"`
}

// The batch delegates each item to the same login implementation as a single
// paste. Only upstream validation/avatar fetches overlap; SQLite writes stay short.
func (s *server) twitchBatchLogin(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Accounts []batchLoginInput `json:"accounts"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	if len(in.Accounts) == 0 || len(in.Accounts) > 2000 {
		writeErr(w, http.StatusBadRequest, "bad_request")
		return
	}
	for _, a := range in.Accounts {
		if a.URL == "" || a.Group < 0 || a.Group > 10000 {
			writeErr(w, http.StatusBadRequest, "bad_request")
			return
		}
		if a.Group > 0 && s.d.Settings == nil {
			writeErr(w, http.StatusServiceUnavailable, "unavailable")
			return
		}
	}
	users := batchUsers{}
	accounts, err := s.d.Q.ListAllAccounts(r.Context())
	if err != nil {
		writeErr(w, 500, "internal")
		return
	}
	for _, a := range accounts {
		if a.Platform != "twitch" {
			continue
		}
		sess, ok, err := s.d.Sessions.Get(r.Context(), a.ID)
		if err != nil {
			writeErr(w, 500, "internal")
			return
		}
		if ok && sess.UserID != "" {
			users[sess.UserID] = a.ID
		}
	}
	ctx := context.WithValue(r.Context(), batchUsersKey{}, users)
	results := make([]bool, len(in.Accounts))
	ids := make([]string, len(in.Accounts))
	jobs := make(chan int)
	var wg sync.WaitGroup
	for n := 0; n < min(16, len(in.Accounts)); n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				if ctx.Err() != nil {
					continue
				}
				body, _ := json.Marshal(in.Accounts[i])
				req := r.Clone(ctx)
				req.Body = io.NopCloser(bytes.NewReader(body))
				out := &loginStatus{header: make(http.Header)}
				s.twitchAndroidLogin(out, req)
				results[i] = out.status == http.StatusOK
				if results[i] {
					var result struct {
						ID string `json:"id"`
					}
					_ = json.Unmarshal(out.body.Bytes(), &result)
					ids[i] = result.ID
				}
			}
		}()
	}
	for i := range in.Accounts {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	var assignments []store.AccountGroupAssignment
	for i, a := range in.Accounts {
		if results[i] && a.Group > 0 {
			assignments = append(assignments, store.AccountGroupAssignment{AccountID: ids[i], Group: a.Group})
		}
	}
	if len(assignments) > 0 {
		if err := s.d.Settings.AddAccountGroups(ctx, assignments); err != nil {
			writeErr(w, http.StatusInternalServerError, "internal")
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}

type loginStatus struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (w *loginStatus) Header() http.Header         { return w.header }
func (w *loginStatus) WriteHeader(status int)      { w.status = status }
func (w *loginStatus) Write(b []byte) (int, error) { return w.body.Write(b) }

func (s *server) deleteAllAccounts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	accounts, err := s.d.Q.ListAllAccounts(ctx)
	if err != nil {
		writeErr(w, 500, "internal")
		return
	}
	for _, a := range accounts {
		s.cancelTwitchFlow(a.ID)
	}
	tx, err := s.d.DB.BeginTx(ctx, nil)
	if err != nil {
		writeErr(w, 500, "internal")
		return
	}
	defer tx.Rollback()
	// Explicit children also cover installations with foreign_keys disabled.
	for _, stmt := range []string{
		"DELETE FROM sessions", "DELETE FROM account_games",
		"DELETE FROM account_campaign_links", "DELETE FROM account_channels",
		"DELETE FROM account_force_channels", "DELETE FROM campaign_priorities WHERE account_id IS NOT NULL",
		"DELETE FROM progress", "DELETE FROM claims", "DELETE FROM drop_states",
		"DELETE FROM account_scans", "DELETE FROM notifications WHERE account_id IS NOT NULL",
		"DELETE FROM accounts", "DELETE FROM kv WHERE key = 'settings:account_group_queue'",
		"DELETE FROM kv WHERE key LIKE 'authcheck:%'",
		"DELETE FROM kv WHERE substr(key,1,14) = 'skip_override:' OR substr(key,1,12) = 'link_notice:'",
	} {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			writeErr(w, 500, "internal")
			return
		}
	}
	if err := tx.Commit(); err != nil {
		writeErr(w, 500, "internal")
		return
	}
	for _, a := range accounts {
		if s.d.AccountRemoved != nil {
			s.d.AccountRemoved(a.ID)
		}
	}
	w.WriteHeader(http.StatusNoContent)
}
