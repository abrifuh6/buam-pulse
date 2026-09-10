package http

import (
	"log/slog"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/abrifuh6/buam-pulse/internal/alerting"
	"github.com/abrifuh6/buam-pulse/internal/auth"
	"github.com/abrifuh6/buam-pulse/internal/plans"
)

const inviteTTL = 7 * 24 * time.Hour

type Member struct {
	ID       string    `json:"id"`
	Email    string    `json:"email"`
	Role     string    `json:"role"`
	Verified bool      `json:"verified"`
	JoinedAt time.Time `json:"joined_at"`
	IsYou    bool      `json:"is_you"`
}

func (s *Server) ListMembers(w http.ResponseWriter, r *http.Request) {
	c := claimsFrom(r)
	rows, err := s.DB.Query(r.Context(), `
		SELECT id, email, role, verified_at IS NOT NULL, created_at
		FROM users WHERE tenant_id=$1
		ORDER BY CASE role WHEN 'owner' THEN 1 WHEN 'admin' THEN 2 ELSE 3 END, email`,
		c.TenantID)
	if err != nil {
		writeErr(w, 500, "db")
		return
	}
	defer rows.Close()

	out := []Member{}
	for rows.Next() {
		var m Member
		if rows.Scan(&m.ID, &m.Email, &m.Role, &m.Verified, &m.JoinedAt) == nil {
			m.IsYou = m.ID == c.UserID
			out = append(out, m)
		}
	}
	writeJSON(w, 200, out)
}

type Invitation struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	ExpiresAt time.Time `json:"expires_at"`
}

func (s *Server) ListInvitations(w http.ResponseWriter, r *http.Request) {
	c := claimsFrom(r)
	rows, err := s.DB.Query(r.Context(), `
		SELECT id, email, role, expires_at FROM invitations
		WHERE tenant_id=$1 AND accepted_at IS NULL AND expires_at > now()
		ORDER BY created_at DESC`, c.TenantID)
	if err != nil {
		writeErr(w, 500, "db")
		return
	}
	defer rows.Close()

	out := []Invitation{}
	for rows.Next() {
		var i Invitation
		if rows.Scan(&i.ID, &i.Email, &i.Role, &i.ExpiresAt) == nil {
			out = append(out, i)
		}
	}
	writeJSON(w, 200, out)
}

type inviteReq struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

func (s *Server) InviteMember(w http.ResponseWriter, r *http.Request) {
	c := claimsFrom(r)
	var in inviteReq
	if err := decode(r, &in); err != nil {
		writeErr(w, 400, "bad request")
		return
	}
	addr, err := mail.ParseAddress(strings.TrimSpace(in.Email))
	if err != nil {
		writeErr(w, 400, "invalid email address")
		return
	}
	email := strings.ToLower(addr.Address)

	// Only owner and admin can invite, and neither can mint an owner: ownership
	// transfers explicitly, it is never granted by invitation.
	if in.Role != "admin" && in.Role != "member" {
		writeErr(w, 400, "role must be admin or member")
		return
	}

	// A user can only belong to one tenant in this model, so an address that
	// already has an account anywhere cannot be invited.
	var exists bool
	if err := s.DB.QueryRow(r.Context(),
		`SELECT EXISTS (SELECT 1 FROM users WHERE email=$1)`, email).Scan(&exists); err == nil && exists {
		writeErr(w, 409, "that address already has a Pulse account")
		return
	}

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
	if err := plans.CheckMember(r.Context(), tx, c.TenantID); err != nil {
		writeErr(w, http.StatusPaymentRequired, err.Error())
		return
	}

	token := randomToken()
	// Re-inviting the same address replaces the outstanding invite rather than
	// failing, which is what a user expects when they mistype and retry.
	_, err = tx.Exec(r.Context(), `
		INSERT INTO invitations (tenant_id, email, role, token_hash, invited_by, expires_at)
		VALUES ($1,$2,$3,$4,$5, now() + $6::interval)
		ON CONFLICT (tenant_id, email) DO UPDATE
		SET role=$3, token_hash=$4, invited_by=$5, expires_at=now() + $6::interval,
		    accepted_at=NULL, created_at=now()`,
		c.TenantID, email, in.Role, hashToken(token), c.UserID, inviteTTL.String())
	if err != nil {
		slog.Error("create invitation", "err", err)
		writeErr(w, 500, "could not create invitation")
		return
	}

	var tenantName string
	_ = tx.QueryRow(r.Context(), `SELECT name FROM tenants WHERE id=$1`, c.TenantID).Scan(&tenantName)

	if err := tx.Commit(r.Context()); err != nil {
		writeErr(w, 500, "db")
		return
	}

	link := s.Cfg.DashboardURL + "/accept?token=" + token
	body := "You've been invited to join " + tenantName + " on Pulse as " + in.Role + ".\n\n" +
		"Accept the invitation (valid for 7 days): " + link + "\n\n" +
		"If you weren't expecting this, ignore this email.\n"
	if err := alerting.SendEmail(s.smtp(), email, "You've been invited to "+tenantName+" on Pulse", body); err != nil {
		slog.Error("send invitation", "err", err)
	}

	writeJSON(w, 201, map[string]string{"status": "invitation sent"})
}

