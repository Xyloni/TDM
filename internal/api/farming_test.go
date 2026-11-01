package api

import (
	"encoding/json"
	"github.com/aalejandrofer/grubdrops/internal/farming"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestFarmingEndpointIncludesAccountsAndIsNotCached(t *testing.T) {
	e := newResEnv(t, nil)
	e.s.protected.Get("/farming", e.s.getFarming)
	resp, body := e.do("GET", "/api/farming", "", nil)
	require.Equal(t, 200, resp.StatusCode)
	require.Equal(t, "no-store", resp.Header.Get("Cache-Control"))
	var report farming.Report
	require.NoError(t, json.Unmarshal(body, &report))
	require.Len(t, report.Accounts, 2)
	require.Equal(t, 2, report.Group.Unknown)
	require.Nil(t, report.Group.ETAMinutes)
}

func TestFarmingEndpointRequiresAdmin(t *testing.T) {
	e := newEnv(t)
	e.s.protected.Get("/farming", e.s.getFarming)
	resp, _ := e.do("GET", "/api/farming", "", nil)
	require.Equal(t, 401, resp.StatusCode)
}
