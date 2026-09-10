// Package alerting turns check results into incidents, and incidents into
// queued notifications. Kept out of the worker's main loop so the transition
// rules are testable and live in one place.
package alerting

import (
	"context"
	"log/slog"

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
