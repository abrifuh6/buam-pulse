package http

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

// Public status page data. NO authentication — anyone with the slug can read it.
// The response exposes only what a customer intends to publish: display name,
// current state, uptime, and incident timing. Never monitor IDs, target URLs,
// or raw error strings, which can leak internal hostnames and stack details.

type publicMonitor struct {
	Name       string  `json:"name"`
	Status     string  `json:"status"`
	Uptime90d  float64 `json:"uptime_90d"`
	AvgLatency *int    `json:"avg_latency_ms"`
	// One entry per day for the last 90 days, oldest first: the uptime bar
	// every status page has. -1 means no data recorded that day.
	Daily []float64 `json:"daily_uptime"`
}

type publicIncident struct {
	Monitor    string     `json:"monitor"`
	StartedAt  time.Time  `json:"started_at"`
	ResolvedAt *time.Time `json:"resolved_at"`
	Minutes    int        `json:"minutes"`
}

type publicStatus struct {
	Tenant    string           `json:"tenant"`
	Monitors  []publicMonitor  `json:"monitors"`
	Active    []publicIncident `json:"active_incidents"`
	Recent    []publicIncident `json:"recent_incidents"`
	UpdatedAt time.Time        `json:"updated_at"`
}

func (s *Server) PublicStatus(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	var tenantID, name string
	if err := s.DB.QueryRow(r.Context(),
		`SELECT id, name FROM tenants WHERE slug=$1`, slug).Scan(&tenantID, &name); err != nil {
		writeErr(w, 404, "status page not found")
		return
	}

	out := publicStatus{
		Tenant:    name,
		Monitors:  []publicMonitor{},
		Active:    []publicIncident{},
		Recent:    []publicIncident{},
		UpdatedAt: time.Now().UTC(),
	}

	// Per-monitor summary. Paused monitors are excluded: a customer who paused
	// a check has said they don't want it reported.
	rows, err := s.DB.Query(r.Context(), `
		SELECT m.id, m.name, m.status,
		       COALESCE(ROUND(100.0 * COUNT(*) FILTER (WHERE cr.ok) / NULLIF(COUNT(cr.id),0), 2), 0)::float8,
		       AVG(cr.latency_ms) FILTER (WHERE cr.ok)::int
		FROM monitors m
		LEFT JOIN check_results cr
		       ON cr.monitor_id = m.id
		      AND cr.checked_at > now() - interval '90 days'
		WHERE m.tenant_id = $1 AND m.enabled
		GROUP BY m.id, m.name, m.status
		ORDER BY m.name`, tenantID)
	if err != nil {
		writeErr(w, 500, "db")
		return
	}

	type row struct {
		id string
		pm publicMonitor
	}
	var summaries []row
	for rows.Next() {
		var rr row
		if rows.Scan(&rr.id, &rr.pm.Name, &rr.pm.Status, &rr.pm.Uptime90d, &rr.pm.AvgLatency) == nil {
			summaries = append(summaries, rr)
		}
	}
	rows.Close()

	// Daily uptime for every monitor in ONE query, reading the rollup table
	// rather than scanning raw results. This replaces the previous per-monitor
	// query (an N+1) and keeps the cost flat as history grows: 90 days of one
	// monitor is 90 rollup rows instead of a quarter of a million raw ones.
	//
	// Today is not in the rollup yet — it is still accumulating — so today's
	// figure is computed live from raw results and appended.
	daily := map[string][]float64{}
	{
		ids := make([]string, 0, len(summaries))
		for _, rr := range summaries {
			ids = append(ids, rr.id)
			daily[rr.id] = make([]float64, 0, 90)
		}

		dRows, err := s.DB.Query(r.Context(), `
			WITH days AS (
				SELECT generate_series(
					(CURRENT_DATE - 89), CURRENT_DATE, interval '1 day')::date AS day
			),
			mons AS (SELECT unnest($1::uuid[]) AS monitor_id)
			SELECT m.monitor_id, d.day,
			       CASE
			         WHEN du.checks_total > 0
			           THEN ROUND(100.0 * du.checks_ok / du.checks_total, 2)::float8
			         WHEN today.total > 0
			           THEN ROUND(100.0 * today.ok / today.total, 2)::float8
			         ELSE -1
			       END AS uptime
			FROM mons m
			CROSS JOIN days d
			LEFT JOIN daily_uptime du
			       ON du.monitor_id = m.monitor_id AND du.day = d.day
			LEFT JOIN LATERAL (
			       SELECT count(*) AS total, count(*) FILTER (WHERE ok) AS ok
			       FROM check_results cr
			       WHERE cr.monitor_id = m.monitor_id
			         AND d.day = CURRENT_DATE
			         AND cr.checked_at >= CURRENT_DATE
			) today ON true
			ORDER BY m.monitor_id, d.day`, ids)
		if err == nil {
			for dRows.Next() {
				var id string
				var day time.Time
				var v float64
				if dRows.Scan(&id, &day, &v) == nil {
					daily[id] = append(daily[id], v)
				}
			}
			dRows.Close()
		}
	}

	for _, rr := range summaries {
		rr.pm.Daily = daily[rr.id]
		out.Monitors = append(out.Monitors, rr.pm)
	}

	// Incidents: anything open now, plus what happened in the last 90 days.
	iRows, err := s.DB.Query(r.Context(), `
		SELECT m.name, i.started_at, i.resolved_at,
		       EXTRACT(EPOCH FROM (COALESCE(i.resolved_at, now()) - i.started_at))/60
		FROM incidents i
		JOIN monitors m ON m.id = i.monitor_id
		WHERE i.tenant_id = $1
		  AND i.started_at > now() - interval '90 days'
		  AND m.enabled
		ORDER BY i.started_at DESC
		LIMIT 50`, tenantID)
	if err == nil {
		for iRows.Next() {
			var pi publicIncident
			var mins float64
			if iRows.Scan(&pi.Monitor, &pi.StartedAt, &pi.ResolvedAt, &mins) == nil {
				pi.Minutes = int(mins)
				if pi.ResolvedAt == nil {
					out.Active = append(out.Active, pi)
				} else {
					out.Recent = append(out.Recent, pi)
				}
			}
		}
		iRows.Close()
	}

	// Cached briefly: status pages get hammered during an outage, and the data
	// only changes once per check interval anyway.
	w.Header().Set("Cache-Control", "public, max-age=30")
	writeJSON(w, 200, out)
}
