package store

import (
	"context"
	"github.com/aalejandrofer/grubdrops/internal/store/gen"
	"github.com/stretchr/testify/require"
	"path/filepath"
	"testing"
)

func TestFarmDigestMessageIsScopedAndSurvivesStoreRebuild(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	defer db.Close()
	s := NewSettings(gen.New(db))
	require.NoError(t, s.SetFarmDigestMessage(ctx, "test-token", "123", 42))
	s = NewSettings(gen.New(db))
	id, err := s.FarmDigestMessage(ctx, "test-token", "123")
	require.NoError(t, err)
	require.Equal(t, int64(42), id)
	id, err = s.FarmDigestMessage(ctx, "other-token", "123")
	require.NoError(t, err)
	require.Zero(t, id)
	id, err = s.FarmDigestMessage(ctx, "test-token", "456")
	require.NoError(t, err)
	require.Zero(t, id)
}
