# Domain Monitoring & Asset Intelligence Platform

ระบบภายในสำหรับจัดการและตรวจสอบ domain assets โดยเน้น accuracy, explainability และ raw evidence auditability  
Internal platform for managing and monitoring domain assets with accuracy, explainability, and auditable raw evidence.

เอกสาร architecture ที่อนุมัติแล้วอยู่ที่ [`ARCHITECTURE.md`](ARCHITECTURE.md).  
The approved architecture is documented in [`ARCHITECTURE.md`](ARCHITECTURE.md).

## Implemented capabilities / ความสามารถที่พัฒนาแล้ว

### Phase 1

- Thai/English API locale negotiation (`Accept-Language: th|en`)
- PostgreSQL static migrations and constraints
- Redis durable-outbox dispatcher (PostgreSQL remains source of truth)
- Bootstrap local admin, secure Argon2id password hashing, opaque sessions, CSRF and RBAC
- Domain normalization with IDN/punycode and Public Suffix validation
- Domain CRUD, optimistic concurrency, archive/restore and append-only audit chain
- `/health`, `/ready`, structured JSON logs and versioned `/api/v1`

### Phase 2

- Local DNS wire queries for A, AAAA, CNAME and NS, including UDP truncation retry over TCP
- Cloudflare DoH POST wireformat (`application/dns-message`) with bounded responses
- DNS RCODE taxonomy, bounded retry with full jitter, CNAME loop/depth protection and local/DoH discrepancy comparison
- HTTPS-first HTTP checker with HTTP fallback only after a connection-stage failure
- Explicit redirect evidence for loops, limits, malformed locations, cross-domain targets and HTTPS downgrades
- Bounded decoded-body hashing, excerpt redaction, title extraction, content modes and response-header allowlisting
- Verified TLS inspection plus certificate-only diagnostics for invalid certificate evidence
- SSRF-safe pinned dialing with private/loopback/link-local/metadata/documentation-range and port protection

### Phase 3

- Durable scheduled/manual monitoring runs with policy snapshots and idempotency keys
- Centralized due-schedule claiming with `FOR UPDATE SKIP LOCKED`, coalescing and stable per-domain jitter
- PostgreSQL outbox → Redis Streams consumer group with at-least-once delivery and stale-message reclaim
- Configurable bounded worker pool using the shared Phase 2 protocol transports
- Transactional DNS, HTTP, redirect, TLS, content and classification evidence persistence
- Deterministic observed/effective state separation with 3-failure/2-success hysteresis
- Incident open/recovery/close events and immutable effective-state history
- 24h/7d/30d/90d uptime, coverage, response-time and incident history queries
- Authenticated manual check, run detail, domain run history and incident APIs

Phase 3 deliberately keeps ISP status `UNKNOWN` until Phase 4 supplies authenticated remote-probe evidence. A local/DoH discrepancy remains evidence and is never promoted to an ISP-block claim by this phase.

### Phase 4

- DoH-pinned local HTTPS/HTTP observation using Cloudflare DoH A/AAAA answers while preserving Host/SNI and the local network path
- Independent `/usr/local/bin/probe` with no PostgreSQL or Redis configuration/import dependency
- One-time registration token, locally generated Ed25519 identity, signed challenge and short-lived bearer tokens
- Outbound-only heartbeat, bounded claim and signed result APIs with lease, expiry, clock-skew, payload and replay checks
- Signed raw envelope plus normalized remote DNS/HTTP/TLS/redirect/content evidence in PostgreSQL
- SG deployment example with read-only root filesystem, dropped capabilities and a persistent private-key volume
- Cross-vantage ISP classifier for `UNKNOWN`, `NOT_DETECTED`, `SUSPECTED` and network-scoped `HIGH_CONFIDENCE_BLOCK`
- Stable reason codes and Thai/English API errors; DNS differences or HTTP 403/451 alone never become a high-confidence claim

### Phase 5

- IANA DNS bootstrap discovery and authoritative RDAP `/domain/{name}` queries with bounded payloads, retry/rate limiting, conditional bootstrap cache and stale-cache warnings
- Nullable RDAP normalization for registrar/IANA ID, registration/expiration/updated events, nameservers, DNSSEC and statuses; absent fields remain absent
- RDAP raw hash/excerpt, response metadata, confidence, field provenance and explicit conflict records without silently overwriting Sheet/manual data
- Google Sheets read-only connector with service-account OAuth, API-key/public-sheet and short-lived bearer-token modes; secrets remain environment/mounted-file only
- Semantic header mapping, staged preview, `ADD/MODIFY/UNCHANGED/MISSING/INVALID`, duplicate/invalid isolation, import history and transactional idempotent apply/reject
- Scheduled Sheet fetch creates a reviewable preview; only an authorized apply changes inventory. Missing rows become `missing_from_source` and are never deleted
- Exact rational-decimal money/tax/FX calculations serialized as strings, with inclusive/exclusive/exempt/unknown tax modes and non-annual billing cycles
- Price precedence (`registrar_api → google_sheet → manual → estimate`), immutable original currency, report-time FX snapshots/freshness and incomplete-budget warnings
- 30/60/90-day and current-year renewal windows plus audited manual overrides with original value, actor, reason, effective time and optional expiry

