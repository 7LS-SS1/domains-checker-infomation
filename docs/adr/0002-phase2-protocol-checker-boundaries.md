# ADR 0002: Phase 2 protocol checker boundaries

- Status: Accepted
- Date: 2026-08-20

## Decision

1. Protocol checkers return immutable attempt evidence and typed errors; they do not directly mutate domain status.
2. HTTP redirects are traced with a shared `RoundTripper`, not `http.Client` automatic redirect handling.
3. Production HTTP/TLS dialing always resolves, validates and pins an allowed IP for each hop.
4. Invalid TLS verification may trigger a second diagnostic handshake to the already validated remote address. It reads certificate evidence only, sends no application request and can never yield `VALID`.
5. The normal test suite uses deterministic local fixtures. Public Cloudflare DoH is an explicit live-tag test.

## Consequences

- Phase 3 can persist every retry/hop without parsing log strings or reconstructing discarded evidence.
- DNS discrepancy and TLS/content failures remain separate dimensions and cannot independently claim ISP blocking or ownership failure.
- Internal/private monitoring requires an explicit future named allowlist rather than weakening the public-target SSRF policy.
