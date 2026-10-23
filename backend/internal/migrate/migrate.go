// Package migrate applies the embedded SQL migrations in filename order, each
// in its own transaction, recording applied versions in fos.schema_migrations.
package migrate

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"sort"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var files embed.FS

func Up(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, `
		CREATE SCHEMA IF NOT EXISTS fos;
		CREATE TABLE IF NOT EXISTS fos.schema_migrations (
			version    VARCHAR PRIMARY KEY,
			applied_at TIMESTAMP NOT NULL DEFAULT (now() AT TIME ZONE 'UTC')
		)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}
	names, err := fs.Glob(files, "migrations/*.sql")
	if err != nil {
		return err
	}
	sort.Strings(names)
	for _, name := range names {
		var applied bool
		if err := pool.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM fos.schema_migrations WHERE version = $1)`, name).Scan(&applied); err != nil {
			return err
		}
		if applied {
			continue
		}
		sql, err := files.ReadFile(name)
		if err != nil {
			return err
		}
		tx, err := pool.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, string(sql)); err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("apply %s: %w", name, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO fos.schema_migrations (version) VALUES ($1)`, name); err != nil {
			tx.Rollback(ctx)
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
	}
	return nil
}
