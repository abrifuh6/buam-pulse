// Package http wires routes to handlers. Kept separate from main so the
// router can be tested without starting a real server.
package http

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/abrifuh6/buam-pulse/internal/config"
)

type Server struct {
	DB  *pgxpool.Pool
	Cfg config.Config
}

func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.Recoverer, Metrics, CORS(s.Cfg.CORSOrigins))

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })
	r.Get("/readyz", func(w http.ResponseWriter, req *http.Request) {
		if err := s.DB.Ping(req.Context()); err != nil {
			http.Error(w, "db unavailable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(200)
	})
	r.Handle("/metrics", promhttp.Handler())

	r.Route("/api/v1", func(r chi.Router) {
		// Auth endpoints are rate limited by IP: 10 attempts per minute is
		// generous for humans and useless for brute force.
		r.Group(func(r chi.Router) {
			r.Use(NewRateLimiter(10, time.Minute).Middleware(s.Cfg.TrustProxy))
			r.Post("/auth/signup", s.Signup)
			r.Post("/auth/login", s.Login)
			r.Post("/auth/forgot", s.RequestPasswordReset)
			r.Post("/auth/reset", s.ResetPassword)
			r.Post("/invitations/accept", s.AcceptInvitation)
		})

		// Token-gated but unauthenticated: the link itself is the credential.
		r.Get("/auth/verify", s.VerifyUser)
		r.Get("/channels/verify", s.VerifyChannel)
		r.Get("/invitations/info", s.InvitationInfo)

		// Public status pages.
		r.Get("/public/status/{slug}", s.PublicStatus)
		r.Get("/plans", s.ListPlans)

		r.Group(func(r chi.Router) {
			r.Use(RequireAuth(s.Cfg.JWTSecret))

			// Reads: any member.
			r.Get("/monitors", s.ListMonitors)
			r.Get("/monitors/{id}/results", s.MonitorResults)
			r.Get("/channels", s.ListChannels)
			r.Get("/team/members", s.ListMembers)
			r.Get("/team/invitations", s.ListInvitations)
			r.Get("/billing/plan", s.CurrentPlan)
			r.Get("/account", s.GetAccount)

			// Writes: admin and above.
			r.Group(func(r chi.Router) {
				r.Use(RequireRole("admin"))
				r.Post("/monitors", s.CreateMonitor)
				r.Patch("/monitors/{id}", s.UpdateMonitor)
				r.Delete("/monitors/{id}", s.DeleteMonitor)

				r.Post("/channels", s.CreateChannel)
				r.Delete("/channels/{id}", s.DeleteChannel)
				r.Post("/channels/{id}/test", s.TestChannel)

				r.Post("/team/invitations", s.InviteMember)
				r.Delete("/team/invitations/{id}", s.RevokeInvitation)
			})

			// Membership changes: owner only.
			r.Group(func(r chi.Router) {
				r.Use(RequireRole("owner"))
				r.Patch("/team/members/{id}", s.ChangeRole)
				r.Delete("/team/members/{id}", s.RemoveMember)
			})
		})
	})
	return r
}
