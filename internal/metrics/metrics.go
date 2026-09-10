// Package metrics defines Pulse's Prometheus metrics in one place so every
// service names them consistently. Naming follows Prometheus conventions:
// <namespace>_<subsystem>_<name>_<unit>, counters end in _total.
package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// Worker: how many checks ran, split by outcome and monitor type.
	ChecksTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "pulse_checks_total",
		Help: "Checks performed, by monitor type and result.",
	}, []string{"type", "result"}) // result = ok | fail

	// Worker: latency distribution. Buckets chosen for web checks: 10ms..10s.
	CheckLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "pulse_check_latency_seconds",
		Help:    "Target response time observed by checks.",
		Buckets: []float64{.01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
	}, []string{"type"})

	// Scheduler: work waiting in Redis. This is the HPA scaling signal later.
	QueueDepth = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "pulse_queue_depth",
		Help: "Number of check jobs waiting in the queue.",
	})

	// Scheduler: monitors enqueued per tick.
	Enqueued = promauto.NewCounter(prometheus.CounterOpts{
		Name: "pulse_scheduler_enqueued_total",
		Help: "Monitors enqueued by the scheduler.",
	})

	// API: request count and latency by route and status.
	HTTPRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "pulse_http_requests_total",
		Help: "API requests by route and status code.",
	}, []string{"route", "status"})

	// Notifier: delivery outcomes and how much work is waiting.
	NotificationsSent = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "pulse_notifications_total",
		Help: "Notification delivery attempts by channel type, kind and outcome.",
	}, []string{"channel", "kind", "outcome"})

	NotificationBacklog = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "pulse_notification_backlog",
		Help: "Notifications waiting to be delivered.",
	})

	// Incidents opened, for correlating alert volume with real failures.
	IncidentsOpened = promauto.NewCounter(prometheus.CounterOpts{
		Name: "pulse_incidents_opened_total",
		Help: "Incidents opened.",
	})

	HTTPLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "pulse_http_request_duration_seconds",
		Help:    "API request latency.",
		Buckets: prometheus.DefBuckets,
	}, []string{"route"})
)

// Serve starts a tiny HTTP server exposing /metrics and /healthz. Used by the
// worker and scheduler, which otherwise have no HTTP listener.
func Serve(addr string) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })
	go func() { _ = http.ListenAndServe(addr, mux) }()
}
