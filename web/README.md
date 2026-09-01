# Domain Monitoring & Asset Intelligence Platform — Frontend

Next.js 16 (App Router, TypeScript strict) admin dashboard for the Go backend in the repository root. Talks to the backend only through server-side BFF (Backend-for-Frontend) route handlers under `src/app/api/bff/**` — the browser never calls the Go API directly, never holds a session token in JavaScript-readable storage, and never sees a Google OAuth token.

See the root [`README.md`](../README.md#frontend--admin-dashboard) for the quickest way to run this alongside the full stack. This file covers the frontend specifically.

## Pages

Dashboard, Domains (list/detail), Incidents, Finance, Recommendations, Reports, Import Center (Google Drive connect / Google Sheet sync / Excel upload, with staged-row review and import history), Probe Nodes, Settings, Login.

## Architecture

- **BFF pattern**: every session-sensitive or CSRF-protected call goes through `src/app/api/bff/[...path]/route.ts`, a generic catch-all that only forwards requests matching an explicit allowlist (`src/lib/api/bff-allowlist.ts`) — method + exact path pattern, with every `:param` segment validated as a UUID. This is what keeps the BFF from becoming an open proxy. A few non-JSON-envelope routes (login/logout, Excel multipart upload, report download streaming, Drive OAuth callback) have dedicated handlers instead.
- **Session**: backend's `HttpOnly` session cookie is forwarded as-is. The BFF additionally mints its own `HttpOnly` `bff_csrf` cookie from the `csrf_token` the backend returns in the login response body, and attaches it as `X-CSRF-Token` on every mutating request. Neither cookie is ever readable from browser JavaScript.
- **RBAC**: `src/lib/auth/capability.ts` maps ADMIN/STAFF/VIEWER to a capability matrix built directly from the `requireRoles(...)` call sites in `internal/api/server.go` — the UI hides/disables actions a role can't perform, but the backend remains the final authority.
- **Money/decimals**: every currency, tax, and FX amount stays a string end-to-end (`src/lib/utils/decimal-format.ts`) — never parsed through `Number()`/`parseFloat()` for display or math.
- **i18n**: next-intl, Thai default / English selectable, cookie-persisted, no URL locale prefix. All UI text lives in `src/messages/{en,th}.json`; `src/messages/messages.test.ts` asserts both catalogs have identical key sets and no empty values.

Full design notes: [`FRONTEND_ARCHITECTURE.md`](FRONTEND_ARCHITECTURE.md), [`API_CONTRACT_MATRIX.md`](API_CONTRACT_MATRIX.md), [`IMPLEMENTATION_PLAN.md`](IMPLEMENTATION_PLAN.md), [`API_GAPS.md`](API_GAPS.md).

## Running

```powershell
npm install
Copy-Item .env.local.example .env.local
npm run dev
```

Requires a reachable backend at the URL in `.env.local` (`API_INTERNAL_URL`, default `http://127.0.0.1:8081`). `.env.local.example` contains only that non-secret placeholder — never add credentials to it or commit `.env.local`.

Docker: see the root README's [Frontend / Admin Dashboard](../README.md#frontend--admin-dashboard) section — `docker compose up --build -d` builds and starts this service via [`Dockerfile`](Dockerfile) alongside the backend.

## Commands

```text
npm run dev             Dev server (Turbopack, hot reload)
npm run build            Production build (output: "standalone")
npm run start             Serve a production build
npm run lint               ESLint — includes React Compiler diagnostics and accessibility rules
npm run format              Prettier --write
npm run format:check         Prettier --check
npm run typecheck              tsc --noEmit (strict mode)
npm run test                     Vitest — unit/component tests
npm run test:e2e                   Playwright — see below
```

## Testing

**Unit/component** (`npm run test`): no backend needed. Covers the API envelope/error parser, BFF allowlist, RBAC capability matrix, decimal formatting, `returnTo` open-redirect validation, i18n catalog parity, and component-level RBAC gating (e.g. import review Apply/Reject visibility per role).

**E2E** (`npm run test:e2e`, Playwright): most specs need a real backend and a seeded session —

```powershell
$env:E2E_ADMIN_EMAIL = "<seeded admin email>"
$env:E2E_ADMIN_PASSWORD = "<seeded admin password>"
$env:E2E_ALLOW_MUTATION = "true"   # optional — only for specs that write real data
npm run test:e2e
```

They skip cleanly (not fail) when those env vars are absent. `PLAYWRIGHT_BASE_URL` points them at any running instance (dev server or the Docker `web` container on `127.0.0.1:3000`); left unset, Playwright starts and manages its own dev server.

Coverage: login/logout/session-expiry/401, domain create → detail → manual check, finance/recommendations/reports, Google Drive OAuth as a **mocked contract** test (the BFF's Drive endpoints and the OAuth popup's destination are intercepted at the browser network layer — no real Google account or consent screen is used, and this must never be described as verifying real Google OAuth), Excel import against the real backend with a checked-in fixture workbook containing no secrets, probe registration-token one-time reveal, settings, and a responsive/accessibility browser matrix at 1440×900 / 900×900 / 390×844 (horizontal overflow, console errors, axe-core WCAG 2.2 AA serious/critical violations, keyboard navigation including the mobile nav drawer).

## Known limitations

- **RBAC E2E coverage is ADMIN-only.** The backend has no user-management endpoint (see `API_GAPS.md` GAP-06) and the only available test account is the bootstrap admin, so STAFF/VIEWER role-gating is verified at the unit/capability-matrix level and by code review of each component's `hasCapability(...)` checks, not by a live STAFF/VIEWER session driving the real UI.
- **Real Google OAuth consent has not been tested.** The Drive connect flow is verified as a mocked-contract test only (see above). Do not report or imply that a real Google account has completed the consent screen.
- **Import Center's Excel fixture** (`e2e/fixtures/import-fixture.xlsx`) uses a `.com`-style domain, not `.example` — the backend's `internal/domain.Normalizer` rejects the RFC 2606 `.example` TLD unless `ALLOW_UNKNOWN_TLD=true` (see `API_GAPS.md` GAP-11); the same applies to `domains.spec.ts`'s generated test domain.
