# Implementation Plan — Phase Frontend 1 output

This plan sequences Phase Frontend 2+ work. Each milestone has an explicit test gate; no milestone starts application code beyond what its own phase authorizes (Phase Frontend 1 itself produces **no runtime code**, only these four documents). Milestone numbering is independent of the backend's Phase 1–6 numbering already used in this repository — these are **frontend** milestones (FE-M0…FE-M9).

---

## FE-M0 — Toolchain & scaffold

**Scope:** `create-next-app` (App Router, TypeScript strict, Tailwind) inside `web/`; ESLint/Prettier config; Vitest + React Testing Library setup; Playwright install + config; `.env.local.example` (placeholders only, no real secrets); base `tsconfig.json` strict flags; CI-equivalent local scripts (`lint`, `typecheck`, `test`, `test:e2e`, `format`).

**Depends on:** none (can start immediately in Phase Frontend 2).

**Test gate:** `npm run lint`, `npm run typecheck`, `npm run test` (empty/smoke suite) all exit 0. `npm run build` succeeds against a placeholder page. No `.env` values committed — verify with `git status`/`git diff` before any commit.

---

## FE-M1 — BFF core: auth, session, CSRF

**Scope:** Route Handlers for `login`, `logout`, `auth/me`; cookie-forwarding proxy helper shared by all future BFF routes; `ApiError` type + shared error-normalizing fetch wrapper; root authenticated layout with 401 redirect; Login page (form, validation via Zod, error rendering with `request_id`).

**Depends on:** FE-M0.

**Test gate:**

- Unit: Zod schema validation for login form (empty email/password, malformed email).
- Component (RTL): Login page renders `error.message` + `request_id` on a mocked 401; redirects to `/dashboard` on mocked 200.
- Integration/E2E (Playwright, against a running backend per `README.md` compose instructions): real login with seeded bootstrap admin succeeds; session cookie is `HttpOnly` (assert via `page.context().cookies()` that the cookie is not readable from `document.cookie` in-page); logout clears session and redirects to `/login`; hitting an authenticated route while logged out redirects to `/login`.
- Security spot-check: confirm no `csrf_token` or session token value ever appears in a `NEXT_PUBLIC_*` env read, `localStorage`, or a response body sent to the browser (grep the built client bundle for the literal string `csrf_token` returning zero matches, or assert via a Playwright network-log check that no login response body reaching the browser contains it).

---

## FE-M2 — App shell, i18n, design tokens, RBAC gating

