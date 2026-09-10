// Package checks performs the actual probes. Kept free of database code so
// it is trivial to unit test.
package checks

import (
	"context"
	"net"
	"net/http"
	"time"
)

type Result struct {
	OK         bool
	StatusCode int
	LatencyMs  int
	Err        string
}

func HTTP(ctx context.Context, url string, expected int, timeout time.Duration) Result {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Result{Err: err.Error()}
	}
	req.Header.Set("User-Agent", "Pulse-Monitor/1.0")

	start := time.Now()
	resp, err := http.DefaultClient.Do(req)
	latency := int(time.Since(start).Milliseconds())
	if err != nil {
		return Result{LatencyMs: latency, Err: err.Error()}
	}
	defer func() { _ = resp.Body.Close() }()

	return Result{
		OK:         resp.StatusCode == expected,
		StatusCode: resp.StatusCode,
		LatencyMs:  latency,
	}
}

func TCP(ctx context.Context, hostport string, timeout time.Duration) Result {
	start := time.Now()
	var d net.Dialer
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	conn, err := d.DialContext(ctx, "tcp", hostport)
	latency := int(time.Since(start).Milliseconds())
	if err != nil {
		return Result{LatencyMs: latency, Err: err.Error()}
	}
	_ = conn.Close()

	return Result{OK: true, LatencyMs: latency}
}
