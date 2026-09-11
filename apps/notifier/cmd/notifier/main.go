// notifier drains the notifications table: claims due rows, sends them, and
// retries with exponential backoff. Separate from the worker so a slow or
// broken alert provider can never delay monitoring checks.
package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"math"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/abrifuh6/buam-pulse/internal/alerting"
	"github.com/abrifuh6/buam-pulse/internal/config"
	"github.com/abrifuh6/buam-pulse/internal/db"
	"github.com/abrifuh6/buam-pulse/internal/metrics"
)

// After this many failed attempts a notification is marked dead rather than
// retried forever. Dead rows stay in the table as evidence.
const maxAttempts = 6

type job struct {
	ID          int64
	Kind        string
	ChannelType string
	ChannelCfg  []byte
	MonitorName string
	Target      string
	TenantSlug  string
	StartedAt   time.Time
	Cause       *string
	Attempts    int
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

	metrics.Serve(":" + cfg.MetricsPort)
	log.Info("notifier started", "metrics_port", cfg.MetricsPort)

	smtpCfg := alerting.SMTPConfig{
		Host: cfg.SMTPHost, Port: cfg.SMTPPort,
		User: cfg.SMTPUser, Password: cfg.SMTPPassword, From: cfg.AlertFrom,
	}

	send := func(to, subject, body string) error {
		return alerting.SendEmail(smtpCfg, to, subject, body)
	}

	// Certificate expiry is checked far less often than the delivery queue is
	// drained: expiry dates move on the scale of days, and re-scanning every
	// five seconds would be pure waste.
	certTick := time.NewTicker(6 * time.Hour)
	go func() {
		if err := alerting.CheckCertExpiry(ctx, pool, send); err != nil {
			log.Error("cert expiry scan", "err", err)
		}
		for range certTick.C {
			if err := alerting.CheckCertExpiry(ctx, pool, send); err != nil {
				log.Error("cert expiry scan", "err", err)
			}
		}
	}()

	tick := time.NewTicker(5 * time.Second)
	for range tick.C {
		drain(ctx, pool, smtpCfg, cfg.StatusURL, log)
	}
}

func drain(ctx context.Context, pool *pgxpool.Pool, smtpCfg alerting.SMTPConfig, statusURL string, log *slog.Logger) {
	// FOR UPDATE SKIP LOCKED lets several notifier replicas drain the same
	// table without ever handing the same row to two of them.
	rows, err := pool.Query(ctx, `
		SELECT n.id, n.kind, c.type, c.config, m.name, m.target, t.slug,
		       i.started_at, i.cause, n.attempts
		FROM notifications n
		JOIN alert_channels c ON c.id = n.channel_id
		JOIN incidents i      ON i.id = n.incident_id
		JOIN monitors m       ON m.id = i.monitor_id
		JOIN tenants t        ON t.id = n.tenant_id
		WHERE n.status IN ('pending','failed')
		  AND n.next_try_at <= now()
		ORDER BY n.next_try_at
		LIMIT 50
		FOR UPDATE OF n SKIP LOCKED`)
	if err != nil {
		log.Error("claim notifications", "err", err)
		return
	}

	var jobs []job
	for rows.Next() {
		var j job
		if err := rows.Scan(&j.ID, &j.Kind, &j.ChannelType, &j.ChannelCfg,
			&j.MonitorName, &j.Target, &j.TenantSlug, &j.StartedAt, &j.Cause, &j.Attempts); err == nil {
			jobs = append(jobs, j)
		}
	}
	rows.Close()

	for _, j := range jobs {
		cause := ""
		if j.Cause != nil {
			cause = *j.Cause
		}
		subject, body := alerting.Message(j.Kind, j.MonitorName, j.Target,
			j.TenantSlug, statusURL, cause, j.StartedAt)

		var cfgMap map[string]string
		_ = json.Unmarshal(j.ChannelCfg, &cfgMap)

		var sendErr error
		switch j.ChannelType {
		case "email":
			sendErr = alerting.SendEmail(smtpCfg, cfgMap["address"], subject, body)
		case "slack":
			sendErr = alerting.SendSlack(ctx, cfgMap["webhook_url"], subject+"\n"+body)
		}

		if sendErr == nil {
			_, _ = pool.Exec(ctx, `
				UPDATE notifications
				SET status='sent', sent_at=now(), attempts=attempts+1, last_error=NULL
				WHERE id=$1`, j.ID)
			metrics.NotificationsSent.WithLabelValues(j.ChannelType, j.Kind, "sent").Inc()
			log.Info("notification sent", "id", j.ID, "channel", j.ChannelType, "kind", j.Kind)
			continue
		}

		attempts := j.Attempts + 1
		status := "failed"
		if attempts >= maxAttempts {
			status = "dead"
			metrics.NotificationsSent.WithLabelValues(j.ChannelType, j.Kind, "dead").Inc()
			log.Error("notification dead", "id", j.ID, "err", sendErr)
		} else {
			metrics.NotificationsSent.WithLabelValues(j.ChannelType, j.Kind, "failed").Inc()
			log.Warn("notification failed, will retry", "id", j.ID, "attempt", attempts, "err", sendErr)
		}

		// Exponential backoff: 1, 2, 4, 8, 16, 32 minutes.
		backoff := time.Duration(math.Pow(2, float64(attempts-1))) * time.Minute
		_, _ = pool.Exec(ctx, `
			UPDATE notifications
			SET status=$2, attempts=$3, last_error=$4, next_try_at=now()+$5::interval
			WHERE id=$1`, j.ID, status, attempts, sendErr.Error(), backoff.String())
	}

	// Expose the backlog so Prometheus can alert if delivery stalls.
	var pending int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM notifications WHERE status IN ('pending','failed')`).Scan(&pending); err == nil {
		metrics.NotificationBacklog.Set(float64(pending))
	}
}