**Scope:** Sidebar/drawer/topbar shell; language selector wired to `/api/v1/meta/locales` + `NEXT_LOCALE` cookie; i18n catalog scaffolding for the namespaces listed in [FRONTEND_ARCHITECTURE.md §6](./FRONTEND_ARCHITECTURE.md#6-i18n); design tokens (colors incl. status palette, typography, spacing) as Tailwind theme extensions; `StatusBadge`, `ConfirmDialog` (reason-required variant), `MoneyValue`, `DateValue`, `StaleBadge`, `EmptyState`, `ErrorState`, `PermissionDeniedState`, `SkeletonRows` shared primitives; RBAC capability matrix implemented as a typed lookup consumed by nav rendering.

**Depends on:** FE-M1 (needs authenticated user/roles in context).

**Test gate:**

- Unit: RBAC lookup returns correct capability booleans for each of ADMIN/STAFF/VIEWER against every row in [FRONTEND_ARCHITECTURE.md §5](./FRONTEND_ARCHITECTURE.md#5-rbac-capability-matrix-from-requirerolescall-sites-only--see-gap-05).
- Component: `StatusBadge` snapshot/DOM assertions confirm icon **and** text are present for every enum value in [API_CONTRACT_MATRIX.md §9](./API_CONTRACT_MATRIX.md#9-canonical-enums-from-migrations-ground-truth-for-statusbadgesfilters) — a table-driven test iterating the full enum list so a future enum addition fails the test until mapped.
- Accessibility: automated axe-core pass (zero critical/serious violations) on the empty shell in both `th` and `en`, both themes if a dark mode is implemented, and on the mobile drawer open state.
- Manual/E2E: keyboard-only navigation reaches every nav item and the language selector; drawer traps focus and closes on `Esc`.

---

## FE-M3 — Domains list + Domain Detail (read paths)

**Scope:** `/domains` (server-side filter/sort/paginate table + mobile card list), `/domains/[domainId]` overview tab, RDAP tab, Provenance tab (all read-only). TanStack Table + TanStack Query wiring per the query-key convention in [FRONTEND_ARCHITECTURE.md §4](./FRONTEND_ARCHITECTURE.md#4-data-fetching-strategy-cache-keys-polling-limits).

**Depends on:** FE-M2.

**Test gate:**

- Unit: query-param ↔ URL search-param round-trip for filters/sort/page.
- Component: loading/empty/error/permission-denied states each render distinctly (per [FRONTEND_ARCHITECTURE.md §8](./FRONTEND_ARCHITECTURE.md#8-error-model-client-side)) using mocked BFF responses for each case.
- E2E (live backend): create a domain via the seeded admin (or a fixture), confirm it appears in the list, filter by `lifecycle_status`, sort by `expiration`, open detail, confirm RDAP tab shows `RDAP_RESULT_NOT_FOUND` as an explicit empty state (not a blank panel) when no RDAP check has run yet.
- Responsive: Playwright viewport tests at `sm`/`md`/`lg` confirm no horizontal page scroll and correct table↔card switch.

---

## FE-M4 — Domain Detail mutations + Monitoring tab

**Scope:** Patch domain, archive/restore (confirm dialogs with mandatory reason), manual check trigger + polling per [FRONTEND_ARCHITECTURE.md §4](./FRONTEND_ARCHITECTURE.md#4-data-fetching-strategy-cache-keys-polling-limits), monitoring-runs table, monitoring-history chart (24h/7d/30d/90d window selector) using Recharts against the real `HistoryAggregate` shape only.

**Depends on:** FE-M3.

**Test gate:**

- Unit: idempotency-key generation is fresh per manual-check click (never reused across two distinct clicks).
- Component: chart renders `UNKNOWN`/absent `uptime_percentage`/`average_response_ms` as an explicit "insufficient data" state, never as `0%`/`0ms` (direct test of the Master Prompt's "no fake zero" rule against `HistoryAggregate`'s nullable fields).
- E2E: trigger a manual check against a live backend, observe the polling UI transition through `queued→running→completed` (or timeout to the manual-refresh fallback at the 60s cap), confirm `ETag`/`version` conflict path (edit the same domain in two tabs, second save shows `VERSION_CONFLICT` UI) — this also exercises the still-unverified reason-enforcement path from [GAP-04](./API_GAPS.md#gap-04--reason-non-empty-enforcement-is-inconsistent-across-mutating-endpoints); record the observed behavior in `API_GAPS.md`.

---

## FE-M5 — Finance, Overrides, Recommendation tabs

**Scope:** Finance page (summary + windows), Domain Detail Finance/Overrides/Recommendation tabs, add-cost form, create/revoke-override flows (ADMIN-only gating), recommendation generate/recompute action, `manual_override` banner rendering when `effective_action !== action`.

**Depends on:** FE-M3, FE-M2 (RBAC).

**Test gate:**

- Unit: `MoneyValue` renders decimal strings byte-for-byte without float coercion — property-based test feeding strings like `"0.10"`, `"1234567.999999"` and asserting no precision loss/reformatting artifacts.
- Component: STAFF-role session sees override create/revoke controls **absent**, not disabled-with-tooltip, matching the binary RBAC gate from [GAP-05](./API_GAPS.md#gap-05--architecturemds-rbac-matrix-171-does-not-match-the-roles-actually-enforced-in-code).
- E2E: add a cost, confirm it appears with correct tax-mode label; create an override, confirm the effective recommendation banner reflects it; revoke the override, confirm the banner reverts.

---

## FE-M6 — Google Sheet Sync / Excel Import / Google Drive Connect

**Scope:** Sync shell (tabs or separate routes per Phase 2 UX decision), Sheet config form, Sheet/Excel preview + staged-rows review table (ADD/MODIFY/UNCHANGED/MISSING/INVALID), apply/reject actions (ADMIN-only), Excel file upload with client-side size pre-check mirroring `EXCEL_IMPORT_MAX_BYTES`, Drive connect/disconnect/file-list per the redirect flow in [FRONTEND_ARCHITECTURE.md §1.6](./FRONTEND_ARCHITECTURE.md#16-google-drive-oauth-not-a-login-flow--see-api_gapsmd-gap-01-gap-07).

**Depends on:** FE-M2.

**Test gate:**

- Unit: Idempotency-Key freshness enforced per distinct preview submission (per [GAP-08](./API_GAPS.md#gap-08--idempotency-key-does-not-hash-the-request-payload-reusing-a-key-with-a-different-payload-silently-returns-the-first-result)) — a test asserting two different form payloads never share a key.
- Component: staged-rows table renders `INVALID` rows with their `validation_errors` list visibly, never hidden/collapsed by default.
- E2E: full Drive OAuth redirect round-trip against a live backend with `GOOGLE_OAUTH_*` configured (if available in the test environment — otherwise this specific E2E is marked skipped/manual with a note, since it requires real Google credentials); Sheet preview → apply happy path with a fixture spreadsheet; Excel upload happy path and an oversized-file rejection path.

---

## FE-M7 — Incidents, Recommendations (list), Reports, Probe Nodes

**Scope:** Incidents list (status filter, pagination — no detail per [GAP-06](./API_GAPS.md#gap-06--architecturemd-documents-incident-detailacknowledge-and-several-settingsauditusers-routes-that-do-not-exist-anywhere-in-the-code)), Recommendations list + bulk "run" action, Reports (summary, create with format/currency, list, download with SHA-256 display), Probe Nodes (list, registration-token create with one-time secret reveal, revoke).

**Depends on:** FE-M2, FE-M5 (shares recommendation types).

**Test gate:**

- Component: probe registration-token reveal renders the token exactly once with a copy-to-clipboard affordance and a persistent warning that it will not be shown again; navigating away and back never re-displays it from client state.
- Component: report download surfaces the `X-Content-SHA256` response header value in the UI (integrity display per Master Prompt data-correctness expectations for generated files).
- E2E: create a report (json then csv, distinct idempotency keys), download both, confirm content-type/filename; run bulk recommendations, confirm list updates; revoke a probe, confirm status badge updates to `REVOKED`.

---

## FE-M8 — Settings / System status, Dashboard

**Scope:** Dashboard (aggregates from `reports/summary` + `finance/summary` only — no client-side raw-data aggregation), Settings/System status scoped to real endpoints only (`/health`, `/ready`, locales, Drive connection status) per [GAP-06](./API_GAPS.md#gap-06--architecturemd-documents-incident-detailacknowledge-and-several-settingsauditusers-routes-that-do-not-exist-anywhere-in-the-code) — explicitly **not** building settings/audit-log/user-management screens against nonexistent endpoints.

**Depends on:** FE-M3–FE-M7 (dashboard links into most other pages).

**Test gate:**

- Component: dashboard renders every summary metric with its real field name/value, no placeholder/mock numbers ever committed to the component tree (grep-able absence of hardcoded numeric literals in dashboard components except test fixtures).
- E2E: `/ready` degraded-dependency scenario (simulate via stopping the Postgres or Redis container in the dev compose stack) renders the system readiness indicator in the amber/rose state, not silently green.

---

## FE-M9 — Cross-cutting hardening pass

**Scope:** Full WCAG 2.2 AA audit (axe-core + manual keyboard pass) across all pages; full bilingual QA pass (every string in both `th`/`en`, no missing-key fallbacks visible); security review pass re-confirming no token/secret ever reaches `localStorage`/`sessionStorage`/JS-readable cookies/client bundle; performance pass on the Domains table with a realistic row count; final reconciliation of [API_GAPS.md](./API_GAPS.md) against live-backend behavior observed across all prior milestones (close out GAP-04, GAP-08, GAP-10's open verification items with recorded findings).

**Depends on:** FE-M0–FE-M8 complete.

**Test gate:** Full `lint`, `typecheck`, unit, component, and E2E suites green in one CI-equivalent run; axe-core zero critical/serious across all routes in both locales; manual sign-off checklist (attached to the PR/report, not a new file) covering every Master Prompt required-states list item (loading/skeleton/empty/error/permission-denied/stale/retry) present on every page that fetches data.

---

## Sequencing notes

- Milestones FE-M3 through FE-M8 can reorder within reason (e.g. Probes before Finance) but **FE-M1 and FE-M2 are hard prerequisites for everything else** — no page-level work starts before auth/session/BFF and the shared primitives/RBAC gating exist, to avoid rebuilding ad hoc auth or status-rendering logic per page.
- Every milestone's E2E gate that requires "a live backend" is blocked until [GAP-10](./API_GAPS.md#gap-10--backend-was-not-running-during-this-audit-nothing-here-is-live-verified) is resolved (i.e., until the compose stack is actually started and spot-checked). Component/unit tests using mocked BFF responses are not blocked and should be the majority of each milestone's coverage so implementation isn't stalled on environment availability.
- No milestone description authorizes editing `internal/`, `migrations/`, `api/openapi.yaml`, root `docker-compose.yml`, or root `README.md` — those remain out of scope per the Master Prompt unless a future phase explicitly instructs otherwise.
