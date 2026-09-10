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
		       (SELECT count(*) FROM users    WHERE tenant_id = t.id),
		       (SELECT count(*) FROM monitors WHERE tenant_id = t.id)
		FROM tenants t
		JOIN plans p ON p.code = t.plan_code
		JOIN users u ON u.id = $2
		WHERE t.id = $1`, c.TenantID, c.UserID).
		Scan(&a.Tenant.Name, &a.Tenant.Slug, &a.User.Email, &a.User.Role,
			&a.Plan.Code, &a.Plan.Name, &a.Counts.Members, &a.Counts.Monitors)
	if err != nil {
		writeErr(w, 500, "db")
		return
	}

	// The tenant's own public status page, so they can copy or share the link
	// without having to construct it from the slug themselves.
	a.StatusURL = s.Cfg.StatusURL + "/" + a.Tenant.Slug

	writeJSON(w, 200, a)
}
