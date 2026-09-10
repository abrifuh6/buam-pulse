# ADR 0006: One tracked subscription per tenant

Status: Accepted · Date: 2026-09-10

## Context
Webhook handlers initially matched tenants by `stripe_customer_id` alone. A
customer in Stripe can hold several subscriptions — a stray Checkout, a
duplicate from a plan change, a legacy plan — and we hit exactly that: a second
Checkout created a parallel subscription, and cancelling it revoked a plan the
tenant was still paying for.

## Decision
- The tenant stores the ID of the ONE subscription that determines its plan.
- `customer.subscription.deleted` acts only when the deleted subscription is
  the tracked one.
- `customer.subscription.updated/created` applies only to the tracked
  subscription, or adopts one when none is tracked yet (first checkout).
- Plan changes for existing customers go through the Stripe billing portal,
  which modifies the existing subscription rather than creating a new one.

## Consequences
- Duplicate subscriptions no longer corrupt plan state, though they can still
  double-bill; detecting and reconciling them is a follow-up.
- `cancel_at_period_end` is tracked separately from `canceled`: a scheduled
  cancellation keeps full access until the period ends, and the UI must say so
  or customers assume the cancellation failed and try again.
