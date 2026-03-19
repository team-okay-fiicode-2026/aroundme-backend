package database

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

func RunMigrations(ctx context.Context, postgres *Postgres, migrationsDir string) error {
	if postgres == nil {
		return fmt.Errorf("postgres is required")
	}

	dir := strings.TrimSpace(migrationsDir)
	if dir == "" {
		return fmt.Errorf("migrations directory is required")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read migrations directory: %w", err)
	}

	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		files = append(files, entry.Name())
	}
	slices.Sort(files)

	if _, err := postgres.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			filename TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`); err != nil {
		return fmt.Errorf("ensure schema_migrations table: %w", err)
	}

	rows, err := postgres.pool.Query(ctx, `SELECT filename FROM schema_migrations`)
	if err != nil {
		return fmt.Errorf("list applied migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[string]struct{}, len(files))
	for rows.Next() {
		var filename string
		if err := rows.Scan(&filename); err != nil {
			return fmt.Errorf("scan applied migration: %w", err)
		}
		applied[filename] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate applied migrations: %w", err)
	}

	// Baseline detection: if schema_migrations is empty but the database already has tables
	// (e.g. a deployment that was manually migrated before the migration runner existed),
	// mark all known files as applied without re-running them to avoid errors on non-idempotent SQL.
	if len(applied) == 0 {
		var tableExists bool
		if err := postgres.pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM information_schema.tables
				WHERE table_schema = 'public' AND table_name = 'users'
			)
		`).Scan(&tableExists); err != nil {
			return fmt.Errorf("check for existing schema: %w", err)
		}
		if tableExists {
			for _, filename := range files {
				if _, err := postgres.pool.Exec(ctx, `INSERT INTO schema_migrations (filename) VALUES ($1)`, filename); err != nil {
					return fmt.Errorf("baseline migration %s: %w", filename, err)
				}
				applied[filename] = struct{}{}
			}
		}
	}

	for _, filename := range files {
		if _, exists := applied[filename]; exists {
			continue
		}

		contents, err := os.ReadFile(filepath.Join(dir, filename))
		if err != nil {
			return fmt.Errorf("read migration %s: %w", filename, err)
		}

		tx, err := postgres.pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin migration %s: %w", filename, err)
		}

		if _, err := tx.Exec(ctx, string(contents)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("apply migration %s: %w", filename, err)
		}

		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (filename) VALUES ($1)`, filename); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("record migration %s: %w", filename, err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit migration %s: %w", filename, err)
		}
	}

	return nil
}
