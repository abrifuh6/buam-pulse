// worker pulls monitor IDs off the queue, performs the check, records the
// result, and hands transitions to the alerting package.
//
// Stateless and horizontally scalable: more customers means more replicas.
package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/abrifuh6/buam-pulse/internal/alerting"
	"github.com/abrifuh6/buam-pulse/internal/checks"
	"github.com/abrifuh6/buam-pulse/internal/config"
	"github.com/abrifuh6/buam-pulse/internal/db"
	"github.com/abrifuh6/buam-pulse/internal/metrics"
	"github.com/abrifuh6/buam-pulse/internal/queue"
)

type monitorSpec struct {
	Type           string
	Target         string
	ExpectedStatus int
	TimeoutSeconds int
	Keyword        *string
	KeywordPresent bool
	CheckSSL       bool
	SSLWarnDays    int
}

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

	q, err := queue.New(cfg.RedisURL)
	if err != nil {
		log.Error("redis", "err", err)
		os.Exit(1)
	}

	metrics.Serve(":" + cfg.MetricsPort)
	log.Info("worker started", "metrics_port", cfg.MetricsPort, "region", cfg.Region)

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
		handle(ctx, pool, cfg.Region, id, log)
	}
}

func handle(ctx context.Context, pool *pgxpool.Pool, region, id string, log *slog.Logger) {
	var m monitorSpec
	err := pool.QueryRow(ctx, `
		SELECT type, target, COALESCE(expected_status, 200), timeout_seconds,
		       keyword, keyword_present, check_ssl, ssl_warn_days
		FROM monitors WHERE id=$1`, id).
		Scan(&m.Type, &m.Target, &m.ExpectedStatus, &m.TimeoutSeconds,
			&m.Keyword, &m.KeywordPresent, &m.CheckSSL, &m.SSLWarnDays)
	if err != nil {
		log.Error("load monitor", "id", id, "err", err)
		return
	}

	timeout := time.Duration(m.TimeoutSeconds) * time.Second

	var res checks.Result
	if m.Type == "http" {
		opt := checks.Options{
			ExpectedStatus: m.ExpectedStatus,
			Timeout:        timeout,
			KeywordPresent: m.KeywordPresent,
			CheckSSL:       m.CheckSSL,
		}
		if m.Keyword != nil {
			opt.Keyword = *m.Keyword
		}
		res = checks.HTTP(ctx, m.Target, opt)
	} else {
		res = checks.TCP(ctx, m.Target, timeout)
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO check_results
			(monitor_id, tenant_id, ok, status_code, latency_ms, error, region, failure_kind)
		SELECT id, tenant_id, $2, $3, $4, NULLIF($5,''), $6, NULLIF($7,'')
		FROM monitors WHERE id=$1`,
		id, res.OK, res.StatusCode, res.LatencyMs, res.Err, region, res.Kind); err != nil {
		log.Error("save result", "id", id, "err", err)
		return
	}

	// Certificate details are recorded whenever we learn them, independent of
	// whether the check passed: a near-expiry cert on a healthy site is
	// precisely the case worth warning about before it becomes an outage.
	if res.CertExpiry != nil {
		if _, err := pool.Exec(ctx, `
			UPDATE monitors SET ssl_expires_at=$2, ssl_issuer=$3 WHERE id=$1`,
			id, *res.CertExpiry, res.CertIssuer); err != nil {
			log.Warn("save cert details", "id", id, "err", err)
		}
		days := int(time.Until(*res.CertExpiry).Hours() / 24)
		metrics.CertDaysRemaining.WithLabelValues(id).Set(float64(days))
	}

	result := "ok"
	if !res.OK {
		result = "fail"
	}
	metrics.ChecksTotal.WithLabelValues(m.Type, result).Inc()
	metrics.CheckLatency.WithLabelValues(m.Type).Observe(float64(res.LatencyMs) / 1000)

	if err := alerting.ProcessResult(ctx, pool, id, res.OK, res.Err); err != nil {
		log.Error("process result", "id", id, "err", err)
	}

	log.Info("checked", "id", id, "ok", res.OK,
		"latency_ms", res.LatencyMs, "status", res.StatusCode, "kind", res.Kind)
}
