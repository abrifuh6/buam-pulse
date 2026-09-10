# ADR 0004: Outbound request safety (SSRF and abuse)

Status: Accepted · Date: 2026-09-10

## Context
Pulse fetches customer-supplied URLs on a schedule from inside our VPC. Without
restriction a customer could point a monitor at 169.254.169.254 to read cloud
instance credentials, at internal services behind the firewall, or at a third
party at high frequency to use Pulse as a DDoS amplifier.

## Decision
- Validate at creation: reject loopback, RFC1918, link-local, CGNAT, multicast,
  unspecified addresses, and .internal/.local/localhost names.
- Re-check at request time (ResolveGuard): a hostname that validated at creation
  can be re-pointed at a private address later (DNS rebinding).
- Enforce a minimum check interval of 30s per monitor, and per-tenant monitor
  limits, so aggregate request volume against any one target stays bounded.
- Workers run in private subnets with no IAM instance-profile access to
  sensitive APIs; IRSA scopes pod permissions narrowly (Phase 4).

## Consequences
- Customers cannot monitor their own internal services with the hosted product.
  That is the correct default; a self-hosted or agent-based option is the answer
  for that use case, not relaxing the guard.
- ResolveGuard costs a DNS lookup per check. Acceptable; Go caches nothing here
  by default, and the lookup is what makes the guarantee real.
