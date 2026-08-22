# ADR 0003: Durable Phase 3 monitoring runtime

Status: Accepted  
Date: 2026-08-21

## Context

Phase 2 produced deterministic protocol collectors but intentionally did not create durable jobs, persist evidence or mutate effective domain state. Phase 3 must tolerate Redis/process failure, prevent uncontrolled concurrency and avoid treating one transient request as definitive downtime.

## Decision

- PostgreSQL `monitoring_runs` and `job_outbox` are durable intent; Redis Streams is transient delivery.
- Scheduler replicas share due work using row locks with `SKIP LOCKED` and unique schedule-slot keys.
- Worker delivery is at least once. A Redis message is acknowledged only after the evidence transaction commits.
- One active local execution per domain is enforced with a database-backed run check and advisory transaction lock.
- Worker concurrency is fixed by configuration; shared DNS/HTTP transports retain their Phase 2 bounds.
- Observed classifications are immutable. Effective availability uses versioned failure/recovery hysteresis and transition history.
- Manual checks are fully auditable observations but are non-qualifying for effective-state and incident streak changes.
- ISP remains `UNKNOWN`; Phase 3 cannot infer network blocking from local/DoH evidence alone.

## Consequences

Redis can be restarted without becoming the permanent source of truth, and redelivery cannot duplicate committed evidence. Effective status may intentionally lag a failed observation while waiting for the configured threshold. Manual diagnostics remain visible without allowing repeated button presses to open or close incidents.
