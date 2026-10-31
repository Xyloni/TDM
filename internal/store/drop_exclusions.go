package store

import (
	"context"
	"database/sql"
	"strings"

	"github.com/aalejandrofer/grubdrops/internal/store/gen"
)

// Drop exclusions are platform-wide preferences, including future accounts.
// They do not record a claim or pretend a platform prerequisite was satisfied.
type DropExclusions struct{ q *gen.Queries }

func NewDropExclusions(q *gen.Queries) *DropExclusions { return &DropExclusions{q: q} }
func (s *DropExclusions) List(ctx context.Context, platform string) (map[string]bool, error) {
	prefix := "excluded_drop:" + platform + ":"
	rows, err := s.q.ListKVByPrefix(ctx, sql.NullString{String: prefix, Valid: true})
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for _, row := range rows {
		if string(row.Value) == "1" {
			out[strings.TrimPrefix(row.Key, prefix)] = true
		}
	}
	return out, nil
}
func (s *DropExclusions) Set(ctx context.Context, platform, id string, excluded bool) error {
	key := "excluded_drop:" + platform + ":" + id
	if !excluded {
		return s.q.DeleteKV(ctx, key)
	}
	return s.q.UpsertSettingString(ctx, gen.UpsertSettingStringParams{Key: key, Value: []byte("1")})
}
