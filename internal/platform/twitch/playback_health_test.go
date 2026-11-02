package twitch

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/aalejandrofer/grubdrops/internal/platform"
	"github.com/stretchr/testify/require"
)

func TestHLS404RemainsTypedAfterRefresh(t *testing.T) {
	f := newHLSFake(t, "live")
	f.chunkStatus = func(string) int { return http.StatusNotFound }
	w := &watch{c: f.client(), cachedUserID: 123}
	h := startWatch(t, w, "live", "broadcast")
	err := w.pulse(context.Background(), h)
	require.ErrorIs(t, err, platform.ErrPlaybackUnavailable)
	f.mu.Lock()
	hits := f.masterHits
	f.mu.Unlock()
	require.Equal(t, 2, hits, "a failed cached URL must be refreshed before recovery")
	require.NotErrorIs(t, fmt.Errorf("wrapped: %w", hlsStatusError("playlist", 503)), platform.ErrPlaybackUnavailable)
}
