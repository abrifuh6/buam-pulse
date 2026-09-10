package http

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/subscription"
)

// Export and deletion exist because customers have a legal right to both
// (GDPR art. 15 and 17, PIPEDA principles 9 and 4.5). They are also just good
// product behaviour: a customer who can leave easily is more willing to join.

type exportMonitor struct {
	Name            string           `json:"name"`
	Type            string           `json:"type"`
	Target          string           `json:"target"`
	IntervalSeconds int              `json:"interval_seconds"`
	Enabled         bool             `json:"enabled"`
	Status          string           `json:"status"`
	CreatedAt       time.Time        `json:"created_at"`
	Results         []exportResult   `json:"check_results"`
	Incidents       []exportIncident `json:"incidents"`
}

type exportResult struct {
	CheckedAt  time.Time `json:"checked_at"`
	OK         bool      `json:"ok"`
	StatusCode *int      `json:"status_code"`
	LatencyMs  *int      `json:"latency_ms"`
	Error      *string   `json:"error"`
	Region     string    `json:"region"`
}

type exportIncident struct {
	StartedAt  time.Time  `json:"started_at"`
	ResolvedAt *time.Time `json:"resolved_at"`
	Cause      *string    `json:"cause"`
}

type exportMember struct {
	Email    string    `json:"email"`
	Role     string    `json:"role"`
	JoinedAt time.Time `json:"joined_at"`
}

type exportChannel struct {
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	Verified  bool      `json:"verified"`
	CreatedAt time.Time `json:"created_at"`
}

type accountExport struct {
	ExportedAt time.Time       `json:"exported_at"`
	Tenant     map[string]any  `json:"tenant"`
	Members    []exportMember  `json:"members"`
	Channels   []exportChannel `json:"alert_channels"`
	Monitors   []exportMonitor `json:"monitors"`
}

// ExportData returns everything the tenant owns as one JSON document.
// Deliberately excludes: password hashes, auth tokens, Stripe identifiers, and
// alert channel configs (a Slack webhook URL is a live credential). The point
// is the customer's own data, not our internal state.
func (s *Server) ExportData(w http.ResponseWriter, r *http.Request) {
	c := claimsFrom(r)

	out := accountExport{
		ExportedAt: time.Now().UTC(),
		Members:    []exportMember{},
		Channels:   []exportChannel{},
		Monitors:   []exportMonitor{},
	}

	var name, slug, planCode string
	var createdAt time.Time
	if err := s.DB.QueryRow(r.Context(),
		`SELECT name, slug, plan_code, created_at FROM tenants WHERE id=$1`, c.TenantID).
		Scan(&name, &slug, &planCode, &createdAt); err != nil {
		writeErr(w, 500, "db")
		return
	}
	out.Tenant = map[string]any{
		"name": name, "slug": slug, "plan": planCode, "created_at": createdAt,
	}

	mRows, err := s.DB.Query(r.Context(), `
		SELECT email, role, created_at FROM users WHERE tenant_id=$1 ORDER BY created_at`, c.TenantID)
	if err == nil {
		for mRows.Next() {
			var m exportMember
			if mRows.Scan(&m.Email, &m.Role, &m.JoinedAt) == nil {
				out.Members = append(out.Members, m)
			}
		}
		mRows.Close()
	}

	cRows, err := s.DB.Query(r.Context(), `
		SELECT name, type, verified_at IS NOT NULL, created_at
		FROM alert_channels WHERE tenant_id=$1 ORDER BY created_at`, c.TenantID)
	if err == nil {
		for cRows.Next() {
			var ch exportChannel
			if cRows.Scan(&ch.Name, &ch.Type, &ch.Verified, &ch.CreatedAt) == nil {
				out.Channels = append(out.Channels, ch)
			}
		}
		cRows.Close()
	}

	type monRow struct {
		id string
		m  exportMonitor
	}
	var mons []monRow
	monRows, err := s.DB.Query(r.Context(), `
		SELECT id, name, type, target, interval_seconds, enabled, status, created_at
		FROM monitors WHERE tenant_id=$1 ORDER BY created_at`, c.TenantID)
	if err != nil {
		writeErr(w, 500, "db")
		return
	}
	for monRows.Next() {
		var mr monRow
		if monRows.Scan(&mr.id, &mr.m.Name, &mr.m.Type, &mr.m.Target,
			&mr.m.IntervalSeconds, &mr.m.Enabled, &mr.m.Status, &mr.m.CreatedAt) == nil {
			mons = append(mons, mr)
		}
	}
	monRows.Close()

	for _, mr := range mons {
		mr.m.Results = []exportResult{}
		mr.m.Incidents = []exportIncident{}

		// Capped per monitor. An unbounded export of millions of rows would
		// exhaust memory and time out; a customer who needs the full history
		// can ask, and a production system would generate it asynchronously
		// and email a download link.
		rRows, err := s.DB.Query(r.Context(), `
			SELECT checked_at, ok, status_code, latency_ms, error, region
			FROM check_results WHERE monitor_id=$1
			ORDER BY checked_at DESC LIMIT 10000`, mr.id)
		if err == nil {
			for rRows.Next() {
				var res exportResult
				if rRows.Scan(&res.CheckedAt, &res.OK, &res.StatusCode,
					&res.LatencyMs, &res.Error, &res.Region) == nil {
					mr.m.Results = append(mr.m.Results, res)
				}
			}
			rRows.Close()
		}

		iRows, err := s.DB.Query(r.Context(), `
			SELECT started_at, resolved_at, cause FROM incidents
			WHERE monitor_id=$1 ORDER BY started_at DESC`, mr.id)
		if err == nil {
			for iRows.Next() {
				var inc exportIncident
				if iRows.Scan(&inc.StartedAt, &inc.ResolvedAt, &inc.Cause) == nil {
					mr.m.Incidents = append(mr.m.Incidents, inc)
				}
			}
			iRows.Close()
		}

		out.Monitors = append(out.Monitors, mr.m)
	}

	filename := fmt.Sprintf("pulse-export-%s-%s.json", slug, time.Now().Format("2006-01-02"))
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
}

