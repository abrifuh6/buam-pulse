package http

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/abrifuh6/buam-pulse/internal/checks"
	"github.com/abrifuh6/buam-pulse/internal/plans"
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

	Keyword           *string    `json:"keyword"`
	KeywordPresent    bool       `json:"keyword_present"`
	CheckSSL          bool       `json:"check_ssl"`
	SSLWarnDays       int        `json:"ssl_warn_days"`
	SSLExpiresAt      *time.Time `json:"ssl_expires_at"`
	SSLIssuer         *string    `json:"ssl_issuer"`
	AlertDelaySeconds int        `json:"alert_delay_seconds"`
	Public            bool       `json:"public"`
	PublicName        *string    `json:"public_name"`
}

// Every query below filters by tenant_id from the token. A user can never
// read or change another customer's monitors, even by guessing an ID.

func (s *Server) ListMonitors(w http.ResponseWriter, r *http.Request) {
	c := claimsFrom(r)
	rows, err := s.DB.Query(r.Context(), `
		SELECT id, name, type, target, interval_seconds, timeout_seconds,
		       expected_status, enabled, status, created_at,
		       keyword, keyword_present, check_ssl, ssl_warn_days,
		       ssl_expires_at, ssl_issuer, alert_delay_seconds, public, public_name
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
			&m.ExpectedStatus, &m.Enabled, &m.Status, &m.CreatedAt,
			&m.Keyword, &m.KeywordPresent, &m.CheckSSL, &m.SSLWarnDays,
			&m.SSLExpiresAt, &m.SSLIssuer, &m.AlertDelaySeconds,
			&m.Public, &m.PublicName); err == nil {
			out = append(out, m)
		}
	}
	writeJSON(w, 200, out)
}

type monitorReq struct {
	Name              string `json:"name"`
	Type              string `json:"type"`
	Target            string `json:"target"`
	IntervalSeconds   int    `json:"interval_seconds"`
	TimeoutSeconds    int    `json:"timeout_seconds"`
	ExpectedStatus    int    `json:"expected_status"`
	Keyword           string `json:"keyword"`
	KeywordPresent    *bool  `json:"keyword_present"`
	CheckSSL          *bool  `json:"check_ssl"`
	SSLWarnDays       int    `json:"ssl_warn_days"`
	AlertDelaySeconds int    `json:"alert_delay_seconds"`
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
	// Rejects malformed targets AND private/loopback/metadata addresses:
	// Pulse fetches these URLs on a schedule, so an open target field is an
	// SSRF vector. See internal/checks/validate.go.
	if err := checks.ValidateTarget(in.Type, in.Target); err != nil {
		writeErr(w, 400, err.Error())
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
	// Default to "must contain" and to checking TLS: the safe, expected
	// behaviour when the caller says nothing.
	keywordPresent := true
	if in.KeywordPresent != nil {
		keywordPresent = *in.KeywordPresent
	}
	checkSSL := true
	if in.CheckSSL != nil {
		checkSSL = *in.CheckSSL
	}
	if in.SSLWarnDays <= 0 {
		in.SSLWarnDays = 14
	}

	// Limit check and insert share one transaction, with the tenant row locked,
	// so two concurrent requests cannot both pass a check at the boundary.
	tx, err := s.DB.Begin(r.Context())
	if err != nil {
		writeErr(w, 500, "db")
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()

	if err := plans.LockTenant(r.Context(), tx, c.TenantID); err != nil {
		writeErr(w, 500, "db")
		return
	}
	if err := plans.CheckMonitor(r.Context(), tx, c.TenantID, in.IntervalSeconds); err != nil {
		var le plans.LimitError
		if errors.As(err, &le) {
			writeErr(w, http.StatusPaymentRequired, le.Error())
			return
		}
		writeErr(w, http.StatusPaymentRequired, err.Error())
		return
	}

	var id string
	err = tx.QueryRow(r.Context(), `
		INSERT INTO monitors (tenant_id, name, type, target, interval_seconds,
		                      timeout_seconds, expected_status,
		                      keyword, keyword_present, check_ssl, ssl_warn_days,
		                      alert_delay_seconds)
		VALUES ($1,$2,$3,$4,$5,$6,$7, NULLIF($8,''), $9, $10, $11, $12) RETURNING id`,
		c.TenantID, in.Name, in.Type, in.Target, in.IntervalSeconds, in.TimeoutSeconds,
		in.ExpectedStatus, in.Keyword, keywordPresent, checkSSL, in.SSLWarnDays,
		in.AlertDelaySeconds).Scan(&id)
	if err != nil {
		slog.Error("create monitor", "err", err)
		writeErr(w, 400, "could not create monitor (interval must be 30–3600s)")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeErr(w, 500, "db")
		return
	}
	writeJSON(w, 201, map[string]string{"id": id})
}

type monitorUpdateReq struct {
	Name              *string `json:"name"`
	Target            *string `json:"target"`
	IntervalSeconds   *int    `json:"interval_seconds"`
	TimeoutSeconds    *int    `json:"timeout_seconds"`
	ExpectedStatus    *int    `json:"expected_status"`
	Enabled           *bool   `json:"enabled"`
	Keyword           *string `json:"keyword"`
	KeywordPresent    *bool   `json:"keyword_present"`
	CheckSSL          *bool   `json:"check_ssl"`
	SSLWarnDays       *int    `json:"ssl_warn_days"`
	AlertDelaySeconds *int    `json:"alert_delay_seconds"`
	Public            *bool   `json:"public"`
	PublicName        *string `json:"public_name"`
}

// UpdateMonitor is a partial update: pointer fields distinguish "not supplied"
// from "supplied as zero". Type is deliberately immutable — changing http↔tcp
// would invalidate the target and the historical results.
func (s *Server) UpdateMonitor(w http.ResponseWriter, r *http.Request) {
	c := claimsFrom(r)
	id := chi.URLParam(r, "id")

	var in monitorUpdateReq
	if err := decode(r, &in); err != nil {
		writeErr(w, 400, "bad request")
		return
	}

	var currentType string
	if err := s.DB.QueryRow(r.Context(),
		`SELECT type FROM monitors WHERE id=$1 AND tenant_id=$2`, id, c.TenantID).
		Scan(&currentType); err != nil {
		writeErr(w, 404, "not found")
		return
	}

	if in.Target != nil {
		if err := checks.ValidateTarget(currentType, *in.Target); err != nil {
			writeErr(w, 400, err.Error())
			return
		}
	}

	// The plan's minimum interval applies on edit as well as creation —
	// otherwise it is trivially bypassed by creating then editing.
	if in.IntervalSeconds != nil {
		l, err := plans.Get(r.Context(), s.DB, c.TenantID)
		if err == nil && *in.IntervalSeconds < l.MinInterval {
			writeErr(w, http.StatusPaymentRequired,
				fmt.Sprintf("your %s plan allows checks no more often than every %d seconds",
					l.Name, l.MinInterval))
			return
		}
	}

	// COALESCE keeps the existing value wherever the caller sent null.
	// Re-enabling schedules an immediate check rather than waiting a full
	// interval, so the user sees the effect of un-pausing straight away.
	tag, err := s.DB.Exec(r.Context(), `
		UPDATE monitors SET
		  name             = COALESCE($3, name),
		  target           = COALESCE($4, target),
		  interval_seconds = COALESCE($5, interval_seconds),
		  timeout_seconds  = COALESCE($6, timeout_seconds),
		  expected_status  = COALESCE($7, expected_status),
		  enabled          = COALESCE($8, enabled),
		  keyword          = COALESCE(NULLIF($9,''), keyword),
		  keyword_present  = COALESCE($10, keyword_present),
		  check_ssl        = COALESCE($11, check_ssl),
		  ssl_warn_days    = COALESCE($12, ssl_warn_days),
		  alert_delay_seconds = COALESCE($13, alert_delay_seconds),
		  public           = COALESCE($14, public),
		  public_name      = COALESCE(NULLIF($15,''), public_name),
		  next_run_at      = CASE WHEN $8 IS TRUE AND NOT enabled THEN now() ELSE next_run_at END,
		  updated_at       = now()
		WHERE id=$1 AND tenant_id=$2`,
		id, c.TenantID, in.Name, in.Target, in.IntervalSeconds,
		in.TimeoutSeconds, in.ExpectedStatus, in.Enabled,
		in.Keyword, in.KeywordPresent, in.CheckSSL, in.SSLWarnDays, in.AlertDelaySeconds,
		in.Public, in.PublicName)
	if err != nil {
		slog.Error("update monitor", "err", err)
		writeErr(w, 400, "could not update monitor (interval must be 30–3600s)")
		return
	}
	if tag.RowsAffected() == 0 {
		writeErr(w, 404, "not found")
		return
	}
	w.WriteHeader(204)
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

type seriesPoint struct {
	At         time.Time `json:"at"`
	Uptime     float64   `json:"uptime"` // percent for the bucket
	LatencyP50 *int      `json:"latency_p50"`
	LatencyP95 *int      `json:"latency_p95"`
	Checks     int       `json:"checks"`
}

type monitorDetail struct {
	Uptime24h  float64          `json:"uptime_24h"`
	Uptime7d   float64          `json:"uptime_7d"`
	Uptime30d  float64          `json:"uptime_30d"`
	AvgLatency *int             `json:"avg_latency_ms"`
	P95Latency *int             `json:"p95_latency_ms"`
	Series     []seriesPoint    `json:"series"`
	Incidents  []detailIncident `json:"incidents"`
}

type detailIncident struct {
	StartedAt  time.Time  `json:"started_at"`
	ResolvedAt *time.Time `json:"resolved_at"`
	Minutes    int        `json:"minutes"`
	Planned    bool       `json:"planned"`
	Notified   bool       `json:"notified"`
	Cause      *string    `json:"cause"`
}

// MonitorDetail returns everything the detail view needs in one request.
//
// The series is bucketed rather than returned raw: 30 days at 30-second
// intervals is 86,400 points, which no chart can draw and no browser should be
// asked to parse. Bucket width scales with the window so the chart always has
// roughly the same number of points.
func (s *Server) MonitorDetail(w http.ResponseWriter, r *http.Request) {
	c := claimsFrom(r)
	id := chi.URLParam(r, "id")

	var interval, bucket string
	switch r.URL.Query().Get("window") {
	case "7d":
		interval, bucket = "7 days", "1 hour"
	case "30d":
		interval, bucket = "30 days", "6 hours"
	default:
		interval, bucket = "24 hours", "15 minutes"
	}

	var exists bool
	if err := s.DB.QueryRow(r.Context(),
		`SELECT EXISTS (SELECT 1 FROM monitors WHERE id=$1 AND tenant_id=$2)`,
		id, c.TenantID).Scan(&exists); err != nil || !exists {
		writeErr(w, 404, "not found")
		return
	}

	out := monitorDetail{Series: []seriesPoint{}, Incidents: []detailIncident{}}

	// Three uptime windows in one round trip rather than three.
	if err := s.DB.QueryRow(r.Context(), `
		SELECT
		  COALESCE(ROUND(100.0 * count(*) FILTER (WHERE ok AND checked_at > now() - interval '24 hours')
		           / NULLIF(count(*) FILTER (WHERE checked_at > now() - interval '24 hours'),0), 2), 0)::float8,
		  COALESCE(ROUND(100.0 * count(*) FILTER (WHERE ok AND checked_at > now() - interval '7 days')
		           / NULLIF(count(*) FILTER (WHERE checked_at > now() - interval '7 days'),0), 2), 0)::float8,
		  COALESCE(ROUND(100.0 * count(*) FILTER (WHERE ok AND checked_at > now() - interval '30 days')
		           / NULLIF(count(*) FILTER (WHERE checked_at > now() - interval '30 days'),0), 2), 0)::float8,
		  AVG(latency_ms) FILTER (WHERE ok AND checked_at > now() - interval '24 hours')::int,
		  percentile_disc(0.95) WITHIN GROUP (ORDER BY latency_ms)
		    FILTER (WHERE ok AND checked_at > now() - interval '24 hours')
		FROM check_results WHERE monitor_id=$1`, id).
		Scan(&out.Uptime24h, &out.Uptime7d, &out.Uptime30d, &out.AvgLatency, &out.P95Latency); err != nil {
		slog.Error("monitor detail summary", "err", err)
		writeErr(w, 500, "db")
		return
	}

	// date_bin gives fixed-width buckets aligned to the epoch, so the same
	// timestamp always lands in the same bucket regardless of when the query
	// runs — the chart does not shift under you on refresh.
	rows, err := s.DB.Query(r.Context(), `
		SELECT date_bin($2::interval, checked_at, TIMESTAMPTZ '2000-01-01') AS at,
		       COALESCE(ROUND(100.0 * count(*) FILTER (WHERE ok) / count(*), 2), 0)::float8,
		       percentile_disc(0.50) WITHIN GROUP (ORDER BY latency_ms) FILTER (WHERE ok),
		       percentile_disc(0.95) WITHIN GROUP (ORDER BY latency_ms) FILTER (WHERE ok),
		       count(*)::int
		FROM check_results
		WHERE monitor_id=$1 AND checked_at > now() - $3::interval
		GROUP BY at ORDER BY at`, id, bucket, interval)
	if err != nil {
		slog.Error("monitor detail series", "err", err)
		writeErr(w, 500, "db")
		return
	}
	for rows.Next() {
		var p seriesPoint
		if rows.Scan(&p.At, &p.Uptime, &p.LatencyP50, &p.LatencyP95, &p.Checks) == nil {
			out.Series = append(out.Series, p)
		}
	}
	rows.Close()

	iRows, err := s.DB.Query(r.Context(), `
		SELECT started_at, resolved_at,
		       EXTRACT(EPOCH FROM (COALESCE(resolved_at, now()) - started_at))/60,
		       planned, notified_at IS NOT NULL, cause
		FROM incidents WHERE monitor_id=$1
		ORDER BY started_at DESC LIMIT 20`, id)
	if err == nil {
		for iRows.Next() {
			var d detailIncident
			var mins float64
			if iRows.Scan(&d.StartedAt, &d.ResolvedAt, &mins, &d.Planned, &d.Notified, &d.Cause) == nil {
				d.Minutes = int(mins)
				out.Incidents = append(out.Incidents, d)
			}
		}
		iRows.Close()
	}

	writeJSON(w, 200, out)
}

type incidentRow struct {
	ID         string     `json:"id"`
	Monitor    string     `json:"monitor"`
	MonitorID  string     `json:"monitor_id"`
	StartedAt  time.Time  `json:"started_at"`
	ResolvedAt *time.Time `json:"resolved_at"`
	Minutes    int        `json:"minutes"`
	Planned    bool       `json:"planned"`
	Notified   bool       `json:"notified"`
	Cause      *string    `json:"cause"`
}

// ListIncidents is the account-wide view: what has been breaking, across every
// monitor. Ongoing incidents sort first regardless of age, because a current
// outage matters more than a longer one that ended yesterday.
func (s *Server) ListIncidents(w http.ResponseWriter, r *http.Request) {
	c := claimsFrom(r)

	rows, err := s.DB.Query(r.Context(), `
		SELECT i.id, m.name, m.id, i.started_at, i.resolved_at,
		       EXTRACT(EPOCH FROM (COALESCE(i.resolved_at, now()) - i.started_at))/60,
		       i.planned, i.notified_at IS NOT NULL, i.cause
		FROM incidents i
		JOIN monitors m ON m.id = i.monitor_id
		WHERE i.tenant_id = $1
		ORDER BY (i.resolved_at IS NULL) DESC, i.started_at DESC
		LIMIT 100`, c.TenantID)
	if err != nil {
		writeErr(w, 500, "db")
		return
	}
	defer rows.Close()

	out := []incidentRow{}
	for rows.Next() {
		var ir incidentRow
		var mins float64
		if rows.Scan(&ir.ID, &ir.Monitor, &ir.MonitorID, &ir.StartedAt, &ir.ResolvedAt,
			&mins, &ir.Planned, &ir.Notified, &ir.Cause) == nil {
			ir.Minutes = int(mins)
			out = append(out, ir)
		}
	}
	writeJSON(w, 200, out)
}
