package repository

import (
	"context"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

type DBTX interface {
	Begin(ctx context.Context) (pgx.Tx, error)
	Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error)
	Query(context.Context, string, ...interface{}) (pgx.Rows, error)
	QueryRow(context.Context, string, ...interface{}) pgx.Row
	SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults
}

// SetupPool inits pool.
func SetupPool(ctx context.Context, cfg Config) (*pgxpool.Pool, error) {
	return pgxpool.Connect(ctx, cfg.URL())
}

// MustPool inits pool.
// Panics in case of error.
func MustPool(pool *pgxpool.Pool, err error) *pgxpool.Pool {
	if err != nil {
		panic(err)
	}
	return pool
}
