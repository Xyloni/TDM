package log

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionLogFullConcurrentAndReplacedOnRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs", "miner.log")
	f, err := OpenSessionFile(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })
	var console bytes.Buffer
	ring := NewRing(2)
	logger := NewWithSession(&console, f, "info", ring).With("account", "a1").WithGroup("watch")
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) { defer wg.Done(); logger.Debug("heartbeat", "number", n) }(i)
	}
	wg.Wait()
	logger.Info("session completed")
	// Read before closing: journal writes must be available during the run.
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	decoder := json.NewDecoder(bytes.NewReader(data))
	for i := 0; i < 101; i++ {
		var entry map[string]any
		require.NoError(t, decoder.Decode(&entry))
		assert.Equal(t, "a1", entry["account"])
		if entry["msg"] == "heartbeat" {
			assert.NotNil(t, entry["watch"])
		}
	}
	assert.NotContains(t, console.String(), "heartbeat")
	assert.Contains(t, console.String(), "session completed")
	assert.Len(t, ring.Snapshot(), 1, "debug events must not flood dashboard memory")
	require.NoError(t, f.Close())
	second, err := OpenSessionFile(path)
	require.NoError(t, err)
	defer second.Close()
	NewWithSession(&console, second, "info", NewRing(2)).Info("new session")
	data, err = os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), "new session")
	assert.NotContains(t, string(data), "session completed")
	assert.NotContains(t, string(data), "heartbeat")
}
