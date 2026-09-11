// Package checks performs the actual probes. Kept free of database code so
// it is trivial to unit test.
package checks

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// FailureKind names why a check failed, so an alert can say something more
// useful than "it broke".
const (
	FailNone    = ""
	FailConnect = "connect" // DNS, TCP, TLS handshake
	FailTimeout = "timeout"
	FailStatus  = "status"  // wrong HTTP status
	FailKeyword = "keyword" // body assertion failed
	FailBlocked = "blocked" // target resolved to a non-public address
)

type Result struct {
	OK         bool
	StatusCode int
	LatencyMs  int
	Err        string
	Kind       string

	// TLS details, populated for https targets when the handshake succeeds.
	// A near-expiry certificate does NOT fail the check — the site is up —
	// but it warrants its own warning.
	CertExpiry *time.Time
	CertIssuer string
}

// Options carries the per-monitor assertions.
type Options struct {
	ExpectedStatus int
	Timeout        time.Duration
	Keyword        string
	KeywordPresent bool
	CheckSSL       bool

	// AllowPrivateTargets disables the SSRF guard. Only ever true in tests,
	// which necessarily point at 127.0.0.1. Making it an explicit option
	// rather than a build tag keeps the guard visible at every call site.
	AllowPrivateTargets bool
}

// maxBody caps how much of a response we read for keyword matching. Without a
// cap, a monitor pointed at a huge or endless stream would exhaust worker
// memory — a denial of service against ourselves.
const maxBody = 1 << 20 // 1 MiB

func HTTP(ctx context.Context, target string, opt Options) Result {
	// Re-check DNS at request time: a hostname that validated at creation can
	// be re-pointed at a private address afterwards (DNS rebinding).
	if !opt.AllowPrivateTargets {
		if host, err := hostOf(target); err == nil {
			if err := ResolveGuard(host); err != nil {
				return Result{Err: err.Error(), Kind: FailBlocked}
			}
		}
	}

	ctx, cancel := context.WithTimeout(ctx, opt.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return Result{Err: err.Error(), Kind: FailConnect}
	}
	req.Header.Set("User-Agent", "Pulse-Monitor/1.0 (+https://pulse.buamtech.io)")

	// A fresh transport per check rather than the shared default client: reusing
	// a pooled connection would skip the TLS handshake and DNS lookup, so the
	// measured latency would flatter the target and the certificate would never
	// be re-read. Monitoring should measure the cold path a real visitor takes.
	tr := &http.Transport{
		DisableKeepAlives: true,
		ForceAttemptHTTP2: true,
	}
	client := &http.Client{Transport: tr}

	start := time.Now()
	resp, err := client.Do(req)
	latency := int(time.Since(start).Milliseconds())
	if err != nil {
		kind := FailConnect
		if ctx.Err() == context.DeadlineExceeded {
			kind = FailTimeout
		}
		return Result{LatencyMs: latency, Err: err.Error(), Kind: kind}
	}
	defer func() { _ = resp.Body.Close() }()

	res := Result{StatusCode: resp.StatusCode, LatencyMs: latency}

	if opt.CheckSSL && resp.TLS != nil && len(resp.TLS.PeerCertificates) > 0 {
		leaf := resp.TLS.PeerCertificates[0]
		res.CertExpiry = &leaf.NotAfter
		res.CertIssuer = leaf.Issuer.CommonName
	}

	if resp.StatusCode != opt.ExpectedStatus {
		res.Err = fmt.Sprintf("expected status %d, got %d", opt.ExpectedStatus, resp.StatusCode)
		res.Kind = FailStatus
		return res
	}

	if opt.Keyword != "" {
		body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
		if err != nil {
			res.Err = "could not read response body: " + err.Error()
			res.Kind = FailConnect
			return res
		}
		// Case-insensitive: page copy changes capitalisation more often than a
		// user intends the assertion to be strict.
		found := strings.Contains(strings.ToLower(string(body)), strings.ToLower(opt.Keyword))
		if found != opt.KeywordPresent {
			if opt.KeywordPresent {
				res.Err = fmt.Sprintf("page did not contain %q", opt.Keyword)
			} else {
				res.Err = fmt.Sprintf("page contained %q", opt.Keyword)
			}
			res.Kind = FailKeyword
			return res
		}
	}

	res.OK = true
	return res
}

func TCP(ctx context.Context, hostport string, timeout time.Duration) Result {
	if host, _, err := net.SplitHostPort(hostport); err == nil {
		if err := ResolveGuard(host); err != nil {
			return Result{Err: err.Error(), Kind: FailBlocked}
		}
	}

	start := time.Now()
	var d net.Dialer
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	conn, err := d.DialContext(ctx, "tcp", hostport)
	latency := int(time.Since(start).Milliseconds())
	if err != nil {
		kind := FailConnect
		if ctx.Err() == context.DeadlineExceeded {
			kind = FailTimeout
		}
		return Result{LatencyMs: latency, Err: err.Error(), Kind: kind}
	}
	_ = conn.Close()
	return Result{OK: true, LatencyMs: latency}
}

// TLSExpiry reads a certificate without making an HTTP request. Used for TCP
// monitors on TLS ports, where there is no HTTP response to inspect.
func TLSExpiry(ctx context.Context, hostport string, timeout time.Duration) (*time.Time, string, error) {
	d := &net.Dialer{Timeout: timeout}
	conn, err := tls.DialWithDialer(d, "tcp", hostport, &tls.Config{MinVersion: tls.VersionTLS12})
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = conn.Close() }()

	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return nil, "", fmt.Errorf("no certificate presented")
	}
	return &certs[0].NotAfter, certs[0].Issuer.CommonName, nil
}

func hostOf(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	return u.Hostname(), nil
}
