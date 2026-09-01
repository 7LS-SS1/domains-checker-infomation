# Frontend Delivery Report — Domain Monitoring & Asset Intelligence Platform

Covers `web/`, the Next.js admin dashboard built against the existing Go backend, through Phases Frontend 1–8 of `CLAUDE_FRONTEND_PROMPTS.md`.

## Result

**Production-ready for the verified scope.** Every gate below actually ran, including a full authenticated Playwright suite against both a live-backend dev server and the production Docker image, repeated to confirm no flakiness. The scope that was *not* verified (real Google OAuth consent, non-admin RBAC sessions) is called out explicitly under Known limitations — do not read this report as claiming those.

## Architecture

- **BFF pattern**: `src/app/api/bff/[...path]/route.ts` is a generic catch-all that only forwards a request matching an explicit method+path allowlist (`src/lib/api/bff-allowlist.ts`), with every `:param` validated as a UUID — the mechanism that keeps it from being an open proxy. Non-JSON-envelope routes (login/logout, Excel multipart upload, report download streaming, Drive OAuth callback) have dedicated handlers.
- **Session/CSRF**: backend's `HttpOnly` session cookie forwarded as-is; the BFF separately mints its own `HttpOnly` `bff_csrf` cookie from the login response body's `csrf_token`, attached as `X-CSRF-Token` on mutating requests. Neither is ever readable from browser JS (asserted in `e2e/login.spec.ts`).
- **RBAC**: `src/lib/auth/capability.ts` — a capability matrix built directly from `requireRoles(...)` call sites in `internal/api/server.go`. UI hides/disables what a role can't do; backend remains final authority.
- **Money/decimals**: every currency/tax/FX value stays a string end-to-end; never parsed through `Number()`/`parseFloat()`.
- **i18n**: next-intl, Thai default, cookie-persisted. `src/messages/messages.test.ts` asserts en/th catalogs have identical keys and no empty values.

Full design docs: `web/FRONTEND_ARCHITECTURE.md`, `web/API_CONTRACT_MATRIX.md`, `web/IMPLEMENTATION_PLAN.md`, `web/API_GAPS.md`.

## Pages (all 11 required pages built)

Login, Dashboard, Domains (list + detail), Incidents, Finance, Recommendations, Reports, Import Center (Google Drive connect / Google Sheet sync / Excel upload / import history), Probe Nodes, Settings.

## Main files added/changed this session

- `web/src/components/probes/*`, `web/src/lib/probes/*`, `web/src/app/(app)/probes/page.tsx` — Probe Nodes page, registration-token one-time reveal, revoke flow.
- `web/src/components/settings/*`, `web/src/lib/system/*`, `web/src/app/(app)/settings/page.tsx` — Settings/system status page.
- `web/src/components/sync/import-history-tab.tsx` — import history list/detail, completing the Import Center.
- `web/Dockerfile`, `web/.dockerignore`, `docker-compose.yml` (additive `web` service) — Phase 8 Docker integration.
- `web/e2e/probes.spec.ts`, `web/e2e/settings.spec.ts`, `web/e2e/sync.spec.ts`, rewritten `web/e2e/responsive.spec.ts` (axe-core WCAG 2.2 AA + full authenticated-page browser matrix), `web/e2e/fixtures/import-fixture.xlsx`.
- `web/src/messages/{en,th}.json` — probes/settings/sync.history namespaces; `web/src/messages/messages.test.ts` — new i18n parity test.
- Real bugs found via live-backend/Docker testing and fixed (see below).

## Real bugs found and fixed via live verification

Static review and unit tests could not have caught these — each was only found by actually running against a live backend and, in most cases, the production Docker image:

1. **Login was silently broken.** `src/app/api/bff/auth/login/route.ts` set the BFF's own CSRF cookie via `response.cookies.set()` *after* appending the backend's raw session `Set-Cookie` header — Next.js's cookie-jar API rebuilds the `Set-Cookie` header from its own internal state when `.set()` runs, discarding the earlier raw append. Every login returned 200 with correct user data but no session cookie ever reached the browser; the very next protected navigation correctly bounced back to `/login`, silently, with no visible error. Fixed by reordering (CSRF cookie first, then the raw append).
2. **Excel import crashed on the server.** `ExcelTab`'s Zod schema referenced the browser-only `FileList` global inside the render body, throwing `ReferenceError: FileList is not defined` during Next.js's SSR pass. Fixed with `z.custom<FileList>()` guarded by `typeof FileList !== "undefined"`.
3. **Wrong i18n namespace** on Domain Detail's "Run check now" button (`domains.actions` instead of `domainDetail.actions`) — the button rendered the literal missing-message fallback text instead of its label.
4. **Duplicate "Reports" heading** — a summary card reused the page-level title key instead of its own.
5. **A real WCAG 2.2 AA violation**: the login form's password-visibility toggle button was 18×40px with insufficient spacing to the input (`target-size`, axe/WCAG 2.5.8). Fixed to a full 40×40px touch target.
6. **Non-keyboard-focusable scrollable tables** (`scrollable-region-focusable`, WCAG 2.1.1/4.1.2) on every data table in the app — fixed uniformly with `tabIndex={0}` + `role="region"` + a label.
7. **A page-header row that didn't wrap** at narrow viewports (`ProbesPageContent`, and the shared `CardHeader` primitive used everywhere) — fixed with `flex-wrap`.
8. **Backend domain-validation behavior**: `internal/domain.Normalizer` rejects the RFC 2606 `.example` TLD unless `ALLOW_UNKNOWN_TLD=true` (default off) — both the Excel fixture and `domains.spec.ts`'s generated test domain used `.example` and were silently failing domain validation. Switched to `.com`-style test domains (see `web/API_GAPS.md` GAP-11).
9. Several Playwright locator ambiguities (`getByLabel("Password")` also matching the "Show password" button via substring match, etc.) — fixed with `{ exact: true }` throughout.
10. A responsive-test false positive: `document.documentElement.scrollWidth` reports a page's full theoretical content extent largely independent of `overflow` CSS, and does not reliably shrink even when a wide table is correctly contained in its own `overflow-x-auto` region — `document.body.scrollWidth` does not have this quirk and was confirmed accurate via a `getBoundingClientRect()` sweep finding no element actually exceeding the viewport. The test now measures `document.body`, not `document.documentElement`.

