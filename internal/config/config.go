// Package config reads settings from environment variables.
// Twelve-factor rule: config lives in the environment, never in code.
// The same binary runs on your laptop, k3s, and EKS — only the env differs.
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
