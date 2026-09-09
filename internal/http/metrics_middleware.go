package http

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/abrifuh6/buam-pulse/internal/metrics"
)

// Metrics records count and latency per route pattern (e.g. /api/v1/monitors/{id}),
// not per raw URL — otherwise every monitor ID would create a new time series.
func Metrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)

		route := chi.RouteContext(r.Context()).RoutePattern()
		if route == "" {
			route = "unmatched"
		}
		metrics.HTTPRequests.WithLabelValues(route, strconv.Itoa(ww.Status())).Inc()
		metrics.HTTPLatency.WithLabelValues(route).Observe(time.Since(start).Seconds())
	})
}
