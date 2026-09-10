package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/abrifuh6/buam-pulse/internal/alerting"
	"github.com/abrifuh6/buam-pulse/internal/checks"
	"github.com/abrifuh6/buam-pulse/internal/config"
	"github.com/abrifuh6/buam-pulse/internal/db"
	"github.com/abrifuh6/buam-pulse/internal/metrics"
	"github.com/abrifuh6/buam-pulse/internal/queue"
)

// Workers are stateless and scale horizontally: more customers → more
// replicas. In Phase 5 an HPA scales them on queue depth.
func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(log)
	cfg, err := config.Load()
	if err != nil {
		log.Error("config", "err", err)
		os.Exit(1)
	}
	ctx := context.Background()
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("db", "err", err)
		os.Exit(1)
	}
	q, err := queue.New(cfg.RedisURL)
	metrics.Serve(":" + cfg.MetricsPort)
	log.Info("worker started", "metrics_port", cfg.MetricsPort)

	if err != nil {
		log.Error("redis", "err", err)
		os.Exit(1)
	}

	for {
		id, err := q.Dequeue(ctx, 5*time.Second)
		if err != nil {
			log.Error("dequeue", "err", err)
			time.Sleep(time.Second)
			continue
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
			log.Error("load monitor", "id", id, "err", err)
			continue
		}

		var res checks.Result
		timeout := time.Duration(timeoutS) * time.Second
		if typ == "http" {
			res = checks.HTTP(ctx, target, expected, timeout)
		} else {
			res = checks.TCP(ctx, target, timeout)
		}

		result := "ok"
		if !res.OK {
			result = "fail"
		}
		metrics.ChecksTotal.WithLabelValues(typ, result).Inc()
		metrics.CheckLatency.WithLabelValues(typ).Observe(float64(res.LatencyMs) / 1000)

		_, err = pool.Exec(ctx, `
			INSERT INTO check_results (monitor_id, tenant_id, ok, status_code, latency_ms, error, region)
			SELECT id, tenant_id, $2, $3, $4, NULLIF($5,''), $6 FROM monitors WHERE id=$1`,
			id, res.OK, res.StatusCode, res.LatencyMs, res.Err, cfg.Region)
		if err != nil {
			log.Error("save result", "id", id, "err", err)
			continue
		}

		// Incident transitions and notification queueing live in one place.
		cause := res.Err
		if cause == "" && !res.OK {
			cause = fmt.Sprintf("unexpected status %d", res.StatusCode)
		}
		if err := alerting.ProcessResult(ctx, pool, id, res.OK, cause); err != nil {
			log.Error("process result", "id", id, "err", err)
		}

		log.Info("checked", "id", id, "ok", res.OK, "latency_ms", res.LatencyMs, "status", res.StatusCode)
	}
}