type deleteAccountReq struct {
	// Typing the organization name is the confirmation. A checkbox is too easy
	// to click through for an action that destroys everything irreversibly.
	Confirm string `json:"confirm"`
}

// DeleteAccount removes the tenant and everything belonging to it. Owner only.
func (s *Server) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	c := claimsFrom(r)

	var in deleteAccountReq
	if err := decode(r, &in); err != nil {
		writeErr(w, 400, "bad request")
		return
	}

	var name, slug string
	var subID *string
	if err := s.DB.QueryRow(r.Context(),
		`SELECT name, slug, stripe_subscription_id FROM tenants WHERE id=$1`, c.TenantID).
		Scan(&name, &slug, &subID); err != nil {
		writeErr(w, 500, "db")
		return
	}
	if in.Confirm != name {
		writeErr(w, 400, "type the organization name exactly to confirm")
		return
	}

	var email string
	_ = s.DB.QueryRow(r.Context(), `SELECT email FROM users WHERE id=$1`, c.UserID).Scan(&email)

	// Record the request before destroying anything: if the delete fails
	// halfway, this row is what proves what was asked for.
	var logID int64
	if err := s.DB.QueryRow(r.Context(), `
		INSERT INTO deletion_log (tenant_slug, tenant_name, requested_by)
		VALUES ($1,$2,$3) RETURNING id`, slug, name, email).Scan(&logID); err != nil {
		writeErr(w, 500, "db")
		return
	}

	// Cancel billing FIRST. Deleting the tenant while a subscription is live
	// would keep charging a customer whose account no longer exists.
	if subID != nil && *subID != "" {
		if _, err := subscription.Cancel(*subID, &stripe.SubscriptionCancelParams{}); err != nil {
			slog.Error("cancel subscription on delete", "sub", *subID, "err", err)
			writeErr(w, 502, "could not cancel your subscription — please contact support")
			return
		}
	}

	// Counts for the log, taken before the delete.
	var monitors int
	var results int64
	_ = s.DB.QueryRow(r.Context(), `
		SELECT (SELECT count(*) FROM monitors      WHERE tenant_id=$1),
		       (SELECT count(*) FROM check_results WHERE tenant_id=$1)`, c.TenantID).
		Scan(&monitors, &results)

	// One statement. Every child table declares ON DELETE CASCADE against
	// tenants, so the database removes users, monitors, results, incidents,
	// channels, notifications and invitations atomically. Deleting them by
	// hand in the right order would be both slower and easy to get wrong.
	if _, err := s.DB.Exec(r.Context(), `DELETE FROM tenants WHERE id=$1`, c.TenantID); err != nil {
		slog.Error("delete tenant", "tenant", slug, "err", err)
		writeErr(w, 500, "could not delete account")
		return
	}

	_, _ = s.DB.Exec(r.Context(), `
		UPDATE deletion_log SET completed_at=now(), monitors_removed=$2, results_removed=$3
		WHERE id=$1`, logID, monitors, results)

	slog.Info("account deleted", "tenant", slug, "monitors", monitors, "results", results)
	writeJSON(w, 200, map[string]string{"status": "account deleted"})
}
