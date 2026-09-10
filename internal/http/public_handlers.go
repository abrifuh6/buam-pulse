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

	// Daily uptime per monitor. generate_series produces every day in the
	// window so gaps stay visible rather than silently collapsing the bar.
	// NOTE: this is one query per monitor (N+1). Acceptable at current scale
	// behind the 30s cache; the fix is a single GROUP BY monitor_id, day.
	for _, rr := range summaries {
		daily := make([]float64, 0, 90)
		dRows, err := s.DB.Query(r.Context(), `
			SELECT COALESCE(
			         ROUND(100.0 * COUNT(*) FILTER (WHERE cr.ok) / NULLIF(COUNT(cr.id),0), 2),
			         -1)::float8
			FROM generate_series(
			       (now() - interval '89 days')::date, now()::date, interval '1 day') AS d(day)
			LEFT JOIN check_results cr
			       ON cr.monitor_id = $1
			      AND cr.checked_at >= d.day
			      AND cr.checked_at <  d.day + interval '1 day'
			GROUP BY d.day
			ORDER BY d.day`, rr.id)
		if err == nil {
			for dRows.Next() {
				var v float64
				if dRows.Scan(&v) == nil {
					daily = append(daily, v)
				}
			}
			dRows.Close()
		}
		rr.pm.Daily = daily
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
