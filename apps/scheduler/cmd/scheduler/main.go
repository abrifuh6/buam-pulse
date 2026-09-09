package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/abrifuh6/buam-pulse/internal/config"
	"github.com/abrifuh6/buam-pulse/internal/db"
	"github.com/abrifuh6/buam-pulse/internal/queue"
)

// The scheduler runs as ONE replica. Every 5s it claims due monitors and
// pushes their IDs onto the queue. "FOR UPDATE SKIP LOCKED" means that if a
// second replica ever runs, they never enqueue the same monitor twice.
func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		log.Error("config", "err", err); os.Exit(1)
	}
	ctx := context.Background()
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("db", "err", err); os.Exit(1)
	}
	q, err := queue.New(cfg.RedisURL)
	if err != nil {
		log.Error("redis", "err", err); os.Exit(1)
	}

	const claimSQL = `
		UPDATE monitors SET next_run_at = now() + (interval_seconds || ' seconds')::interval
		WHERE id IN (
			SELECT id FROM monitors
			WHERE enabled AND next_run_at <= now()
			ORDER BY next_run_at
			LIMIT 500
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id`

	tick := time.NewTicker(5 * time.Second)
	for range tick.C {
		rows, err := pool.Query(ctx, claimSQL)
		if err != nil {
			log.Error("claim", "err", err); continue
		}
		n := 0
		for rows.Next() {
			var id string
			if rows.Scan(&id) == nil {
				if err := q.Enqueue(ctx, id); err == nil {
					n++
				}
			}
		}
		rows.Close()
		if n > 0 {
			log.Info("enqueued", "count", n)
		}
	}
}
