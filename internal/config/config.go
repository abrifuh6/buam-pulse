// Package config reads settings from environment variables.
// Twelve-factor rule: config lives in the environment, never in code.
// The same binary runs on your laptop, Kubernetes, and EKS — only the env differs.
package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	DatabaseURL string
	RedisURL    string
	APIPort     string
	Region      string // which worker region produced a result
	JWTSecret   string
	MetricsPort string
	CORSOrigins []string

	// Alert delivery. Mailpit locally, SES in stage/prod — the app only ever
	// knows SMTP settings, so the provider swap is a config change.
	SMTPHost     string
	SMTPPort     string
	SMTPUser     string
	SMTPPassword string
	AlertFrom    string

	// Three public addresses. In production these collapse onto two hosts
	// (app.pulse.io serves the dashboard and, under /api, the API), but in dev
	// they are three ports. Getting one wrong produces emails with dead links.
	APIURL       string // API base — links the browser follows to an endpoint
	DashboardURL string // dashboard base — links to a page in the web app
	StatusURL    string // where public status pages live — alert links

	// Only true when the API sits behind a proxy we control (the ingress/ALB).
	// Never true when the API is directly reachable: X-Forwarded-For is spoofable.
	TrustProxy bool

	// Stripe. The secret key signs API calls; the webhook secret verifies that
	// an incoming webhook really came from Stripe and not from anyone who found
	// the endpoint URL.
	StripeSecretKey     string
	StripeWebhookSecret string
}

func Load() (Config, error) {
	c := Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		RedisURL:    os.Getenv("REDIS_URL"),
		APIPort:     getenv("API_PORT", "8080"),
		Region:      getenv("PULSE_REGION", "local"),
		JWTSecret:   os.Getenv("JWT_SECRET"),
		MetricsPort: getenv("METRICS_PORT", "9090"),
		CORSOrigins: strings.Split(getenv("CORS_ORIGINS", "http://localhost:5173,http://localhost:5174"), ","),

		SMTPHost:     getenv("SMTP_HOST", "localhost"),
		SMTPPort:     getenv("SMTP_PORT", "1025"),
		SMTPUser:     os.Getenv("SMTP_USER"),
		SMTPPassword: os.Getenv("SMTP_PASSWORD"),
		AlertFrom:    getenv("ALERT_FROM", "alerts@pulse.local"),

		APIURL:       getenv("API_URL", "http://localhost:8080"),
		DashboardURL: getenv("DASHBOARD_URL", "http://localhost:5173"),
		StatusURL:    getenv("STATUS_URL", "http://localhost:5174"),

		TrustProxy: getenv("TRUST_PROXY", "false") == "true",

		StripeSecretKey:     os.Getenv("STRIPE_SECRET_KEY"),
		StripeWebhookSecret: os.Getenv("STRIPE_WEBHOOK_SECRET"),
	}
	if c.DatabaseURL == "" || c.RedisURL == "" {
		return c, fmt.Errorf("DATABASE_URL and REDIS_URL are required")
	}
	return c, nil
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
