package http

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

type Monitor struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Type            string    `json:"type"`
	Target          string    `json:"target"`
	IntervalSeconds int       `json:"interval_seconds"`
	TimeoutSeconds  int       `json:"timeout_seconds"`
	ExpectedStatus  *int      `json:"expected_status,omitempty"`
	Enabled         bool      `json:"enabled"`
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"created_at"`
}

// Every query below filters by tenant_id from the token. A user can never
// read or change another customer's monitors, even by guessing an ID.

func (s *Server) ListMonitors(w http.ResponseWriter, r *http.Request) {
	c := claimsFrom(r)
	rows, err := s.DB.Query(r.Context(), `
		SELECT id, name, type, target, interval_seconds, timeout_seconds, expected_status, enabled, status, created_at
		FROM monitors WHERE tenant_id=$1 ORDER BY created_at`, c.TenantID)
	if err != nil {
		writeErr(w, 500, "db")
		return
	}
	defer rows.Close()
	out := []Monitor{}
	for rows.Next() {
		var m Monitor
		if err := rows.Scan(&m.ID, &m.Name, &m.Type, &m.Target, &m.IntervalSeconds, &m.TimeoutSeconds,
			&m.ExpectedStatus, &m.Enabled, &m.Status, &m.CreatedAt); err == nil {
			out = append(out, m)
		}
	}
	writeJSON(w, 200, out)
}

type monitorReq struct {
	Name            string `json:"name"`
	Type            string `json:"type"`
	Target          string `json:"target"`
	IntervalSeconds int    `json:"interval_seconds"`
	TimeoutSeconds  int    `json:"timeout_seconds"`
	ExpectedStatus  int    `json:"expected_status"`
}

func (s *Server) CreateMonitor(w http.ResponseWriter, r *http.Request) {
	c := claimsFrom(r)
	var in monitorReq
	if err := decode(r, &in); err != nil || in.Name == "" || in.Target == "" {
		writeErr(w, 400, "name and target required")
		return
	}
	if in.Type != "http" && in.Type != "tcp" {
		writeErr(w, 400, "type must be http or tcp")
		return
	}
	if in.IntervalSeconds == 0 {
		in.IntervalSeconds = 60
	}
	if in.TimeoutSeconds == 0 {
		in.TimeoutSeconds = 10
	}
	if in.ExpectedStatus == 0 {
		in.ExpectedStatus = 200
	}
	var id string
	err := s.DB.QueryRow(r.Context(), `
		INSERT INTO monitors (tenant_id, name, type, target, interval_seconds, timeout_seconds, expected_status)
		VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id`,
		c.TenantID, in.Name, in.Type, in.Target, in.IntervalSeconds, in.TimeoutSeconds, in.ExpectedStatus).Scan(&id)
	if err != nil {
		writeErr(w, 400, "could not create monitor (interval must be 30–3600s)")
		return
	}
	writeJSON(w, 201, map[string]string{"id": id})
}

func (s *Server) DeleteMonitor(w http.ResponseWriter, r *http.Request) {
	c := claimsFrom(r)
	tag, err := s.DB.Exec(r.Context(),
		`DELETE FROM monitors WHERE id=$1 AND tenant_id=$2`, chi.URLParam(r, "id"), c.TenantID)
	if err != nil || tag.RowsAffected() == 0 {
		writeErr(w, 404, "not found")
		return
	}
	w.WriteHeader(204)
}

type CheckResult struct {
	CheckedAt  time.Time `json:"checked_at"`
	OK         bool      `json:"ok"`
	StatusCode *int      `json:"status_code"`
	LatencyMs  *int      `json:"latency_ms"`
	Error      *string   `json:"error"`
	Region     string    `json:"region"`
}

func (s *Server) MonitorResults(w http.ResponseWriter, r *http.Request) {
	c := claimsFrom(r)
	rows, err := s.DB.Query(r.Context(), `
		SELECT checked_at, ok, status_code, latency_ms, error, region
		FROM check_results WHERE monitor_id=$1 AND tenant_id=$2
		ORDER BY checked_at DESC LIMIT 100`, chi.URLParam(r, "id"), c.TenantID)
	if err != nil {
		writeErr(w, 500, "db")
		return
	}
	defer rows.Close()
	out := []CheckResult{}
	for rows.Next() {
		var cr CheckResult
		if rows.Scan(&cr.CheckedAt, &cr.OK, &cr.StatusCode, &cr.LatencyMs, &cr.Error, &cr.Region) == nil {
			out = append(out, cr)
		}
	}
	writeJSON(w, 200, out)
}