### Phase 6

- Button-oriented Google Drive OAuth start/callback APIs with state, PKCE, encrypted access/refresh tokens, bounded Google Sheet listing and revocable local connections
- Bounded `.xlsx` upload with compressed/uncompressed size, row and column limits; Excel uses the same staged preview/apply/history pipeline and never hard-deletes missing domains
- Deterministic rule-based `RENEW`, `DROP`, `REVIEW` and `PROFIT_OPPORTUNITY` recommendations with immutable input snapshots, Thai/English reasons, confidence and evidence references
- Conservative completeness policy: unknown monitoring/financial evidence becomes `REVIEW`; profit opportunity is an indicator only and never an invented monetary valuation
- Effective recommendation view honors audited, expiring manual overrides without rewriting the generated recommendation
- Set-based summary metrics plus exact-money renewal/tax/budget totals, completeness warnings, and immutable JSON/CSV report payloads with SHA-256

## Requirements / สิ่งที่ต้องมี

- Docker Desktop with Compose
- Copy `.env.example` to `.env` and replace every placeholder
- Host Go is optional; all project commands can run through the pinned Go container

## Development startup / เริ่มระบบสำหรับพัฒนา

```powershell
Copy-Item .env.example .env
# Edit .env and replace POSTGRES_PASSWORD and BOOTSTRAP_ADMIN_PASSWORD.
docker compose up --build -d
docker compose --profile tools run --rm seed-admin
```

