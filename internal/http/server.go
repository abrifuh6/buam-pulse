// Package http wires routes to handlers. Kept separate from main so the
// router can be tested without starting a real server.
package http

import (
	"net/http"

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
		r.Post("/auth/signup", s.Signup)
		r.Post("/auth/login", s.Login)

		// Public: the status page is meant to be linked publicly.
		r.Get("/public/status/{slug}", s.PublicStatus)
		// Unauthenticated by design: the token in the emailed link is the proof.
		r.Get("/channels/verify", s.VerifyChannel)

		r.Group(func(r chi.Router) {
			r.Use(RequireAuth(s.Cfg.JWTSecret))

			r.Get("/monitors", s.ListMonitors)
			r.Post("/monitors", s.CreateMonitor)
			r.Delete("/monitors/{id}", s.DeleteMonitor)
			r.Get("/monitors/{id}/results", s.MonitorResults)

			r.Get("/channels", s.ListChannels)
			r.Post("/channels", s.CreateChannel)
			r.Delete("/channels/{id}", s.DeleteChannel)
			r.Post("/channels/{id}/test", s.TestChannel)
		})
	})
	return r
}
