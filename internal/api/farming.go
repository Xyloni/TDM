package api

import (
	"github.com/aalejandrofer/grubdrops/internal/farming"
	"github.com/aalejandrofer/grubdrops/internal/store"
	"net/http"
)

func (s *server) getFarming(w http.ResponseWriter, r *http.Request) {
	if s.d.Pool == nil {
		writeErr(w, http.StatusServiceUnavailable, "unavailable")
		return
	}
	accounts, err := s.d.Q.ListAllAccounts(r.Context())
	if err != nil {
		writeErr(w, 500, "internal")
		return
	}
	queue := store.AccountGroupQueue{}
	if s.d.Settings != nil {
		queue, err = s.d.Settings.AccountGroupsQueue(r.Context())
		if err != nil {
			writeErr(w, 500, "internal")
			return
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, farming.Build(s.overviewNow(), accounts, s.d.Pool.Snapshots(), queue))
}
