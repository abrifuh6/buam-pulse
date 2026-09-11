package http

import (
	"net/http"

	"github.com/abrifuh6/buam-pulse/internal/plans"
)

type planResponse struct {
	Plan  plans.Limits `json:"plan"`
	Usage plans.Usage  `json:"usage"`
}

// CurrentPlan powers the usage display and the upgrade prompt. Any member can
// read it: knowing you're at 3 of 3 monitors isn't privileged information.
func (s *Server) CurrentPlan(w http.ResponseWriter, r *http.Request) {
	c := claimsFrom(r)

	l, err := plans.Get(r.Context(), s.DB, c.TenantID)
	if err != nil {
		writeErr(w, 500, "db")
		return
	}
	u, err := plans.GetUsage(r.Context(), s.DB, c.TenantID)
	if err != nil {
		writeErr(w, 500, "db")
		return
	}
	writeJSON(w, 200, planResponse{Plan: l, Usage: u})
}

// ListPlans is public: the pricing table needs it before anyone signs up.
func (s *Server) ListPlans(w http.ResponseWriter, r *http.Request) {
	rows, err := s.DB.Query(r.Context(), `
		SELECT code, name, max_monitors, min_interval, max_members,
		       max_channels, retention_days, price_cents,
		       price_cents_quarterly, price_cents_yearly
		FROM plans ORDER BY sort_order`)
	if err != nil {
		writeErr(w, 500, "db")
		return
	}
	defer rows.Close()

	out := []plans.Limits{}
	for rows.Next() {
		var l plans.Limits
		if rows.Scan(&l.Code, &l.Name, &l.MaxMonitors, &l.MinInterval,
			&l.MaxMembers, &l.MaxChannels, &l.RetentionDays, &l.PriceCents,
			&l.PriceQuarterly, &l.PriceYearly) == nil {
			out = append(out, l)
		}
	}
	writeJSON(w, 200, out)
}
