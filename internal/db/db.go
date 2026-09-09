package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Connect returns a connection pool. Pools matter in Kubernetes: each pod
// keeps a small number of connections, and Postgres has a hard connection cap.
func Connect(ctx context.Context, url string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, err
	}
	cfg.MaxConns = 10
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return pool, pool.Ping(ctx)
}
