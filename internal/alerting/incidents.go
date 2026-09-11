// Package alerting turns check results into incidents, and incidents into
// queued notifications. Kept out of the worker's main loop so the transition
// rules are testable and live in one place.
package alerting

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// FailureThreshold: a monitor must fail this many consecutive checks before we
// call it down. One transient blip should never page anyone.
const FailureThreshold = 2

// ProcessResult applies a check outcome to a monitor and, if the monitor
// changes state, opens or closes an incident and queues notifications.
//
// The whole thing runs in one transaction: if we crash midway, we never end up
// with an incident that has no notifications, or a monitor marked down with no
// incident recorded.
func ProcessResult(ctx context.Context, pool *pgxpool.Pool, monitorID string, ok bool, cause string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Lock the monitor row so two workers processing results for the same
	// monitor cannot both decide "this is the transition".
	var tenantID, name, status string
	var fails int
	err = tx.QueryRow(ctx, `
		SELECT tenant_id, name, status, consecutive_fails
		FROM monitors WHERE id=$1 FOR UPDATE`, monitorID).
		Scan(&tenantID, &name, &status, &fails)
	if err != nil {
		return err
	}

	newFails := fails + 1
	if ok {
		newFails = 0
	}

	newStatus := status
	switch {
	case ok:
		newStatus = "up"
	case newFails >= FailureThreshold:
		newStatus = "down"
	}

	if _, err = tx.Exec(ctx, `
		UPDATE monitors SET consecutive_fails=$2, status=$3, updated_at=now()
		WHERE id=$1`, monitorID, newFails, newStatus); err != nil {
		return err
	}

	switch {
	case newStatus == "down" && status != "down":
		var incidentID string
		if err = tx.QueryRow(ctx, `
			INSERT INTO incidents (tenant_id, monitor_id, cause)
			VALUES ($1,$2,$3) RETURNING id`, tenantID, monitorID, cause).Scan(&incidentID); err != nil {
			return err
		}
		if err = queueNotifications(ctx, tx, tenantID, monitorID, incidentID, "down"); err != nil {
			return err
		}
		slog.Info("incident opened", "monitor", name, "incident", incidentID)

	case newStatus == "up" && status == "down":
		var incidentID string
		err = tx.QueryRow(ctx, `
			UPDATE incidents SET resolved_at=now()
			WHERE monitor_id=$1 AND resolved_at IS NULL
			RETURNING id`, monitorID).Scan(&incidentID)
		if err == pgx.ErrNoRows {
			break // nothing open; nothing to announce
		}
		if err != nil {
			return err
		}
		if err = queueNotifications(ctx, tx, tenantID, monitorID, incidentID, "recovered"); err != nil {
			return err
		}
		slog.Info("incident resolved", "monitor", name, "incident", incidentID)
	}

	return tx.Commit(ctx)
}

// queueNotifications writes one pending row per target channel. The table's
// UNIQUE(incident_id, channel_id, kind) constraint means a retry of this whole
// operation can never produce a duplicate page — ON CONFLICT DO NOTHING makes
// that explicit rather than an error.
func queueNotifications(ctx context.Context, tx pgx.Tx, tenantID, monitorID, incidentID, kind string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO notifications (tenant_id, incident_id, channel_id, kind)
		SELECT $1, $2, c.id, $4
		FROM alert_channels c
		WHERE c.tenant_id = $1
		  AND c.enabled
		  -- email channels must be verified; slack webhooks are self-authorising
		  AND (c.type <> 'email' OR c.verified_at IS NOT NULL)
		  -- a monitor with explicit routing uses only those channels;
		  -- one with none uses all of the tenant's channels
		  AND (
		        NOT EXISTS (SELECT 1 FROM monitor_channels mc WHERE mc.monitor_id = $3)
		     OR EXISTS (SELECT 1 FROM monitor_channels mc
		                 WHERE mc.monitor_id = $3 AND mc.channel_id = c.id)
		      )
		ON CONFLICT (incident_id, channel_id, kind) DO NOTHING`,
		tenantID, incidentID, monitorID, kind)
	return err
}

// CheckCertExpiry queues a warning for certificates approaching expiry.
//
// Deliberately NOT an incident: the site is up, so opening an incident would
// corrupt uptime figures and put a false outage on the status page. This is a
// warning about a future problem, which is a different thing from a current one.
//
// ssl_alerted_for records which certificate we warned about, so renewing the
// cert re-arms the warning while the same cert never warns twice.
func CheckCertExpiry(ctx context.Context, pool *pgxpool.Pool, smtpSend func(to, subject, body string) error) error {
	rows, err := pool.Query(ctx, `
		SELECT m.id, m.name, m.target, m.ssl_expires_at, m.ssl_warn_days, t.id
		FROM monitors m
		JOIN tenants t ON t.id = m.tenant_id
		WHERE m.enabled
		  AND m.check_ssl
		  AND m.ssl_expires_at IS NOT NULL
		  AND m.ssl_expires_at < now() + (m.ssl_warn_days || ' days')::interval
		  AND (m.ssl_alerted_for IS NULL OR m.ssl_alerted_for <> m.ssl_expires_at)`)
	if err != nil {
		return err
	}
	defer rows.Close()

	type warning struct {
		monitorID, name, target, tenantID string
		expires                           time.Time
		warnDays                          int
	}
	var warns []warning
	for rows.Next() {
		var w warning
		if rows.Scan(&w.monitorID, &w.name, &w.target, &w.expires, &w.warnDays, &w.tenantID) == nil {
			warns = append(warns, w)
		}
	}

	for _, w := range warns {
		days := int(time.Until(w.expires).Hours() / 24)
		subject := fmt.Sprintf("[Pulse] TLS certificate for %s expires in %d days", w.name, days)
		body := fmt.Sprintf(
			"The TLS certificate for %s expires on %s (%d days from now).\n\n"+
				"Target: %s\n\nRenew it before then to avoid an outage.\n",
			w.name, w.expires.UTC().Format("2 Jan 2006"), days, w.target)

		addrs, err := verifiedEmails(ctx, pool, w.tenantID)
		if err != nil {
			slog.Error("cert warning recipients", "monitor", w.monitorID, "err", err)
			continue
		}
		sent := false
		for _, addr := range addrs {
			if err := smtpSend(addr, subject, body); err != nil {
				slog.Error("send cert warning", "to", addr, "err", err)
				continue
			}
			sent = true
		}
		if sent {
			if _, err := pool.Exec(ctx,
				`UPDATE monitors SET ssl_alerted_for=$2 WHERE id=$1`, w.monitorID, w.expires); err != nil {
				slog.Error("mark cert alerted", "monitor", w.monitorID, "err", err)
			}
			slog.Info("cert expiry warning sent", "monitor", w.name, "days", days)
		}
	}
	return nil
}

func verifiedEmails(ctx context.Context, pool *pgxpool.Pool, tenantID string) ([]string, error) {
	rows, err := pool.Query(ctx, `
		SELECT config->>'address' FROM alert_channels
		WHERE tenant_id=$1 AND type='email' AND enabled AND verified_at IS NOT NULL`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var addr string
		if rows.Scan(&addr) == nil && addr != "" {
			out = append(out, addr)
		}
	}
	return out, nil
}
