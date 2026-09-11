// retention rolls yesterday's check results into daily summaries and prunes
// raw rows past each plan's window.
//
// Runs as its own process rather than inside an existing service: it is
// long-running, IO-heavy, and must not compete with the check pipeline. In
// Kubernetes it becomes a CronJob; here it loops on a timer so it behaves the
// same way locally.
package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/abrifuh6/buam-pulse/internal/config"
	"github.com/abrifuh6/buam-pulse/internal/db"
	"github.com/abrifuh6/buam-pulse/internal/metrics"
	"github.com/abrifuh6/buam-pulse/internal/retention"
)

const batchSize = 10000

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
	defer pool.Close()

	metrics.Serve(":" + cfg.MetricsPort)

	// RUN_ONCE makes this usable as a Kubernetes CronJob: do the work, exit.
	once := os.Getenv("RUN_ONCE") == "true"
	log.Info("retention started", "run_once", once, "metrics_port", cfg.MetricsPort)

	for {
		runPass(ctx, pool, log)
		if once {
			return
		}
		// Hourly rather than daily: a missed run then costs an hour, not a day,
		// and the rollup is idempotent so extra passes are harmless.
		time.Sleep(time.Hour)
	}
}

func runPass(ctx context.Context, pool *pgxpool.Pool, log *slog.Logger) {
	start := time.Now()

	rolled, err := retention.Rollup(ctx, pool)
	if err != nil {
		log.Error("rollup", "err", err)
	} else if rolled > 0 {
		log.Info("rollup complete", "days_rolled", rolled, "took", time.Since(start).String())
		metrics.RollupRows.Add(float64(rolled))
	}

	pruned, err := retention.Prune(ctx, pool, batchSize)
	if err != nil {
		log.Error("prune", "err", err)
	} else if pruned > 0 {
		log.Info("prune complete", "rows_deleted", pruned, "took", time.Since(start).String())
		metrics.PrunedRows.Add(float64(pruned))
	}
}