func (s *Server) RevokeInvitation(w http.ResponseWriter, r *http.Request) {
	c := claimsFrom(r)
	tag, err := s.DB.Exec(r.Context(),
		`DELETE FROM invitations WHERE id=$1 AND tenant_id=$2 AND accepted_at IS NULL`,
		chi.URLParam(r, "id"), c.TenantID)
	if err != nil || tag.RowsAffected() == 0 {
		writeErr(w, 404, "not found")
		return
	}
	w.WriteHeader(204)
}

type acceptReq struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

// AcceptInvitation is unauthenticated: the token is the proof. It creates the
// user and consumes the invite in one transaction.
func (s *Server) AcceptInvitation(w http.ResponseWriter, r *http.Request) {
	var in acceptReq
	if err := decode(r, &in); err != nil || len(in.Password) < 8 {
		writeErr(w, 400, "token and a password of at least 8 characters are required")
		return
	}

	tx, err := s.DB.Begin(r.Context())
	if err != nil {
		writeErr(w, 500, "db")
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()

	var tenantID, email, role string
	err = tx.QueryRow(r.Context(), `
		UPDATE invitations SET accepted_at = now()
		WHERE token_hash=$1 AND accepted_at IS NULL AND expires_at > now()
		RETURNING tenant_id, email, role`, hashToken(in.Token)).
		Scan(&tenantID, &email, &role)
	if err != nil {
		writeErr(w, 400, "invalid or expired invitation")
		return
	}

	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		writeErr(w, 500, "hash failed")
		return
	}

	var userID string
	// verified_at is set immediately: receiving the invite email at that
	// address IS the proof of ownership, so a second confirmation is noise.
	err = tx.QueryRow(r.Context(), `
		INSERT INTO users (tenant_id, email, password_hash, role, verified_at)
		VALUES ($1,$2,$3,$4, now()) RETURNING id`,
		tenantID, email, hash, role).Scan(&userID)
	if err != nil {
		writeErr(w, 409, "that address already has an account")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeErr(w, 500, "db")
		return
	}

	tok, _ := auth.IssueToken(s.Cfg.JWTSecret, userID, tenantID, role)
	writeJSON(w, 201, map[string]string{"token": tok})
}

type roleReq struct {
	Role string `json:"role"`
}

// ChangeRole is owner-only. The owner cannot demote themselves, because a
// tenant with no owner has no one who can manage billing or delete it.
func (s *Server) ChangeRole(w http.ResponseWriter, r *http.Request) {
	c := claimsFrom(r)
	targetID := chi.URLParam(r, "id")

	var in roleReq
	if err := decode(r, &in); err != nil {
		writeErr(w, 400, "bad request")
		return
	}
	if in.Role != "admin" && in.Role != "member" && in.Role != "owner" {
		writeErr(w, 400, "invalid role")
		return
	}
	if targetID == c.UserID {
		writeErr(w, 400, "you cannot change your own role")
		return
	}

	tx, err := s.DB.Begin(r.Context())
	if err != nil {
		writeErr(w, 500, "db")
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()

	// Promoting someone to owner is a transfer: the current owner steps down to
	// admin in the same transaction, so there is never zero or two owners.
	if in.Role == "owner" {
		if _, err = tx.Exec(r.Context(),
			`UPDATE users SET role='admin' WHERE id=$1 AND tenant_id=$2`, c.UserID, c.TenantID); err != nil {
			writeErr(w, 500, "db")
			return
		}
	}

	tag, err := tx.Exec(r.Context(),
		`UPDATE users SET role=$3 WHERE id=$1 AND tenant_id=$2 AND role <> 'owner'`,
		targetID, c.TenantID, in.Role)
	if err != nil || tag.RowsAffected() == 0 {
		writeErr(w, 404, "member not found")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeErr(w, 500, "db")
		return
	}
	writeJSON(w, 200, map[string]string{"status": "role updated"})
}

// RemoveMember is owner-only and cannot remove the owner.
func (s *Server) RemoveMember(w http.ResponseWriter, r *http.Request) {
	c := claimsFrom(r)
	targetID := chi.URLParam(r, "id")
	if targetID == c.UserID {
		writeErr(w, 400, "you cannot remove yourself; transfer ownership first")
		return
	}
	tag, err := s.DB.Exec(r.Context(),
		`DELETE FROM users WHERE id=$1 AND tenant_id=$2 AND role <> 'owner'`, targetID, c.TenantID)
	if err != nil {
		writeErr(w, 500, "db")
		return
	}
	if tag.RowsAffected() == 0 {
		writeErr(w, 404, "member not found")
		return
	}
	w.WriteHeader(204)
}

// InvitationInfo lets the accept page show who invited you and to what, before
// you commit to creating an account. Unauthenticated, token-gated, read-only.
func (s *Server) InvitationInfo(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		writeErr(w, 400, "missing token")
		return
	}
	var tenantName, email, role string
	err := s.DB.QueryRow(r.Context(), `
		SELECT t.name, i.email, i.role
		FROM invitations i JOIN tenants t ON t.id = i.tenant_id
		WHERE i.token_hash=$1 AND i.accepted_at IS NULL AND i.expires_at > now()`,
		hashToken(token)).Scan(&tenantName, &email, &role)
	if err == pgx.ErrNoRows {
		writeErr(w, 404, "invalid or expired invitation")
		return
	}
	if err != nil {
		writeErr(w, 500, "db")
		return
	}
	writeJSON(w, 200, map[string]string{"tenant": tenantName, "email": email, "role": role})
}
