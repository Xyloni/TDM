package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/aalejandrofer/grubdrops/internal/store/gen"
	"github.com/stretchr/testify/require"
)

func TestDropExclusionsPersistAcrossAccountsAndCanBeRemoved(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "exclusions.db"))
	require.NoError(t, err)
	defer db.Close()
	s := NewDropExclusions(gen.New(db))
	require.NoError(t, s.Set(ctx, "twitch", "badge", true))
	list, err := NewDropExclusions(gen.New(db)).List(ctx, "twitch")
	require.NoError(t, err)
	require.True(t, list["badge"], "preference must not depend on any account existing")
	other, err := s.List(ctx, "kick")
	require.NoError(t, err)
	require.Empty(t, other)
	require.NoError(t, s.Set(ctx, "twitch", "badge", false))
	list, err = s.List(ctx, "twitch")
	require.NoError(t, err)
	require.Empty(t, list)
}
