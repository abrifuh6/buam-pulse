package http

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// RateLimiter is a fixed-window counter per key, held in memory.
//
// In-memory means each API pod counts separately, so N pods allow N times the
// limit. That is an accepted tradeoff for now: it stops naive brute force with
// zero dependencies. A Redis-backed counter (shared across pods) is the
// follow-up when limits need to be exact — see ADR 0005.
type RateLimiter struct {
	mu       sync.Mutex
	counts   map[string]int
	window   time.Duration
	limit    int
	resetsAt time.Time
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		counts:   map[string]int{},
		window:   window,
		limit:    limit,
		resetsAt: time.Now().Add(window),
	}
}

func (rl *RateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if time.Now().After(rl.resetsAt) {
		rl.counts = map[string]int{}
		rl.resetsAt = time.Now().Add(rl.window)
	}
	rl.counts[key]++
	return rl.counts[key] <= rl.limit
}

// Middleware limits by client IP. Behind a load balancer the real client is in
// X-Forwarded-For, but we do NOT trust that header from arbitrary clients —
// it is spoofable (the reason chi's RealIP was dropped). In EKS the ALB sets
// it and the ingress is the only path in, so it is trustworthy there and we
// read the LAST entry, which the trusted proxy appends.
func (rl *RateLimiter) Middleware(trustProxy bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !rl.allow(clientKey(r, trustProxy)) {
				w.Header().Set("Retry-After", "60")
				writeErr(w, http.StatusTooManyRequests, "too many attempts, try again shortly")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func clientKey(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			// Last entry is the one appended by our own proxy.
			for i := len(xff) - 1; i >= 0; i-- {
				if xff[i] == ',' {
					return trimSpace(xff[i+1:])
				}
			}
			return trimSpace(xff)
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}