## Commands run and results

All run from `web/` unless noted.

| Gate | Command | Result |
|---|---|---|
| Format | `npx prettier --check .` | ✅ clean |
| Lint | `npx eslint .` | ✅ 0 errors, 1 pre-accepted warning (`react-hooks/incompatible-library` on TanStack Table's `useReactTable()` — a known, harmless React Compiler diagnostic) |
| Typecheck | `npx tsc --noEmit` | ✅ clean |
| Unit/component tests | `npx vitest run` | ✅ **165/165 passed**, 21 files |
| Production build | `npm run build` | ✅ 18 routes compiled |
| Docker build | `docker compose build web` | ✅ |
| Docker Compose up | `docker compose up -d` (postgres, redis, migration, api, web) | ✅ all 4 long-running services report `healthy` |
| Backend health/ready | `GET /health`, `GET /ready` | ✅ 200, `{"status":"ready","checks":{"postgres":"ok","redis":"ok"}}` |
| Frontend health | `GET http://127.0.0.1:3000/login` | ✅ 200 |
| **Full E2E suite**, real backend, real Docker image, `E2E_ALLOW_MUTATION=true` | `npx playwright test --workers=1` | ✅ **23/23 passed** (login, domain create→detail→manual-check, finance, recommendations, reports, Drive mocked-contract, Excel real-backend fixture, probe registration-token, settings, 3×3 responsive/a11y matrix) |
| Responsive/a11y matrix, repeated for stability | `npx playwright test e2e/responsive.spec.ts` ×2 more | ✅ **9/9 passed** both times (no flakiness) |

## Browser/responsive/accessibility matrix

Desktop 1440×900, tablet 900×900, mobile 390×844 — each checked against every real authenticated page (Dashboard, Domains, Incidents, Finance, Recommendations, Reports, Sync, Probes, Settings) plus Login, for:
- No page-level horizontal overflow
- No console errors or uncaught exceptions (network-level 4xx logging for intentionally-handled cases like an unconfigured Google Drive connection is filtered out — that's correct app behavior, not a bug)
- No WCAG 2.2 AA serious/critical violations (axe-core, tags `wcag2a`/`wcag2aa`/`wcag22aa`)
- Keyboard navigation, including opening the mobile nav drawer below the `lg` breakpoint and reaching/activating a nav link with no mouse

All passed, on the production Docker image, three consecutive runs.

## Known limitations (verify before calling this complete for your purposes)

- **RBAC E2E coverage is ADMIN-only.** The backend has no user-management endpoint (`web/API_GAPS.md` GAP-06) and only a bootstrap admin account exists in this environment — STAFF/VIEWER role gating is verified at the unit/capability-matrix level (`src/lib/auth/capability.test.ts`) and by code review of each component's `hasCapability(...)` checks, not by a live non-admin session driving the real UI end to end.
- **Real Google OAuth consent has not been tested.** The Drive connect flow is verified only as a mocked-contract test (`e2e/sync.spec.ts` — the BFF's Drive endpoints and the OAuth popup destination are intercepted at the browser network layer; no real Google account or consent screen is used). Do not represent this as "Google OAuth verified."
- **Performance**: no formal Lighthouse/LCP budget run was performed this session; bundle composition and client-component boundaries were reviewed during development but not re-audited as a dedicated pass.
- **`web/API_GAPS.md`** documents remaining real backend/frontend contract gaps (missing OpenAPI entries for several live routes, RBAC doc drift, no incident-detail/settings/audit-log/user-management endpoints, Idempotency-Key payload-hashing gap, the `.example` TLD validation behavior found this session).

## URLs

- Frontend (Docker Compose): `http://localhost:3000`
- Frontend (dev server): `http://localhost:3000` (or `3100` under Playwright's managed dev server)
- Backend API: `http://localhost:8081` — `/health`, `/ready`

## Git status

Not committed — per the Master Prompt, this frontend work stops short of `git add`/`commit`/`push` until explicitly instructed. Current working tree:
- Modified: `README.md` (new Frontend section), `docker-compose.yml` (additive `web` service). `.env.example` was already modified before this session started and was not touched.
- New, untracked: `web/` (the entire frontend), `FRONTEND_DELIVERY_REPORT.md` (this file). `CLAUDE_FRONTEND_PROMPTS.md` and `cookies.txt` predate this session and are unrelated to it.

Docker containers from this session's verification (`postgres`, `redis`, `api`, `web`) are currently still running and healthy, left up in case further manual verification is wanted — stop with `docker compose down` when done (add `-v` only if you also want to discard the database volume, which is not recommended without checking what's in it first).
