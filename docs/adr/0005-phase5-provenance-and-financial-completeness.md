# ADR 0005: Phase 5 provenance and financial completeness

- Status: Accepted
- Date: 2026-08-21

## Context

RDAP, Google Sheets, registrar data and manual decisions may provide missing or contradictory registrar, expiration and price values. A budget that silently treats absent tax/FX as zero is materially misleading. Sheet row deletion must not destroy inventory.

## Decision

1. Preserve observations in append-oriented RDAP/import/cost/provenance history and resolve effective data by explicit source precedence.
2. RDAP never silently overwrites inventory-owned registrar/expiration. Differences create reviewable conflicts.
3. Google Sheets uses persisted preview/apply with semantic mapping and idempotency. Missing rows set `missing_from_source`; scheduled fetch stops at preview.
4. Manual overrides are separate, audited, reasoned, time-bounded records and have effective precedence without erasing source observations.
5. Money and rates cross JSON/database boundaries as decimal strings/`NUMERIC`; calculations use exact rational arithmetic.
6. Unknown tax and missing/stale FX make aggregates incomplete with warning/count, never implicit zero.

## Consequences

- More history rows and explicit review work are accepted in exchange for explainability and recovery.
- Admin UI in Phase 7 can render conflicts/import previews without reconstructing them from logs.
- Reports can distinguish a true zero from an unavailable amount.
- Scheduled Sheet sync cannot autonomously mutate inventory until a later policy explicitly authorizes auto-apply.

