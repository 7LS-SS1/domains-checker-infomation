# Phase 2 — Protocol Checkers

Status: completed and validated on 2026-08-20.

## Scope

Phase 2 supplies deterministic, context-aware components under `internal/`:

- `dnscheck`: local DNS wireformat, Cloudflare DoH wireformat, A/AAAA/CNAME/NS evidence, UDP-to-TCP truncation recovery, CNAME protection and resolver comparison.
- `httpcheck`: HTTPS-first origin checks, explicit redirect tracing, timing, bounded content evidence and typed HTTP/content failures.
- `tlscheck`: verified TLS inspection and a certificate-only diagnostic handshake after verification failure.
- `safedial`: DNS resolution, address-set validation, IP pinning, port policy and SSRF protection for every HTTP redirect hop.
- `retry`: configurable exponential backoff with full jitter and cancellation-aware waiting.
- `netcheck`: stable typed error codes shared by protocol components and future classifiers.
- `protocolcheck`: configuration-to-component wiring for the Phase 3 monitoring worker.

These components do not update a domain's effective status. A single observation is evidence only. Phase 3 will create durable runs, persist every attempt, apply multi-observation classification and update history/incidents.

## Security boundaries

Production HTTP/TLS checks use the safe dialer. It rejects loopback, private, link-local, multicast, carrier-grade NAT, documentation/benchmark ranges, cloud metadata paths and ports other than 80/443 unless an explicit named policy allowlist is supplied. The full resolved answer set is validated before any candidate IP is dialed, and the connection is pinned while preserving the original HTTP Host and TLS SNI.

Redirects are processed by `RoundTripper` directly. Automatic redirect handling is not used, so malformed `Location` values and every 301/302/303/307/308 hop remain observable. HTTPS-to-HTTP downgrade is recorded and not followed.

DoH uses an unredirected POST containing unencoded DNS wire bytes and requires `application/dns-message`. The response is bounded before unpacking, following Cloudflare's documented wireformat contract: [Cloudflare DNS wireformat](https://developers.cloudflare.com/1.1.1.1/encryption/dns-over-https/make-api-requests/dns-wireformat/).

## Default budgets

| Policy | Default |
|---|---:|
| DNS attempt | 3s |
| TCP connect | 5s |
| TLS handshake | 7s |
| Response header | 10s |
| HTTP redirect chain | 25s |
| Operation attempts | 3 |
| Redirect hops | 10 |
| Decoded body | 2 MiB |
| Stored excerpt | 32 KiB |
| Selected response headers | 16 KiB |
| TLS expiry warning | 30 days |

All values are configurable through the `MONITOR_*` and DoH variables documented in `.env.example`.

## Deterministic coverage

The default suite uses local DNS and HTTP/TLS fixtures only. It covers:

- NOERROR records with TTL, NXDOMAIN, SERVFAIL, REFUSED, timeout, cancellation and UDP truncation followed by TCP.
- Cloudflare-compatible DoH POST wireformat and local/alternate answer comparison.
- CNAME loop and maximum-depth handling.
- 200, 301→200, 301→308→200, loop, hop limit, malformed location and HTTPS downgrade.
- HTTP timeout, connection refused, 403, 451, 500 and retryable 503.
- meaningful HTML, blank/too-small HTML, non-HTML modes and oversized chunked/compressed bodies.
- bounded/redacted excerpts, bounded header allowlist and reusable connections.
- TLS valid, expiring, expired, hostname mismatch, unknown authority and handshake failure.
- safe-dial rejection and explicit fixture allowlisting.

The live Cloudflare test is opt-in:

```powershell
make test-live-doh
```

Normal CI never requires public Internet. HTTP timing uses Go's per-request trace hooks; reused connections leave unavailable stage timings at zero instead of inventing measurements. See [Go `httptrace`](https://pkg.go.dev/net/http/httptrace).
