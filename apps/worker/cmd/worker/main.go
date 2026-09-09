package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/abrifuh6/buam-pulse/internal/checks"
	"github.com/abrifuh6/buam-pulse/internal/config"
	"github.com/abrifuh6/buam-pulse/internal/db"
	"github.com/abrifuh6/buam-pulse/internal/queue"
)

// Workers are stateless and scale horizontally: more customers → more
// replicas. In Phase 5 an HPA scales them on queue depth.
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

	for {
		id, err := q.Dequeue(ctx, 5*time.Second)
		if err != nil {
			log.Error("dequeue", "err", err); time.Sleep(time.Second); continue
		}
		if id == "" {
			continue
		}

		var typ, target string
		var expected, timeoutS int
		err = pool.QueryRow(ctx,
			`SELECT type, target, COALESCE(expected_status,200), timeout_seconds FROM monitors WHERE id=$1`, id).
			Scan(&typ, &target, &expected, &timeoutS)
		if err != nil {
			log.Error("load monitor", "id", id, "err", err); continue
		}

		var res checks.Result
		timeout := time.Duration(timeoutS) * time.Second
		if typ == "http" {
			res = checks.HTTP(ctx, target, expected, timeout)
		} else {
			res = checks.TCP(ctx, target, timeout)
		}

		_, err = pool.Exec(ctx, `
			INSERT INTO check_results (monitor_id, tenant_id, ok, status_code, latency_ms, error, region)
			SELECT id, tenant_id, $2, $3, $4, NULLIF($5,''), $6 FROM monitors WHERE id=$1`,
			id, res.OK, res.StatusCode, res.LatencyMs, res.Err, cfg.Region)
		if err != nil {
			log.Error("save result", "id", id, "err", err); continue
		}

		// Status transition + consecutive-failure counter (alerting hooks in here later).
		_, _ = pool.Exec(ctx, `
			UPDATE monitors SET
			  consecutive_fails = CASE WHEN $2 THEN 0 ELSE consecutive_fails + 1 END,
			  status = CASE WHEN $2 THEN 'up'
			                WHEN consecutive_fails + 1 >= 2 THEN 'down'
			                ELSE status END,
			  updated_at = now()
			WHERE id=$1`, id, res.OK)

		log.Info("checked", "id", id, "ok", res.OK, "latency_ms", res.LatencyMs, "status", res.StatusCode)
	}
}
