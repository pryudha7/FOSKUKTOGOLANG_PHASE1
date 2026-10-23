// Package settings reads Dynamic System Settings (SoT §9.25): values change
// without a redeploy, read when used with an in-memory cache of at most 60
// seconds (DB-RULE-084). There are no hidden defaults: a missing or invalid
// setting is an error.
package settings

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AppScope is this application's app_scope; GLOBAL is read when it has no row
// (DB-RULE-083).
const AppScope = "LOAN_APPLICATION"

// cacheFor is DB-RULE-084's maximum cache age.
const cacheFor = 60 * time.Second

// Purposes read by the application (DB-RULE-082).
const MaxFileBytes = "validation/upload/max_file_bytes"

type Settings struct {
	db  *pgxpool.Pool
	now func() time.Time

	mu    sync.Mutex
	cache map[string]entry
}

type entry struct {
	value, valueType string
	read             time.Time
}

func New(db *pgxpool.Pool, now func() time.Time) *Settings {
	return &Settings{db: db, now: now, cache: map[string]entry{}}
}

// Int reads an INTEGER setting.
func (s *Settings) Int(ctx context.Context, purpose string) (int64, error) {
	e, err := s.read(ctx, purpose)
	if err != nil {
		return 0, err
	}
	n, err := strconv.ParseInt(e.value, 10, 64)
	if e.valueType != "INTEGER" || err != nil {
		return 0, fmt.Errorf("setting %s is not an INTEGER", purpose)
	}
	return n, nil
}

func (s *Settings) read(ctx context.Context, purpose string) (entry, error) {
	now := s.now()
	s.mu.Lock()
	e, ok := s.cache[purpose]
	s.mu.Unlock()
	if ok && now.Sub(e.read) < cacheFor && !now.Before(e.read) {
		return e, nil
	}
	err := s.db.QueryRow(ctx, `
		SELECT value, value_type FROM fos.dynamic_system_settings
		WHERE purpose = $1 AND app_scope IN ($2, 'GLOBAL')
		ORDER BY app_scope = 'GLOBAL' LIMIT 1`, purpose, AppScope).Scan(&e.value, &e.valueType)
	if errors.Is(err, pgx.ErrNoRows) {
		return entry{}, fmt.Errorf("setting %s missing", purpose)
	}
	if err != nil {
		return entry{}, err
	}
	e.read = now
	s.mu.Lock()
	s.cache[purpose] = e
	s.mu.Unlock()
	return e, nil
}
