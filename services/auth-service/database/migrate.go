// Package database provides a minimal, dependency-free migration runner
// for Auth Service's Postgres schema (M2 §5). It deliberately does not
// pull in a migration framework (golang-migrate, goose, etc.) — go.mod
// (go.mod) does not currently list one, and
// adding a new dependency is outside the scope of "generate NEW files
// only" for this session. Instead it does the same thing those tools do,
// at the size this project actually needs: read numbered .sql files in
// order, track which have run in a schema_migrations table, apply the
// rest inside a transaction each.
package database

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

var upFilePattern = regexp.MustCompile(`^(\d+)_.+\.up\.sql$`)

type migration struct {
	version int64
	name    string
	upSQL   string
	downSQL string
}

// loadMigrations reads every *.up.sql file under migrations/, pairs each
// with its *.down.sql counterpart, and returns them sorted ascending by
// version number (the numeric prefix, e.g. 0001).
func loadMigrations() ([]migration, error) {
	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		return nil, fmt.Errorf("reading embedded migrations dir: %w", err)
	}

	var migrations []migration
	for _, entry := range entries {
		matches := upFilePattern.FindStringSubmatch(entry.Name())
		if matches == nil {
			continue
		}
		version, err := strconv.ParseInt(matches[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parsing migration version from %q: %w", entry.Name(), err)
		}

		upBytes, err := migrationFiles.ReadFile(path.Join("migrations", entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("reading %q: %w", entry.Name(), err)
		}

		downName := strings.TrimSuffix(entry.Name(), ".up.sql") + ".down.sql"
		downBytes, err := migrationFiles.ReadFile(path.Join("migrations", downName))
		if err != nil {
			return nil, fmt.Errorf("reading %q (down counterpart of %q): %w", downName, entry.Name(), err)
		}

		migrations = append(migrations, migration{
			version: version,
			name:    entry.Name(),
			upSQL:   string(upBytes),
			downSQL: string(downBytes),
		})
	}

	sort.Slice(migrations, func(i, j int) bool { return migrations[i].version < migrations[j].version })
	return migrations, nil
}

const ensureVersionTableSQL = `
CREATE TABLE IF NOT EXISTS public.schema_migrations (
    version     BIGINT PRIMARY KEY,
    name        TEXT NOT NULL,
    applied_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
`

func appliedVersions(ctx context.Context, pool *pgxpool.Pool) (map[int64]bool, error) {
	rows, err := pool.Query(ctx, `SELECT version FROM public.schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("reading applied migrations: %w", err)
	}
	defer rows.Close()

	applied := map[int64]bool{}
	for rows.Next() {
		var v int64
		if err := rows.Scan(&v); err != nil {
			return nil, fmt.Errorf("scanning applied migration version: %w", err)
		}
		applied[v] = true
	}
	return applied, rows.Err()
}

// Up applies every migration that hasn't already been recorded in
// schema_migrations, in ascending version order, each in its own
// transaction. It is safe to call repeatedly (a no-op once everything is
// applied) — that's what makes it safe to run from a deployment's init
// step (see deployment/docker-compose.yml) on every start.
func Up(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, ensureVersionTableSQL); err != nil {
		return fmt.Errorf("ensuring schema_migrations table exists: %w", err)
	}

	migrations, err := loadMigrations()
	if err != nil {
		return err
	}

	applied, err := appliedVersions(ctx, pool)
	if err != nil {
		return err
	}

	for _, m := range migrations {
		if applied[m.version] {
			continue
		}

		tx, err := pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("beginning transaction for migration %s: %w", m.name, err)
		}

		if _, err := tx.Exec(ctx, m.upSQL); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("applying migration %s: %w", m.name, err)
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO public.schema_migrations (version, name) VALUES ($1, $2)`,
			m.version, m.name,
		); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("recording migration %s: %w", m.name, err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("committing migration %s: %w", m.name, err)
		}
	}

	return nil
}

// Down rolls back the single most recently applied migration. Auth
// Service only has one migration today, so this is mainly here for the
// same reason the .down.sql files themselves are: the next migration
// added should follow this same up/down pairing convention.
func Down(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, ensureVersionTableSQL); err != nil {
		return fmt.Errorf("ensuring schema_migrations table exists: %w", err)
	}

	migrations, err := loadMigrations()
	if err != nil {
		return err
	}
	applied, err := appliedVersions(ctx, pool)
	if err != nil {
		return err
	}

	var last *migration
	for i := range migrations {
		if applied[migrations[i].version] {
			last = &migrations[i]
		}
	}
	if last == nil {
		return nil // nothing applied, nothing to roll back
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction for rollback of %s: %w", last.name, err)
	}
	if _, err := tx.Exec(ctx, last.downSQL); err != nil {
		_ = tx.Rollback(ctx)
		return fmt.Errorf("rolling back migration %s: %w", last.name, err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM public.schema_migrations WHERE version = $1`, last.version); err != nil {
		_ = tx.Rollback(ctx)
		return fmt.Errorf("unrecording migration %s: %w", last.name, err)
	}
	return tx.Commit(ctx)
}
