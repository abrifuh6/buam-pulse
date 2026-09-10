package http

import (
	"context"
	"net/http"
	"strings"

	"github.com/abrifuh6/buam-pulse/internal/auth"
)

type ctxKey int

const claimsKey ctxKey = 1

// RequireAuth rejects requests without a valid Bearer token and stores the
// claims (including tenant_id) on the request context for handlers to use.
func RequireAuth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := r.Header.Get("Authorization")
			if !strings.HasPrefix(h, "Bearer ") {
				writeErr(w, http.StatusUnauthorized, "missing bearer token")
				return
			}
			c, err := auth.ParseToken(secret, strings.TrimPrefix(h, "Bearer "))
			if err != nil {
				writeErr(w, http.StatusUnauthorized, "invalid token")
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), claimsKey, c)))
		})
	}
}

func claimsFrom(r *http.Request) *auth.Claims {
	c, _ := r.Context().Value(claimsKey).(*auth.Claims)
	return c
}

// Role ranking. Comparing ranks rather than listing roles at each call site
// means adding a role later doesn't require touching every route.
var roleRank = map[string]int{"member": 1, "admin": 2, "owner": 3}

// RequireRole rejects a request whose token carries a role below the minimum.
// The role comes from the JWT, which is signed — a user cannot elevate
// themselves by editing the token.
//
// Note the tradeoff: a role change does not take effect until the user's token
// is reissued (24h max). For a demotion that matters, so revocation is the
// tracked follow-up — a token version column bumped on role change.
func RequireRole(minimum string) func(http.Handler) http.Handler {
	need := roleRank[minimum]
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c := claimsFrom(r)
			if c == nil || roleRank[c.Role] < need {
				writeErr(w, http.StatusForbidden, "your role does not allow this action")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
