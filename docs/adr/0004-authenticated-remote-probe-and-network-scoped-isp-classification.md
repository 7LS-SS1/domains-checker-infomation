# ADR 0004: Authenticated remote probe and network-scoped ISP classification

Status: Accepted  
Date: 2026-08-21

## Context

Local system DNS, Cloudflare DoH and local HTTP observations cannot by themselves distinguish DNS interference, path filtering, origin failure, geo policy or WAF behavior. A remote node must remain safe to operate outside the trusted database network and its evidence must be attributable and replay-resistant.

## Decision

- The probe is an independent binary. It has no database or Redis credential and communicates outbound to the platform only over HTTPS.
- Enrollment uses an admin-created one-time token bound to name, region, country and network. The probe creates its Ed25519 private key locally; the private key never leaves its state volume.
- The same token endpoint issues a persisted one-time challenge, then a short-lived scoped bearer token after Ed25519 verification.
- Jobs contain a canonical domain, fixed schemes (`https`, `http`), fixed ports (`443`, `80`) and bounded policy. Arbitrary URL paths, methods, headers, bodies, credentials and ports are not part of the protocol.
- Results carry the exact JSON payload hash, job nonce and Ed25519 signature. The server checks identity metadata, protocol version, lease ownership, expiry, clock skew, payload size and nonce uniqueness before commit.
- Local collection adds DoH-pinned HTTP as a separate `resolver_mode`; it does not change the availability result produced by the system-DNS request.
- ISP is a separate dimension. High confidence requires repeated local failure in multiple time buckets, repeated fresh SG success with stable content hash, stable DoH, compatible local pinned failure and a healthy probe/clock.
- `HIGH_CONFIDENCE_BLOCK` is always described as applying to the monitored local network path. Confidence is capped at 99 and is not a universal or legal confirmation.

## Consequences

Remote compromise does not expose platform database credentials. Evidence remains independently verifiable after ingestion, and replayed results fail closed. A new SG node can backfill recent completed local runs within the configured freshness window. Coverage gaps and stale/offline probes result in `UNKNOWN`, not a guessed classification.
