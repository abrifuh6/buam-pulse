# ADR 0005: In-memory rate limiting, for now

Status: Accepted · Date: 2026-09-10

## Context
Auth endpoints need brute-force protection. Options: in-memory per-pod counters,
a shared Redis counter, or the ingress/WAF.

## Decision
Start with an in-memory fixed-window limiter (10 requests per IP per minute) on
signup, login, forgot and reset.

## Consequences
- Limits are per-pod: with N API replicas an attacker gets N times the budget.
  Acceptable against naive brute force; not a real defence against a determined
  distributed attack.
- Client IP comes from RemoteAddr unless TRUST_PROXY=true, because
  X-Forwarded-For is attacker-controlled when the API is directly reachable.
  TRUST_PROXY is set only where an ingress or ALB is the sole path in.
- Follow-up: move counters to Redis (already a dependency) so limits hold across
  pods, and add AWS WAF rate rules at the edge in Phase 4.
