// Package postgres implements the storage interfaces using PostgreSQL.
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mvaleed/aegis/internal/domain"
	"github.com/mvaleed/aegis/internal/storage"
)

// DB wraps the PostgreSQL connection pool and provides access to repositories.
type DB struct {
	pool *pgxpool.Pool
}

// New creates a new PostgreSQL database connection.
func New(ctx context.Context, connString string) (*DB, error) {
	config, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("parsing connection string: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("creating connection pool: %w", err)
	}

	// Verify connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	return &DB{pool: pool}, nil
}

// Close closes all connections in the pool.
func (db *DB) Close() {
	db.pool.Close()
}

// Pool returns the underlying connection pool.
// Use this sparingly - prefer using repository methods.
func (db *DB) Pool() *pgxpool.Pool {
	return db.pool
}

// Repositories returns all repositories backed by this database.
func (db *DB) Repositories() *storage.Repositories {
	return &storage.Repositories{
		Users:       NewUserRepository(db.pool),
		Roles:       NewRoleRepository(db.pool),
		Permissions: NewPermissionRepository(db.pool),
		Tokens:      NewTokenRepository(db.pool),
	}
}

// WithTransaction implements storage.Transactor.
// It executes the given function within a database transaction.
func (db *DB) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}

	// Put the transaction in context so repositories can use it
	txCtx := context.WithValue(ctx, txKey{}, tx)

	if err := fn(txCtx); err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			return fmt.Errorf("rolling back transaction: %v (original error: %w)", rbErr, err)
		}
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}

// txKey is the context key for the transaction.
type txKey struct{}

// DBTX is the interface satisfied by both *pgxpool.Pool and pgx.Tx.
// This allows repositories to work with or without an active transaction.
type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// getDB returns the transaction from context if present, otherwise the pool.
func getDB(ctx context.Context, pool *pgxpool.Pool) DBTX {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return tx
	}
	return pool
}

// Error code constants for PostgreSQL
const (
	uniqueViolationCode = "23505"
	foreignKeyViolation = "23503"
)

// mapError converts PostgreSQL errors to domain errors.
func mapError(err error) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case uniqueViolationCode:
			return domain.ErrAlreadyExists
		case foreignKeyViolation:
			return domain.ErrConflict
		}
	}

	return err
}
