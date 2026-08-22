# Phase 4 — Remote probe, DoH-pinned HTTP and ISP classification

Status: completed and validated on 2026-08-21.

## Evidence topology

Every qualifying local run can now produce five distinguishable signals:

1. `LOCAL_SYSTEM` DNS
2. `CLOUDFLARE_DOH` DNS using RFC wireformat over HTTPS
3. local HTTP(S) with `resolver_mode=system`
4. local HTTP(S) with `resolver_mode=doh_pinned`
5. remote DNS/HTTP/TLS/content with `vantage_type=remote`, `resolver_mode=remote_system`

DoH-pinned mode pins the original hostname to the validated Cloudflare A/AAAA answer while retaining the original Host header and TLS SNI. Traffic still leaves through the local network, so success can support DNS-interference suspicion but is not foreign-vantage evidence. The implementation uses Cloudflare's documented `https://cloudflare-dns.com/dns-query` endpoint and unencoded POST body with `application/dns-message`.

## Enrollment and authentication

Human administration endpoints require the normal session, RBAC and CSRF controls:

| Method | Endpoint | Role | Purpose |
|---|---|---|---|
| `POST` | `/api/v1/probes/registration-tokens` | ADMIN | Create a one-time token bound to expected node metadata |
| `GET` | `/api/v1/probes` | ADMIN, STAFF, VIEWER | List health, version, region and last-seen state |
| `POST` | `/api/v1/probes/{probeID}/revoke` | ADMIN | Revoke identity/tokens and cancel queued leases |

System endpoints are separate from human authentication:

| Method | Endpoint | Authentication |
|---|---|---|
| `POST` | `/api/v1/probe-auth/register` | One-time registration token |
| `POST` | `/api/v1/probe-auth/token` | Challenge request, then Ed25519-signed challenge |
| `POST` | `/api/v1/probe-agent/heartbeat` | Short-lived bearer token |
| `POST` | `/api/v1/probe-agent/jobs/claim` | Short-lived bearer token; wait is capped at 10 seconds |
| `POST` | `/api/v1/probe-agent/jobs/{jobID}/result` | Bearer token plus signed result envelope |

Challenges are single-use and persisted. Result replay is stopped by job state plus unique `(probe_node_id, nonce)` storage. The server rejects incompatible identity metadata, expired challenge/token/job/lease, payloads over `PROBE_MAX_PAYLOAD_BYTES`, excessive clock skew and invalid Ed25519 signatures.

## SG deployment

1. An administrator creates a registration token and writes only the returned token value to a root-readable secret file on the SG host.
2. Set `DOMAIN_MONITOR_IMAGE`, `PROBE_API_URL`, `PROBE_REGISTRATION_TOKEN_FILE` and optional network metadata.
3. Deploy [`deploy/sg-probe.compose.yml`](../deploy/sg-probe.compose.yml).
4. Confirm `/api/v1/probes` reports `ONLINE`, the expected version and acceptable clock offset.

```powershell
docker compose -f deploy/sg-probe.compose.yml up -d
```

The deployment supplies no `DATABASE_URL`, `REDIS_ADDR` or inbound port. The private key is stored with mode `0600` under `/var/lib/domain-monitor-probe`; only the public key is registered. Production `PROBE_API_URL` must be HTTPS. Plain HTTP requires an explicit agent flag intended only for isolated local development.

## Classification policy

`NOT_DETECTED` is emitted when fresh local content succeeds. Remote missing/stale/offline evidence, global local+remote failure, unstable DoH, possible WAF/geo behavior or unhealthy clock/identity yields `UNKNOWN`.

`SUSPECTED` requires a meaningful local/remote divergence. DNS-only interference is distinguished when system DNS differs or fails but DoH-pinned local HTTP succeeds. A path/filtering suspicion is used when the SG content succeeds and both local system and DoH-pinned paths fail.

`HIGH_CONFIDENCE_BLOCK` requires all of the following:

- at least three qualifying local failures;
- failures span at least two five-minute evidence buckets;
- at least two fresh remote successes in the configured SG region;
- stable non-empty remote body hash and stable local DoH answer set;
- compatible local and pinned failure stage;
- healthy `ONLINE` probe and acceptable clock;
- no detected maintenance, global-origin, WAF or geo-policy explanation.

The status means “high-confidence block on the monitored local network path.” It does not claim a country-wide, universal or legal block. Confidence never exceeds 99.

## Configuration

| Variable | Default | Meaning |
|---|---:|---|
| `PROBE_REQUIRED_REGION` | `SG` | Region eligible for cross-vantage jobs |
| `PROBE_TOKEN_TTL` | `5m` | Short-lived agent bearer token |
| `PROBE_CHALLENGE_TTL` | `1m` | One-time challenge lifetime |
| `PROBE_JOB_TTL` | `2m` | Whole remote job lifetime |
| `PROBE_JOB_LEASE` | `90s` | Claim lease; must be shorter than job TTL |
| `PROBE_STALE_AFTER` | `90s` | Last-seen threshold for `OFFLINE` |
| `PROBE_MAX_CLOCK_SKEW` | `2m` | Heartbeat/result clock policy |
| `PROBE_EVIDENCE_FRESHNESS` | `24h` | Cross-vantage evidence window |
| `PROBE_DISPATCH_BATCH` | `100` | Jobs created per scheduler pass |
| `PROBE_MAX_PAYLOAD_BYTES` | `1048576` | Signed payload limit |

## Validation boundary

Deterministic tests cover signature tampering, single-use challenge/result replay, expiry, clock skew, payload limit, revocation, upgrade-required state, fixed target ports, DoH-pinned resolver behavior, DNS-interference/path/global-origin branches and stale remote evidence. The integration suite exercises the complete API and PostgreSQL workflow with synthetic network evidence; it does not require public Internet or an actual SG host.
