# ADR 0003: Same-origin frontend, separate deployment

Status: Accepted · Date: 2026-09-09

## Context
The dashboard is a static React bundle; the API is a Go service. They can be
deployed together or apart, and served on one origin or two.

## Decision
- Deploy separately: the bundle goes to S3 + CloudFront, the API runs in EKS.
  Static assets do not belong in a pod — different scaling profile, different
  release cadence, no server cost.
- Serve on ONE origin: `app.pulse.io/` for the UI, `app.pulse.io/api/*` routed
  to the API. Vite's dev proxy reproduces this locally.
- The public status page is the deliberate exception: it is a separate origin
  and calls an unauthenticated endpoint, so permissive CORS is correct there.

## Consequences
- No CORS and no preflight round trip for dashboard traffic; one certificate.
- Keeps the door open to `HttpOnly; Secure; SameSite` session cookies, which
  are not readable by injected JavaScript. Today we still use a Bearer token in
  localStorage — the XSS exposure is accepted for now and cookie auth is the
  tracked follow-up.
- Path-based routing rules become a critical piece of config; a bad rule sends
  API traffic to the CDN.
- A separate `api.pulse.io` for third-party programmatic access remains possible
  later without changing the dashboard.
