# Phase 3 — Monitoring Engine, Worker, Scheduler and History

Status: completed and validated on 2026-08-21.

## Runtime flow

1. Domain creation creates exactly one `monitor_schedules` row in the same transaction.
2. The centralized scheduler claims due rows with `FOR UPDATE SKIP LOCKED`.
3. That transaction creates a unique `monitoring_runs` intent, appends `job_outbox`, advances the schedule and coalesces missed intervals.
4. The outbox dispatcher publishes to a bounded Redis Stream. PostgreSQL remains the permanent source of truth.
5. `worker` consumes with a Redis consumer group and a fixed `MONITOR_WORKERS` pool.
6. A per-domain claim prevents simultaneous local runs. Redelivery sees completed runs and acknowledges without duplicating evidence.
7. Phase 2 collectors execute under the durable run deadline. One PostgreSQL transaction persists normalized evidence, classification reasons, effective-state transitions, incidents and run completion.
8. Redis is acknowledged only after the database commit.

Delivery is at least once; result persistence is idempotent through durable run status and unique keys. A worker crash before commit leaves the message pending for reclaim. A crash after commit but before `XACK` produces a harmless redelivery.

## Classification and hysteresis

`monitoring_results` stores the observation from one run. `domains.current_*` stores effective state and `domain_status_history` stores only effective transitions.

Defaults:

- First qualifying failure: effective availability becomes `DEGRADED`.
- Third consecutive qualifying failure: becomes `UNAVAILABLE` and opens an incident.
- First recovery success: becomes `DEGRADED` while the incident remains open.
- Second consecutive success: becomes `ACTIVE` and closes the incident.
- `UNKNOWN` evidence does not increment success/failure streaks.
- Manual checks persist full evidence but do not mutate effective state or incident streaks.

The thresholds and full protocol budgets are copied into every run's immutable `policy_snapshot`. Classification reasons use stable codes rather than parsing network error strings.

Phase 3 does not classify ISP blocking. DNS differences are stored as `DISCREPANCY`, while the independent ISP dimension stays `UNKNOWN` until Phase 4 adds fresh remote evidence.

## API

All endpoints are below `/api/v1`, require an authenticated session and return Thai/English error messages.

| Method | Endpoint | Role | Purpose |
|---|---|---|---|
| `POST` | `/domains/{domainID}/check` | ADMIN, STAFF | Queue an idempotent manual check; requires CSRF and `Idempotency-Key` |
| `GET` | `/monitoring-runs/{runID}` | ADMIN, STAFF, VIEWER | Run state plus normalized DNS/HTTP/TLS/redirect evidence |
| `GET` | `/domains/{domainID}/monitoring-runs` | ADMIN, STAFF, VIEWER | Paginated durable runs |
| `GET` | `/domains/{domainID}/monitoring-history?window=24h` | ADMIN, STAFF, VIEWER | Timeline and uptime/coverage aggregates; windows: 24h, 7d, 30d, 90d |
| `GET` | `/incidents?status=open` | ADMIN, STAFF, VIEWER | Paginated incidents |

Repeated manual requests with the same user, domain and `Idempotency-Key` return the same run. The key does not cause duplicate outbox messages.

## Configuration

| Variable | Default | Meaning |
|---|---:|---|
| `MONITOR_POLICY_VERSION` | `monitor-2026-08-v1` | Immutable classifier/policy identifier |
| `MONITOR_RUN_TIMEOUT` | `60s` | Whole durable local-run deadline |
| `MONITOR_WORKERS` | `10` | Fixed worker concurrency |
| `MONITOR_QUEUE_GROUP` | `domain-monitor-workers` | Redis consumer group |
| `MONITOR_QUEUE_LEASE` | `90s` | Pending-message/run recovery lease |
| `MONITOR_QUEUE_BLOCK` | `2s` | Bounded stream read wait |
| `SCHEDULER_INTERVAL` | `5s` | Central scheduler scan interval |
| `SCHEDULER_BATCH_SIZE` | `100` | Maximum schedules claimed per transaction |
| `INCIDENT_OPEN_FAILURES` | `3` | Failure streak required to open |
| `INCIDENT_CLOSE_SUCCESSES` | `2` | Recovery streak required to close |

## History math

Availability uses effective-state intervals rather than counting check rows:

```text
known_seconds = active_seconds + degraded_seconds + unavailable_seconds
uptime_percentage = active_seconds / known_seconds * 100
monitoring_coverage = known_seconds / requested_window_seconds * 100
```

`UNKNOWN` and unmonitored gaps do not inflate uptime. The API always returns monitoring coverage next to uptime. Average response time includes only completed HTTP checks with a final status; timeouts are not converted into invented latency values.

## Test boundary

Default tests are deterministic. Phase 3 integration tests exercise PostgreSQL migrations, API idempotency, schedule creation, outbox dispatch, Redis consumer groups, synthetic protocol evidence, transactional completion, run detail and history without public Internet. A live runtime smoke test is a separate explicit delivery action.