API: `http://localhost:8081`  
Health: `http://localhost:8081/health`  
Readiness: `http://localhost:8081/ready`
Frontend: `http://localhost:3000` (see [Frontend / Admin Dashboard](#frontend--admin-dashboard))

## Login and locale / เข้าสู่ระบบและเลือกภาษา

```powershell
curl.exe -c cookies.txt -H "Accept-Language: th" -H "Content-Type: application/json" `
  --data '{"email":"admin@example.internal","password":"your-password"}' `
  http://localhost:8081/api/v1/auth/login
```

Login returns a `csrf_token`. Send the session cookie and `X-CSRF-Token` for every authenticated write. Error responses always include stable `code`, selected `message`, and both `messages.th`/`messages.en`.

## Commands / คำสั่ง

```text
make tidy              Resolve and pin Go modules
make fmt               Format Go source
make fmt-check         Verify formatting
make test              Run unit tests
make test-race         Run the race-enabled test suite
make test-live-doh     Explicit optional live Cloudflare DoH check
make vet               Run go vet
make docker-build      Run tests and vet in Docker build
make compose-up        Start development stack
make compose-down      Stop stack without deleting named volumes
make migrate           Apply pending migrations
make seed-admin        Idempotently create/grant the bootstrap admin
make integration-test  Run PostgreSQL/Redis integration tests
```

Phase 2 protocol details and fixture coverage: [`docs/phase2-protocol-checkers.md`](docs/phase2-protocol-checkers.md).
Phase 3 monitoring workflow, APIs and operational semantics: [`docs/phase3-monitoring-engine.md`](docs/phase3-monitoring-engine.md).
Phase 4 probe enrollment, deployment, protocol and ISP policy: [`docs/phase4-remote-probe-and-isp.md`](docs/phase4-remote-probe-and-isp.md).
Phase 5 RDAP, Google Sheets and financial engine: [`docs/phase5-asset-intelligence.md`](docs/phase5-asset-intelligence.md).
Phase 6 Drive/Excel imports, recommendations and reports: [`docs/phase6-recommendations-and-reports.md`](docs/phase6-recommendations-and-reports.md).

Direct migration examples:

```powershell
docker compose run --rm migration /usr/local/bin/migrate -dir /app/migrations status
docker compose run --rm migration /usr/local/bin/migrate -dir /app/migrations up
```

## Frontend / Admin Dashboard

Next.js admin dashboard in [`web/`](web/) — Domains, Dashboard, Incidents, Finance, Recommendations, Reports, Import Center (Google Drive/Sheets/Excel), Probe Nodes, and Settings. Talks to the Go API only through server-side BFF route handlers; the browser never calls the Go API directly and never sees a session token or Google OAuth token. See [`web/FRONTEND_ARCHITECTURE.md`](web/FRONTEND_ARCHITECTURE.md) for the full design and [`web/API_GAPS.md`](web/API_GAPS.md) for confirmed backend/frontend contract gaps.

### Run with the rest of the stack (Docker Compose)

```powershell
docker compose up --build -d
docker compose --profile tools run --rm seed-admin
```

This also builds and starts the `web` service from [`web/Dockerfile`](web/Dockerfile) (production, multi-stage, non-root, Next.js `output: "standalone"`). Inside Compose it reaches the API over the internal Docker network only (`API_INTERNAL_URL=http://api:8080`, never a host-exposed port), and is itself exposed only on `127.0.0.1:3000`.

Frontend: `http://localhost:3000`
Frontend health (used by its own Docker `HEALTHCHECK`): `http://localhost:3000/login` (expected 200)

### Run the frontend alone, against a running backend (development)

```powershell
docker compose up -d postgres redis migration api
cd web
npm install
Copy-Item .env.local.example .env.local
npm run dev
```

`http://localhost:3000` — hot-reloading Next.js dev server. `web/.env.local.example` only ever contains the non-secret `API_INTERNAL_URL=http://127.0.0.1:8081` placeholder; never add credentials to it.

### Frontend commands (run from `web/`)

```text
npm run dev             Start the Next.js dev server
npm run build            Production build (also run inside web/Dockerfile)
npm run start             Serve a production build (non-Docker)
npm run lint               ESLint (React Compiler + accessibility rules)
npm run format             Prettier --write
npm run format:check       Prettier --check
npm run typecheck            TypeScript strict typecheck
npm run test                   Unit/component tests (Vitest)
npm run test:e2e                 E2E tests (Playwright — see below)
```

### Frontend E2E tests (Playwright)

Most specs require a real backend and a seeded admin session, and skip cleanly without it:

```powershell
$env:E2E_ADMIN_EMAIL = "<seeded admin email>"
$env:E2E_ADMIN_PASSWORD = "<seeded admin password>"
# Optional — only needed for the specs that create/apply real data
# (domain creation, Excel/Sheet import apply, probe registration token):
$env:E2E_ALLOW_MUTATION = "true"
cd web
npm run test:e2e
```

Point `PLAYWRIGHT_BASE_URL` at a running Docker Compose `web` service (`http://127.0.0.1:3000`) to test the production build, or leave it unset to let Playwright start its own `next dev` server on `127.0.0.1:3100`. The suite covers: login/logout/session-expiry/401, domain create→detail→manual-check, finance/recommendations/reports, Google Drive OAuth (mocked-contract, no real Google account used), Excel import (real backend, fixture workbook, no secrets), probe registration tokens, settings, and a responsive/accessibility browser matrix (1440×900, 900×900, 390×844 — horizontal overflow, console errors, axe WCAG 2.2 AA serious/critical violations, keyboard navigation).

### Frontend troubleshooting

- **`web` container unhealthy / can't reach the API**: confirm `api` is healthy first (`docker compose ps`) — `web` depends on it via `condition: service_healthy`. Check `docker compose logs web`.
- **Login succeeds but every page bounces back to `/login`**: the session cookie didn't reach the browser. Verify `Set-Cookie` on `POST /api/bff/auth/login` includes `domainintel_session` (not just `bff_csrf`); if only the CSRF cookie is present, something changed the cookie-writing order in `web/src/app/api/bff/auth/login/route.ts` — `response.cookies.set()` must run before the backend's raw `Set-Cookie` headers are appended, not after.
- **Playwright specs skip immediately**: `E2E_ADMIN_EMAIL`/`E2E_ADMIN_PASSWORD` are unset — this is deliberate (never commit real credentials), not a failure.
- **`getByLabel`/`getByRole` "resolved to N elements" in a new spec**: Playwright's default text matching is a case-insensitive substring match — a short label like "Name" or "Menu" will also match a longer accessible name containing it (e.g. "Network name", "User menu: ..."). Add `{ exact: true }`.

## Security notes / หมายเหตุความปลอดภัย

- Do not commit `.env`, credentials, session tokens, probe keys, or Google credentials.
- Production must set `SESSION_COOKIE_SECURE=true`, use HTTPS, separate migration/runtime database roles, and replace bootstrap login with corporate OIDC.
- Domain archive is reversible and never deletes evidence.
- Redis is not a permanent source of truth.
