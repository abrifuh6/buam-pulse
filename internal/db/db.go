package db

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Connect returns a connection pool. Pools matter in Kubernetes: each pod keeps
// a small number of connections, and Postgres has a hard connection cap.
//
// It does not retry. ConnectWithRetry is what services should call at startup —
// this remains for callers that want a single attempt (tests, migrations).
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

// ConnectWithRetry connects at startup, retrying a dependency that is not yet
// ready and logging clearly when it cannot be reached.
//
// This exists because the old behaviour — one attempt, then exit — made every
// startup problem look the same: a pod that crash-loops with no log line. A
// database that is briefly unavailable (a failover, a cold start) is survivable
// and should not crash the pod; a database that is genuinely unreachable (a
// security group blocking the port, a wrong endpoint) should say so by name
// rather than leaving someone to run a manual psql probe to find out.
//
// The host is logged on every attempt precisely because the most common real
// cause is a network path that is silently dropping packets, and the host is
// the first thing you check.
func ConnectWithRetry(ctx context.Context, url string, log *slog.Logger) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parsing database url: %w", err)
	}
	cfg.MaxConns = 10

	host := fmt.Sprintf("%s:%d", cfg.ConnConfig.Host, cfg.ConnConfig.Port)

	// ~30 seconds total: long enough to ride out a dependency still starting,
	// short enough that a genuine misconfiguration fails the pod fast rather
	// than hanging in CrashLoopBackOff with no signal.
	const maxAttempts = 10
	const delay = 3 * time.Second

	var pool *pgxpool.Pool
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		log.Info("connecting to database", "host", host, "attempt", attempt)

		pool, err = pgxpool.NewWithConfig(ctx, cfg)
		if err == nil {
			if err = pool.Ping(ctx); err == nil {
				log.Info("database connected", "host", host)
				return pool, nil
			}
			pool.Close()
		}

		if attempt < maxAttempts {
			log.Warn("database not ready, retrying",
				"host", host, "attempt", attempt, "err", err.Error())
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}

	// The message that would have turned an hour of debugging into a minute:
	// it names the host and the actual error, so "the network is dropping
	// packets to the database" is readable in the logs instead of inferred.
	return nil, fmt.Errorf("database at %s unreachable after %d attempts: %w",
		host, maxAttempts, err)
}
