package http

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/mail"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/abrifuh6/buam-pulse/internal/alerting"
)

type Channel struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Type     string            `json:"type"`
	Config   map[string]string `json:"config"`
	Enabled  bool              `json:"enabled"`
	Verified bool              `json:"verified"`
}

func (s *Server) ListChannels(w http.ResponseWriter, r *http.Request) {
	c := claimsFrom(r)
	rows, err := s.DB.Query(r.Context(), `
		SELECT id, name, type, config, enabled, verified_at IS NOT NULL
		FROM alert_channels WHERE tenant_id=$1 ORDER BY created_at`, c.TenantID)
	if err != nil {
		writeErr(w, 500, "db")
		return
	}
	defer rows.Close()

	out := []Channel{}
	for rows.Next() {
		var ch Channel
		var raw []byte
		if rows.Scan(&ch.ID, &ch.Name, &ch.Type, &raw, &ch.Enabled, &ch.Verified) != nil {
			continue
		}
		_ = json.Unmarshal(raw, &ch.Config)
		// Never echo a Slack webhook URL back to the browser: it is a bearer
		// credential — anyone holding it can post to that channel.
		if ch.Type == "slack" {
			ch.Config = map[string]string{"webhook_url": maskWebhook(ch.Config["webhook_url"])}
		}
		out = append(out, ch)
	}
	writeJSON(w, 200, out)
}

func maskWebhook(u string) string {
	if len(u) < 12 {
		return "••••"
	}
	return u[:24] + "…••••"
}

type channelReq struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Address    string `json:"address"`     // email
	WebhookURL string `json:"webhook_url"` // slack
}

func (s *Server) CreateChannel(w http.ResponseWriter, r *http.Request) {
	c := claimsFrom(r)
	var in channelReq
	if err := decode(r, &in); err != nil || in.Name == "" {
		writeErr(w, 400, "name and type required")
		return
	}

	var cfg map[string]string
	var verifyToken string

	switch in.Type {
	case "email":
		addr, err := mail.ParseAddress(strings.TrimSpace(in.Address))
		if err != nil {
			writeErr(w, 400, "invalid email address")
			return
		}
		cfg = map[string]string{"address": addr.Address}
		// Unverified addresses never receive alerts. Without this, Pulse can be
		// used to send mail to anyone the attacker names.
		verifyToken = randomToken()

	case "slack":
		u, err := url.Parse(in.WebhookURL)
		if err != nil || u.Scheme != "https" || !strings.HasSuffix(u.Host, "slack.com") {
			writeErr(w, 400, "webhook must be an https hooks.slack.com URL")
			return
		}
		cfg = map[string]string{"webhook_url": in.WebhookURL}

	default:
		writeErr(w, 400, "type must be email or slack")
		return
	}

	raw, _ := json.Marshal(cfg)
	var id string
	err := s.DB.QueryRow(r.Context(), `
		INSERT INTO alert_channels (tenant_id, name, type, config, verify_token, verified_at)
		VALUES ($1,$2,$3,$4,$5, CASE WHEN $3='slack' THEN now() ELSE NULL END)
		RETURNING id`, c.TenantID, in.Name, in.Type, raw, nullIfEmpty(verifyToken)).Scan(&id)
	if err != nil {
		slog.Error("create channel", "err", err)
		writeErr(w, 400, "could not create channel")
		return
	}

	if in.Type == "email" {
		link := s.Cfg.APIURL + "/api/v1/channels/verify?token=" + verifyToken
		subject := "Confirm alerts to this address"
		body := "You (or someone at your company) added this address to receive Pulse alerts.\n\n" +
			"Confirm: " + link + "\n\nIf this wasn't you, ignore this email — no alerts will be sent.\n"
		if err := alerting.SendEmail(s.smtp(), cfg["address"], subject, body); err != nil {
			slog.Error("send verification", "err", err)
		}
	}

	writeJSON(w, 201, map[string]any{"id": id, "verification_sent": in.Type == "email"})
}

// VerifyChannel is unauthenticated: the token in the link is the proof. It is
// single-use — cleared once redeemed.
func (s *Server) VerifyChannel(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		writeErr(w, 400, "missing token")
		return
	}
	tag, err := s.DB.Exec(r.Context(), `
		UPDATE alert_channels SET verified_at=now(), verify_token=NULL
		WHERE verify_token=$1`, token)
	if err != nil || tag.RowsAffected() == 0 {
		writeErr(w, 404, "invalid or already-used verification link")
		return
	}
	writeJSON(w, 200, map[string]string{"status": "verified"})
}

func (s *Server) DeleteChannel(w http.ResponseWriter, r *http.Request) {
	c := claimsFrom(r)
	tag, err := s.DB.Exec(r.Context(),
		`DELETE FROM alert_channels WHERE id=$1 AND tenant_id=$2`,
		chi.URLParam(r, "id"), c.TenantID)
	if err != nil || tag.RowsAffected() == 0 {
		writeErr(w, 404, "not found")
		return
	}
	w.WriteHeader(204)
}

// TestChannel sends a sample alert so the user can confirm delivery works
// before an incident depends on it.
func (s *Server) TestChannel(w http.ResponseWriter, r *http.Request) {
	c := claimsFrom(r)
	var chType string
	var raw []byte
	var verified bool
	err := s.DB.QueryRow(r.Context(), `
		SELECT type, config, verified_at IS NOT NULL
		FROM alert_channels WHERE id=$1 AND tenant_id=$2`,
		chi.URLParam(r, "id"), c.TenantID).Scan(&chType, &raw, &verified)
	if err != nil {
		writeErr(w, 404, "not found")
		return
	}
	if !verified {
		writeErr(w, 400, "channel not verified yet")
		return
	}

	var cfg map[string]string
	_ = json.Unmarshal(raw, &cfg)

	const subject = "[Pulse] Test alert"
	const body = "This is a test alert from Pulse. If you received it, your channel is working.\n"

	switch chType {
	case "email":
		err = alerting.SendEmail(s.smtp(), cfg["address"], subject, body)
	case "slack":
		err = alerting.SendSlack(r.Context(), cfg["webhook_url"], subject+"\n"+body)
	}
	if err != nil {
		slog.Error("test channel", "err", err)
		writeErr(w, 502, "delivery failed: "+err.Error())
		return
	}
	writeJSON(w, 200, map[string]string{"status": "sent"})
}

func (s *Server) smtp() alerting.SMTPConfig {
	return alerting.SMTPConfig{
		Host: s.Cfg.SMTPHost, Port: s.Cfg.SMTPPort,
		User: s.Cfg.SMTPUser, Password: s.Cfg.SMTPPassword, From: s.Cfg.AlertFrom,
	}
}

func randomToken() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
