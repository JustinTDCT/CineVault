// Package db owns CineVault's PostgreSQL schema and queries.
package db

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strconv"
	"strings"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// Migration is one numbered schema change, loaded from migrations/NNNN_name.sql.
type Migration struct {
	Version int
	Name    string
	SQL     string
}

// migrationStore is what the runner needs from the database.
// pgStore implements it; tests use a fake.
type migrationStore interface {
	// Init creates the schema_migrations table if it doesn't exist.
	Init(ctx context.Context) error
	// Applied returns the versions already applied.
	Applied(ctx context.Context) (map[int]bool, error)
	// Apply runs one migration and records it, atomically. It must be safe to
	// call concurrently from several servers: a migration already recorded by
	// another process is skipped, not run twice.
	Apply(ctx context.Context, m Migration) error
}

// LoadMigrations reads the embedded migrations in version order.
func LoadMigrations() ([]Migration, error) {
	return loadMigrations(migrationFiles, "migrations")
}

func loadMigrations(fsys fs.FS, dir string) ([]Migration, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, err
	}
	var ms []Migration
	seen := map[int]string{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		num, name, ok := strings.Cut(strings.TrimSuffix(e.Name(), ".sql"), "_")
		v, err := strconv.Atoi(num)
		if !ok || err != nil || v <= 0 || name == "" {
			return nil, fmt.Errorf("migration %q: name must look like 0001_description.sql", e.Name())
		}
		if prev, dup := seen[v]; dup {
			return nil, fmt.Errorf("migrations %q and %q share version %d", prev, e.Name(), v)
		}
		seen[v] = e.Name()
		body, err := fs.ReadFile(fsys, path.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		ms = append(ms, Migration{Version: v, Name: name, SQL: string(body)})
	}
	sort.Slice(ms, func(i, j int) bool { return ms[i].Version < ms[j].Version })
	for i, m := range ms {
		if m.Version != i+1 {
			return nil, fmt.Errorf("migration versions must run 1, 2, 3… without gaps; found %d at position %d", m.Version, i+1)
		}
	}
	return ms, nil
}

// MigrateResult reports what a migration run did.
type MigrateResult struct {
	Applied []Migration // newly applied, in order
	Current int         // schema version after the run
}

func migrate(ctx context.Context, store migrationStore, ms []Migration) (MigrateResult, error) {
	var res MigrateResult
	if err := store.Init(ctx); err != nil {
		return res, fmt.Errorf("create schema_migrations: %w", err)
	}
	done, err := store.Applied(ctx)
	if err != nil {
		return res, fmt.Errorf("read applied migrations: %w", err)
	}
	known := map[int]bool{}
	for _, m := range ms {
		known[m.Version] = true
	}
	for v := range done {
		if !known[v] {
			return res, fmt.Errorf("database has migration %d, which this version of CineVault doesn't know; is the server older than the database?", v)
		}
	}
	for _, m := range ms {
		if done[m.Version] {
			res.Current = m.Version
			continue
		}
		if err := store.Apply(ctx, m); err != nil {
			return res, fmt.Errorf("migration %04d_%s: %w", m.Version, m.Name, err)
		}
		res.Applied = append(res.Applied, m)
		res.Current = m.Version
	}
	return res, nil
}
