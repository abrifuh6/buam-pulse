# Roadmap

## v1 — shipped
Multi-tenant monitoring, alerting, status pages, teams, plans, billing,
retention. Running on Kubernetes with CI, a Helm chart, and a full local stack.

## Phase 4 — AWS
Terraform for a multi-account setup, EKS, RDS, ElastiCache, ECR, IRSA, ArgoCD,
multi-region workers, SES for mail.

## Phase 5 — SRE
SLOs and error budgets, alerting on Pulse itself, an external heartbeat, chaos
exercises, postmortems, runbooks.

## v2 — deliberately held back
These are useful features, but they are scheduled after Phase 5 on purpose:
each one is a good excuse to practise a deployment strategy on EKS with real
traffic and real rollback consequences, rather than deploying a no-op change.

| Feature | Deployment strategy it exercises | Why it fits |
|---|---|---|
| Multi-region check quorum | **Canary** | Workers in a second region start voting on failures. A bad canary produces false alerts, which is immediately visible and safely reversible. |
| Response-time SLO alerting | **Blue/green** | Changes alerting behaviour for every tenant at once. Blue/green gives an instant switch back if the new thresholds are noisy. |
| Public REST API with API keys | **Feature flag + progressive rollout** | Enable per tenant, watch error rates, widen. The flag outlives the deploy. |
| Webhook alert channel | **Rolling update** | Low blast radius, additive, the ordinary case — worth doing once to contrast with the others. |
| Status page custom domains | **Canary with traffic mirroring** | Certificate provisioning is hard to test without real traffic; mirroring lets the new path see production requests without serving them. |
| Incident timeline with updates | **Shadow deploy** | Write to the new schema alongside the old, compare, then cut over. Demonstrates an expand/contract migration. |

The point of the table is that the strategy comes first: each v2 feature was
chosen because it has the right risk profile to justify a particular rollout
technique, not the other way round.
