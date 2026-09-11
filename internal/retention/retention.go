// Package retention aggregates raw check results into daily summaries and
// removes raw rows past each plan's retention window.
//
// The arithmetic that motivates this: one monitor at 30s intervals writes
// 2,880 rows a day. A hundred monitors is 288,000 a day and 26 million over
// 90 days. Scanning that range to draw a status page gets slow, and storing it
// forever gets expensive. Rolling up to one row per monitor per day reduces it
// by three orders of magnitude while keeping everything a status page needs.
package retention

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Rollup aggregates every completed day that has not been rolled up yet.
// Today is deliberately excluded: it is still accumulating, and a partial
// aggregate would be wrong until midnight passes.
func Rollup(ctx context.Context, pool *pgxpool.Pool) (int64, error) {
	var last *time.Time
	if err := pool.QueryRow(ctx,
		`SELECT last_rollup_day FROM retention_state WHERE id=1`).Scan(&last); err != nil {
		return 0, err
	}

	// Start the day after the last completed rollup, or from the oldest raw
	// data if this has never run.
	from := "(SELECT COALESCE(MIN(checked_at)::date, CURRENT_DATE) FROM check_results)"
	args := []any{}
	if last != nil {
		from = "$1::date + 1"
		args = append(args, *last)
	}

	// percentile_disc returns an actual observed value rather than an
	// interpolation, which is the right choice for latency: p95 should be a
	// request that really happened.
	sql := `
		WITH days AS (
			SELECT generate_series(` + from + `, CURRENT_DATE - 1, interval '1 day')::date AS day
		),
		agg AS (
			SELECT cr.monitor_id,
			       cr.tenant_id,
			       d.day,
			       count(*)                              AS checks_total,
			       count(*) FILTER (WHERE cr.ok)         AS checks_ok,
			       percentile_disc(0.50) WITHIN GROUP (ORDER BY cr.latency_ms)
			         FILTER (WHERE cr.ok)                AS p50,
			       percentile_disc(0.95) WITHIN GROUP (ORDER BY cr.latency_ms)
			         FILTER (WHERE cr.ok)                AS p95,
			       percentile_disc(0.99) WITHIN GROUP (ORDER BY cr.latency_ms)
			         FILTER (WHERE cr.ok)                AS p99,
			       max(cr.latency_ms) FILTER (WHERE cr.ok) AS lmax
			FROM days d
			JOIN check_results cr
			  ON cr.checked_at >= d.day
			 AND cr.checked_at <  d.day + interval '1 day'
			GROUP BY cr.monitor_id, cr.tenant_id, d.day
		)
		INSERT INTO daily_uptime
		  (monitor_id, tenant_id, day, checks_total, checks_ok,
		   latency_p50, latency_p95, latency_p99, latency_max)
		SELECT monitor_id, tenant_id, day, checks_total, checks_ok, p50, p95, p99, lmax
		FROM agg
		-- Re-running is safe: a day already rolled up is overwritten with the
		-- same numbers rather than duplicated.
		ON CONFLICT (monitor_id, day) DO UPDATE SET
		  checks_total = EXCLUDED.checks_total,
		  checks_ok    = EXCLUDED.checks_ok,
		  latency_p50  = EXCLUDED.latency_p50,
		  latency_p95  = EXCLUDED.latency_p95,
		  latency_p99  = EXCLUDED.latency_p99,
		  latency_max  = EXCLUDED.latency_max`

	tag, err := pool.Exec(ctx, sql, args...)
	if err != nil {
		return 0, err
	}

	// Downtime minutes come from incidents, which already record exact spans —
	// more accurate than inferring it from missed checks.
	if _, err := pool.Exec(ctx, `
		UPDATE daily_uptime du SET downtime_minutes = COALESCE(sub.mins, 0)
		FROM (
			SELECT i.monitor_id,
			       gs.day::date AS day,
			       SUM(EXTRACT(EPOCH FROM (
			             LEAST(COALESCE(i.resolved_at, now()), gs.day + interval '1 day')
			           - GREATEST(i.started_at, gs.day)
			       )) / 60)::int AS mins
			FROM incidents i
			CROSS JOIN LATERAL generate_series(
				i.started_at::date,
				COALESCE(i.resolved_at, now())::date,
				interval '1 day') AS gs(day)
			GROUP BY i.monitor_id, gs.day
		) sub
		WHERE du.monitor_id = sub.monitor_id AND du.day = sub.day`); err != nil {
		slog.Warn("downtime rollup", "err", err)
	}

	if _, err := pool.Exec(ctx, `
		UPDATE retention_state SET last_rollup_day = CURRENT_DATE - 1, last_run_at = now()
		WHERE id=1`); err != nil {
		return tag.RowsAffected(), err
	}

	return tag.RowsAffected(), nil
}

// Prune deletes raw check results older than each tenant's plan allows.
// Deleting in batches keeps each transaction short: one enormous DELETE would
// hold locks and bloat the write-ahead log, and on a busy database that is how
// a maintenance job turns into an outage.
func Prune(ctx context.Context, pool *pgxpool.Pool, batchSize int) (int64, error) {
	var total int64
	for {
		tag, err := pool.Exec(ctx, `
			DELETE FROM check_results
			WHERE id IN (
				SELECT cr.id
				FROM check_results cr
				JOIN tenants t ON t.id = cr.tenant_id
				JOIN plans   p ON p.code = t.plan_code
				WHERE cr.checked_at < now() - (p.retention_days || ' days')::interval
				LIMIT $1
			)`, batchSize)
		if err != nil {
			return total, err
		}
		n := tag.RowsAffected()
		total += n
		if n < int64(batchSize) {
			break
		}
		// Yield between batches so normal traffic is not starved.
		select {
		case <-ctx.Done():
			return total, ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}

	if total > 0 {
		_, _ = pool.Exec(ctx,
			`UPDATE retention_state SET rows_deleted = rows_deleted + $1 WHERE id=1`, total)
	}
	return total, nil
}
