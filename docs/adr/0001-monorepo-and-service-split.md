# ADR 0001: Monorepo with three Go services

Status: Accepted · Date: 2026-09-08 · Author: Buam Technologies

## Context
Northgate Digital needs a monitoring platform that can grow from 40 to thousands
of monitors without re-architecture. Checks are I/O-bound and bursty.

## Decision
- One repository for all services and infrastructure, so a single PR can change
  an API, its Helm values, and its Terraform together.
- Three services: `api` (stateless, user-facing), `scheduler` (single replica,
  decides what is due), `worker` (stateless, N replicas, does the checks).
- Redis list as the work queue. Identical behaviour on laptop, k3s, and EKS.

## Consequences
- Workers scale independently of the API on queue depth.
- Scheduler is a single point of failure; mitigated by fast restart and
  `FOR UPDATE SKIP LOCKED` making a second replica safe if ever needed.
- Redis queue is at-most-once; a lost check is acceptable (the next interval
  re-checks). Revisit if durability requirements change.
