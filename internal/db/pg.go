package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/JustinTDCT/CineVault/internal/catalog"
)

// migrationLockID is the PostgreSQL advisory lock key that serializes
// migrations, so two servers starting at once can't apply the same change twice.
const migrationLockID int64 = 724683001

// DB is CineVault's connection pool.
type DB struct {
	Pool *pgxpool.Pool
}

// Open connects to PostgreSQL and checks the connection.
func Open(ctx context.Context, databaseURL string) (*DB, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("database settings: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("connect to database: %w", err)
	}
	return &DB{Pool: pool}, nil
}

// Close closes all connections.
func (d *DB) Close() { d.Pool.Close() }

// Ping checks the database is reachable.
func (d *DB) Ping(ctx context.Context) error { return d.Pool.Ping(ctx) }

// Migrate applies any pending schema migrations.
func (d *DB) Migrate(ctx context.Context) (MigrateResult, error) {
	ms, err := LoadMigrations()
	if err != nil {
		return MigrateResult{}, err
	}
	return migrate(ctx, pgStore{pool: d.Pool}, ms)
}

// EditionTypes returns the whole Edition List, in display order.
func (d *DB) EditionTypes(ctx context.Context) ([]catalog.EditionType, error) {
	rows, err := d.Pool.Query(ctx, `
		SELECT id, display_name, short_label, media_types, aliases,
		       sort_order, is_default, builtin, hidden
		FROM edition_types
		ORDER BY sort_order, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []catalog.EditionType{}
	for rows.Next() {
		var e catalog.EditionType
		if err := rows.Scan(&e.ID, &e.DisplayName, &e.ShortLabel, &e.MediaTypes, &e.Aliases,
			&e.SortOrder, &e.IsDefault, &e.Builtin, &e.Hidden); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// pgStore implements migrationStore on PostgreSQL.
type pgStore struct {
	pool *pgxpool.Pool
}

func (s pgStore) Init(ctx context.Context) error {
	return s.locked(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			CREATE TABLE IF NOT EXISTS schema_migrations (
				version     integer PRIMARY KEY,
				name        text NOT NULL,
				applied_at  timestamptz NOT NULL DEFAULT now()
			)`)
		return err
	})
}

func (s pgStore) Applied(ctx context.Context) (map[int]bool, error) {
	rows, err := s.pool.Query(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	done := map[int]bool{}
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		done[v] = true
	}
	return done, rows.Err()
}

func (s pgStore) Apply(ctx context.Context, m Migration) error {
	return s.locked(ctx, func(tx pgx.Tx) error {
		// Another server may have applied it while we waited for the lock.
		var exists bool
		if err := tx.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`,
			m.Version).Scan(&exists); err != nil {
			return err
		}
		if exists {
			return nil
		}
		// Migration files hold several statements, which need PostgreSQL's
		// simple query protocol, so run them on the underlying connection.
		if _, err := tx.Conn().PgConn().Exec(ctx, m.SQL).ReadAll(); err != nil {
			return err
		}
		_, err := tx.Exec(ctx,
			`INSERT INTO schema_migrations (version, name) VALUES ($1, $2)`,
			m.Version, m.Name)
		return err
	})
}

// locked runs fn in a transaction that holds the migration advisory lock.
func (s pgStore) locked(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	// Rollback is a no-op once the transaction has committed.
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, migrationLockID); err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
