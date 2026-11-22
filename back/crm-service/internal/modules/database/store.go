package database

import (
	"context"
	"embed"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrations embed.FS

var ErrNotFound = errors.New("record not found")

type Store struct {
	pool *pgxpool.Pool
}

func Open(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	if err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		return migrate(ctx, tx)
	}); err != nil {
		pool.Close()
		return nil, fmt.Errorf("migrate CRM database: %w", err)
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() { s.pool.Close() }

func migrate(ctx context.Context, tx pgx.Tx) error {
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtext('crm-service migrations')::bigint)"); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, "CREATE TABLE IF NOT EXISTS crm_schema_migrations (version text PRIMARY KEY)"); err != nil {
		return err
	}
	entries, err := migrations.ReadDir("migrations")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		version := entry.Name()
		var applied bool
		if err := tx.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM crm_schema_migrations WHERE version = $1)", version).Scan(&applied); err != nil {
			return err
		}
		if applied {
			continue
		}
		body, err := migrations.ReadFile("migrations/" + version)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, string(body)); err != nil {
			return fmt.Errorf("%s: %w", version, err)
		}
		if _, err := tx.Exec(ctx, "INSERT INTO crm_schema_migrations (version) VALUES ($1)", version); err != nil {
			return err
		}
	}
	return nil
}

func one[T any](rows pgx.Rows, err error) (T, error) {
	if err != nil {
		var zero T
		return zero, err
	}
	value, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[T])
	if errors.Is(err, pgx.ErrNoRows) {
		return value, ErrNotFound
	}
	return value, err
}

func many[T any](rows pgx.Rows, err error) ([]T, error) {
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[T])
}
