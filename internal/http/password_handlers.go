package http

import (
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/abrifuh6/buam-pulse/internal/alerting"
	"github.com/abrifuh6/buam-pulse/internal/auth"
)

// Tokens are random, single-use, and time-limited. We store only the SHA-256
// hash: if the database leaks, the stored values are not usable as links.
func hashToken(t string) string {
	sum := sha256.Sum256([]byte(t))
	return hex.EncodeToString(sum[:])
}

const (
	resetTTL  = 1 * time.Hour
	verifyTTL = 48 * time.Hour
)

type emailOnlyReq struct {
	Email string `json:"email"`
}

// RequestPasswordReset always returns 200, whether or not the address exists.
// Anything else is an account-enumeration oracle: an attacker could learn which
// emails are registered by watching the response.
func (s *Server) RequestPasswordReset(w http.ResponseWriter, r *http.Request) {
	var in emailOnlyReq
	if err := decode(r, &in); err != nil {
		writeErr(w, 400, "bad request")
		return
	}
	email := strings.ToLower(strings.TrimSpace(in.Email))

	var userID string
	err := s.DB.QueryRow(r.Context(), `SELECT id FROM users WHERE email=$1`, email).Scan(&userID)
	if err == nil {
		token := randomToken()
		_, err = s.DB.Exec(r.Context(), `
			INSERT INTO auth_tokens (user_id, kind, token_hash, expires_at)
			VALUES ($1,'reset',$2, now() + $3::interval)`,
			userID, hashToken(token), resetTTL.String())
		if err != nil {
			slog.Error("create reset token", "err", err)
		} else {
			link := s.Cfg.DashboardURL + "/reset?token=" + token
			body := "Someone requested a password reset for your Pulse account.\n\n" +
				"Reset it here (valid for 1 hour): " + link + "\n\n" +
				"If this wasn't you, ignore this email — your password is unchanged.\n"
			if err := alerting.SendEmail(s.smtp(), email, "Reset your Pulse password", body); err != nil {
				slog.Error("send reset email", "err", err)
			}
		}
	}

	writeJSON(w, 200, map[string]string{
		"status": "if that address has an account, a reset link is on its way",
	})
}

type resetReq struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

func (s *Server) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var in resetReq
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

	// Claiming the token and changing the password happen together: a token can
	// never be spent without the password actually changing, and vice versa.
	var userID string
	err = tx.QueryRow(r.Context(), `
		UPDATE auth_tokens SET used_at = now()
		WHERE token_hash=$1 AND kind='reset' AND used_at IS NULL AND expires_at > now()
		RETURNING user_id`, hashToken(in.Token)).Scan(&userID)
	if err != nil {
		writeErr(w, 400, "invalid or expired reset link")
		return
	}

	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		writeErr(w, 500, "hash failed")
		return
	}
	if _, err = tx.Exec(r.Context(),
		`UPDATE users SET password_hash=$2 WHERE id=$1`, userID, hash); err != nil {
		writeErr(w, 500, "db")
		return
	}
	// Any other outstanding reset tokens for this user are now void.
	if _, err = tx.Exec(r.Context(), `
		UPDATE auth_tokens SET used_at = now()
		WHERE user_id=$1 AND kind='reset' AND used_at IS NULL`, userID); err != nil {
		writeErr(w, 500, "db")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeErr(w, 500, "db")
		return
	}

	writeJSON(w, 200, map[string]string{"status": "password updated"})
}

// VerifyUser confirms a signup address. Unauthenticated: the token is the proof.
func (s *Server) VerifyUser(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		writeErr(w, 400, "missing token")
		return
	}
	var userID string
	err := s.DB.QueryRow(r.Context(), `
		UPDATE auth_tokens SET used_at = now()
		WHERE token_hash=$1 AND kind='verify' AND used_at IS NULL AND expires_at > now()
		RETURNING user_id`, hashToken(token)).Scan(&userID)
	if err != nil {
		writeErr(w, 400, "invalid or expired verification link")
		return
	}
	if _, err = s.DB.Exec(r.Context(),
		`UPDATE users SET verified_at = now() WHERE id=$1`, userID); err != nil {
		writeErr(w, 500, "db")
		return
	}
	writeJSON(w, 200, map[string]string{"status": "email verified"})
}

// sendVerification is called after signup. Failure to send is logged, not
// returned: the account exists and the user can request another link.
func (s *Server) sendVerification(r *http.Request, userID, email string) {
	token := randomToken()
	if _, err := s.DB.Exec(r.Context(), `
		INSERT INTO auth_tokens (user_id, kind, token_hash, expires_at)
		VALUES ($1,'verify',$2, now() + $3::interval)`,
		userID, hashToken(token), verifyTTL.String()); err != nil {
		slog.Error("create verify token", "err", err)
		return
	}
	link := s.Cfg.APIURL + "/api/v1/auth/verify?token=" + token
	body := "Welcome to Pulse.\n\nConfirm your email address: " + link +
		"\n\nThis link is valid for 48 hours.\n"
	if err := alerting.SendEmail(s.smtp(), email, "Confirm your Pulse account", body); err != nil {
		slog.Error("send verification email", "err", err)
	}
}

type changePasswordReq struct {
	Current string `json:"current_password"`
	New     string `json:"new_password"`
}

// ChangePassword requires the current password even though the user is already
// authenticated. A session left open on a shared machine should not be enough
// to lock the real owner out of their own account.
func (s *Server) ChangePassword(w http.ResponseWriter, r *http.Request) {
	c := claimsFrom(r)

	var in changePasswordReq
	if err := decode(r, &in); err != nil || len(in.New) < 8 {
		writeErr(w, 400, "Your new password must be at least 8 characters.")
		return
	}

	var hash string
	if err := s.DB.QueryRow(r.Context(),
		`SELECT password_hash FROM users WHERE id=$1`, c.UserID).Scan(&hash); err != nil {
		writeErr(w, 500, "db")
		return
	}
	if !auth.CheckPassword(hash, in.Current) {
		writeErr(w, 401, "That is not your current password.")
		return
	}

	next, err := auth.HashPassword(in.New)
	if err != nil {
		writeErr(w, 500, "hash failed")
		return
	}
	if _, err := s.DB.Exec(r.Context(),
		`UPDATE users SET password_hash=$2 WHERE id=$1`, c.UserID, next); err != nil {
		writeErr(w, 500, "db")
		return
	}
	// Any outstanding reset links are void: changing your password should
	// invalidate a link someone may have requested on your behalf.
	if _, err := s.DB.Exec(r.Context(), `
		UPDATE auth_tokens SET used_at=now()
		WHERE user_id=$1 AND kind='reset' AND used_at IS NULL`, c.UserID); err != nil {
		slog.Warn("void reset tokens", "err", err)
	}

	writeJSON(w, 200, map[string]string{"status": "password updated"})
}
