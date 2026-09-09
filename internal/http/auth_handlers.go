package http

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/abrifuh6/buam-pulse/internal/auth"
)

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

type signupReq struct {
	Company  string `json:"company"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Signup creates a tenant and its first (owner) user in ONE transaction.
// If either insert fails, neither is written — no orphan tenants.
func (s *Server) Signup(w http.ResponseWriter, r *http.Request) {
	var in signupReq
	if err := decode(r, &in); err != nil || in.Company == "" || in.Email == "" || len(in.Password) < 8 {
		writeErr(w, 400, "company, email and password (min 8 chars) required")
		return
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		writeErr(w, 500, "hash failed")
		return
	}
	slug := strings.Trim(slugRe.ReplaceAllString(strings.ToLower(in.Company), "-"), "-")

	tx, err := s.DB.Begin(r.Context())
	if err != nil {
		writeErr(w, 500, "db")
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	
	var tenantID, userID string
	if err := tx.QueryRow(r.Context(),
		`INSERT INTO tenants (name, slug) VALUES ($1,$2) RETURNING id`, in.Company, slug).Scan(&tenantID); err != nil {
		writeErr(w, 409, "company name already taken")
		return
	}
	if err := tx.QueryRow(r.Context(),
		`INSERT INTO users (tenant_id, email, password_hash, role) VALUES ($1,$2,$3,'owner') RETURNING id`,
		tenantID, strings.ToLower(in.Email), hash).Scan(&userID); err != nil {
		writeErr(w, 409, "email already registered")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeErr(w, 500, "db")
		return
	}
	tok, _ := auth.IssueToken(s.Cfg.JWTSecret, userID, tenantID, "owner")
	writeJSON(w, 201, map[string]string{"token": tok, "tenant_id": tenantID, "slug": slug})
}

type loginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (s *Server) Login(w http.ResponseWriter, r *http.Request) {
	var in loginReq
	if err := decode(r, &in); err != nil {
		writeErr(w, 400, "bad request")
		return
	}
	var userID, tenantID, role, hash string
	err := s.DB.QueryRow(r.Context(),
		`SELECT id, tenant_id, role, password_hash FROM users WHERE email=$1`, strings.ToLower(in.Email)).
		Scan(&userID, &tenantID, &role, &hash)
	// Same error for unknown email and wrong password: never reveal which.
	if err != nil || !auth.CheckPassword(hash, in.Password) {
		writeErr(w, 401, "invalid credentials")
		return
	}
	tok, _ := auth.IssueToken(s.Cfg.JWTSecret, userID, tenantID, role)
	writeJSON(w, 200, map[string]string{"token": tok})
}