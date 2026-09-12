# ADR 0007: Split by workload, not by domain

Status: Accepted · Date: 2026-09-11

## Context
Pulse runs five Go services that share one PostgreSQL database. The obvious
question is whether this is a microservice architecture. It is not, and the
distinction is worth being precise about.

## Decision
Services are separated by scaling and failure characteristics, not by domain
ownership:

- **worker** — checks are I/O-bound and bursty; replica count tracks customer
  count, independently of API traffic.
- **scheduler** — must run as exactly one replica. Embedding it in a service we
  want several copies of is not possible.
- **notifier** — a slow or failing email provider must never delay a check.
- **retention** — a heavy periodic job that should not compete with request
  handling for connections or CPU.
- **api** — request/response, scales with user traffic.

They share a database and communicate through a Redis queue rather than by
calling each other.

## Consequences
- This is a modular monolith with workload-based process separation, not
  microservices. No service owns its data; a schema change can affect several
  at once, which migrations must account for.
- In exchange there are no network hops on the write path, no distributed
  transactions, and no eventual consistency to reason about — none of which
  this product would gain anything from.
- If a single service ever needed independent scaling of its *data* rather than
  its compute, that would be the signal to split the database. Nothing does yet.
