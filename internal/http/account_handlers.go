package http

import (
	"net/http"
)

// Account is the "who am I and where am I" summary the header needs. One
// request instead of three, because it renders on every page load.
type Account struct {
	Tenant struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
	} `json:"tenant"`
	User struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	} `json:"user"`
	Plan struct {
		Code string `json:"code"`
		Name string `json:"name"`
		// Whether this tenant has ever checked out. Without a Stripe customer
		// the portal cannot be opened, so the UI must offer Checkout instead.
		HasBilling        bool    `json:"has_billing"`
		Status            *string `json:"subscription_status"`
		CancelAtPeriodEnd bool    `json:"cancel_at_period_end"`
	} `json:"plan"`
	Counts struct {
		Members  int `json:"members"`
		Monitors int `json:"monitors"`
	} `json:"counts"`
	StatusURL string `json:"status_url"`
}

func (s *Server) GetAccount(w http.ResponseWriter, r *http.Request) {
	c := claimsFrom(r)

	var a Account
	err := s.DB.QueryRow(r.Context(), `
		SELECT t.name, t.slug, u.email, u.role, p.code, p.name,
		       t.stripe_customer_id IS NOT NULL, t.subscription_status, t.cancel_at_period_end, t.cancel_at_period_end,
		       (SELECT count(*) FROM users    WHERE tenant_id = t.id),
		       (SELECT count(*) FROM monitors WHERE tenant_id = t.id)
		FROM tenants t
		JOIN plans p ON p.code = t.plan_code
		JOIN users u ON u.id = $2
		WHERE t.id = $1`, c.TenantID, c.UserID).
		Scan(&a.Tenant.Name, &a.Tenant.Slug, &a.User.Email, &a.User.Role,
			&a.Plan.Code, &a.Plan.Name, &a.Plan.HasBilling, &a.Plan.Status, &a.Plan.CancelAtPeriodEnd, &a.Plan.CancelAtPeriodEnd,
			&a.Counts.Members, &a.Counts.Monitors)
	if err != nil {
		writeErr(w, 500, "db")
		return
	}

	// The tenant's own public status page, so they can copy or share the link
	// without having to construct it from the slug themselves.
	a.StatusURL = s.Cfg.StatusURL + "/" + a.Tenant.Slug

	writeJSON(w, 200, a)
}
