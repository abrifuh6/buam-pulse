package http

import (
	"log/slog"
	"net/http"
	"net/url"

	"github.com/abrifuh6/buam-pulse/internal/plans"
)

type StatusPageSettings struct {
	Title        *string `json:"title"`
	Description  *string `json:"description"`
	SupportURL   *string `json:"support_url"`
	HideBranding bool    `json:"hide_branding"`
	// Whether the current plan permits hiding branding, so the UI can explain
	// why the toggle is unavailable rather than silently ignoring it.
	CanHideBranding bool   `json:"can_hide_branding"`
	Slug            string `json:"slug"`
	PublicURL       string `json:"public_url"`
}

func (s *Server) GetStatusPageSettings(w http.ResponseWriter, r *http.Request) {
	c := claimsFrom(r)

	var out StatusPageSettings
	if err := s.DB.QueryRow(r.Context(), `
		SELECT status_title, status_description, status_support_url,
		       status_hide_branding, slug
		FROM tenants WHERE id=$1`, c.TenantID).
		Scan(&out.Title, &out.Description, &out.SupportURL,
			&out.HideBranding, &out.Slug); err != nil {
		writeErr(w, 500, "db")
		return
	}

	l, err := plans.Get(r.Context(), s.DB, c.TenantID)
	if err == nil {
		out.CanHideBranding = l.PriceCents > 0
	}
	out.PublicURL = s.Cfg.StatusURL + "/" + out.Slug

	writeJSON(w, 200, out)
}

type statusSettingsReq struct {
	Title        *string `json:"title"`
	Description  *string `json:"description"`
	SupportURL   *string `json:"support_url"`
	HideBranding *bool   `json:"hide_branding"`
}

func (s *Server) UpdateStatusPageSettings(w http.ResponseWriter, r *http.Request) {
	c := claimsFrom(r)

	var in statusSettingsReq
	if err := decode(r, &in); err != nil {
		writeErr(w, 400, "bad request")
		return
	}

	// The support URL is rendered as a link on a public page, so it must be a
	// plain http(s) address — a javascript: URL there would be stored XSS
	// against the customer's own visitors.
	if in.SupportURL != nil && *in.SupportURL != "" {
		u, err := url.Parse(*in.SupportURL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			writeErr(w, 400, "support URL must start with http:// or https://")
			return
		}
	}

	// Branding removal is a paid feature; check the plan rather than trusting
	// the client to have hidden the toggle.
	if in.HideBranding != nil && *in.HideBranding {
		l, err := plans.Get(r.Context(), s.DB, c.TenantID)
		if err != nil {
			writeErr(w, 500, "db")
			return
		}
		if l.PriceCents == 0 {
			writeErr(w, http.StatusPaymentRequired,
				"removing Pulse branding requires a paid plan")
			return
		}
	}

	if _, err := s.DB.Exec(r.Context(), `
		UPDATE tenants SET
		  status_title         = COALESCE($2, status_title),
		  status_description   = COALESCE($3, status_description),
		  status_support_url   = COALESCE($4, status_support_url),
		  status_hide_branding = COALESCE($5, status_hide_branding)
		WHERE id=$1`,
		c.TenantID, in.Title, in.Description, in.SupportURL, in.HideBranding); err != nil {
		slog.Error("update status page settings", "err", err)
		writeErr(w, 500, "db")
		return
	}
	w.WriteHeader(204)
}
