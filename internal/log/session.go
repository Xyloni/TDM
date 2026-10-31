package log

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

// OpenSessionFile replaces the previous session's journal. Writes go directly
// to the file, without a growing in-memory buffer or a flush timer.
func OpenSessionFile(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	return os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
}

// NewWithSession keeps the console/dashboard level and records all debug and
// higher events to disk, including heartbeat timing needed for diagnostics.
func NewWithSession(console, file io.Writer, level string, ring *Ring) *slog.Logger {
	return slog.New(&sessionHandler{handlers: []slog.Handler{
		NewWithRing(console, level, ring).Handler(),
		slog.NewJSONHandler(file, &slog.HandlerOptions{Level: slog.LevelDebug}),
	}})
}

type sessionHandler struct{ handlers []slog.Handler }

func (h *sessionHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, inner := range h.handlers {
		if inner.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (h *sessionHandler) Handle(ctx context.Context, record slog.Record) error {
	var err error
	for _, inner := range h.handlers {
		if inner.Enabled(ctx, record.Level) {
			err = errors.Join(err, inner.Handle(ctx, record.Clone()))
		}
	}
	return err
}

func (h *sessionHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	out := &sessionHandler{}
	for _, inner := range h.handlers {
		out.handlers = append(out.handlers, inner.WithAttrs(attrs))
	}
	return out
}

func (h *sessionHandler) WithGroup(name string) slog.Handler {
	out := &sessionHandler{}
	for _, inner := range h.handlers {
		out.handlers = append(out.handlers, inner.WithGroup(name))
	}
	return out
}
