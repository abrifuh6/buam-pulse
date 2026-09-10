package http

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

// Public status page data. NO authentication — anyone with the slug can read it.
// That means the response must expose only what a customer intends to publish:
// display name, current state, and uptime. Never monitor IDs, target URLs,
// error strings, or anything else that leaks internal detail.

type publicMonitor struct {
	Name       string  `json:"name"`
	Status     string  `json:"status"`
	Uptime90d  float64 `json:"uptime_90d"`   // percent, 2dp
	AvgLatency *int    `json:"avg_latency_ms"`
}

type publicStatus struct {
	Tenant    string          `json:"tenant"`
	Monitors  []publicMonitor `json:"monitors"`
	UpdatedAt time.Time       `json:"updated_at"`
}

func (s *Server) PublicStatus(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	var tenantID, name string
	if err := s.DB.QueryRow(r.Context(),
		`SELECT id, name FROM tenants WHERE slug=$1`, slug).Scan(&tenantID, &name); err != nil {
		writeErr(w, 404, "status page not found")
		return
	}

	// Uptime = successful checks / total checks over the last 90 days, per monitor.
	// LEFT JOIN so a brand-new monitor with no results still appears.
	rows, err := s.DB.Query(r.Context(), `
		SELECT m.name,
		       m.status,
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
	defer rows.Close()

	out := publicStatus{Tenant: name, Monitors: []publicMonitor{}, UpdatedAt: time.Now().UTC()}
	for rows.Next() {
		var pm publicMonitor
		if rows.Scan(&pm.Name, &pm.Status, &pm.Uptime90d, &pm.AvgLatency) == nil {
			out.Monitors = append(out.Monitors, pm)
		}
	}

	// Cache for 30s: status pages get hammered during an outage, and the data
	// only changes once per check interval anyway.
	w.Header().Set("Cache-Control", "public, max-age=30")
	writeJSON(w, 200, out)
}
