# API Gaps — Phase Frontend 1

Every gap below is a conflict between two or more of: running code ([internal/api](../internal/api)), [api/openapi.yaml](../api/openapi.yaml), [ARCHITECTURE.md](../ARCHITECTURE.md), and the Master Prompt / CLAUDE_FRONTEND_PROMPTS.md assumptions. Per the source-of-truth order in the Master Prompt, **code wins** in every case below; nothing in this file changes backend behavior — it only documents what the frontend must adapt to and what a backend owner should reconcile.

---

### GAP-01 — No OIDC/OAuth login exists; Master Prompt's "OAuth popup flow" requirement has no backend counterpart for user authentication

- **Evidence:** [internal/auth/service.go](../internal/auth/service.go) implements only `Login(email, password)` against a local `users` table with Argon2id ([internal/auth/password.go](../internal/auth/password.go)). `POST /api/v1/auth/login` request body is `{email,password}` only ([server.go:231-234](../internal/api/server.go#L231-L234); [openapi.yaml:15-30](../api/openapi.yaml#L15-L30)).
- **ARCHITECTURE.md confirms this is intentional-for-now:** §17.2 states "Prefer OIDC Authorization Code + PKCE with corporate IdP" as the **target**, with "Bootstrap local admin disabled after OIDC setup" ([ARCHITECTURE.md:1450-1453](../ARCHITECTURE.md#L1450-L1453)); §2.2 assumption 7 says "Corporate OIDC เป็น authentication เป้าหมาย; local bootstrap admin ใช้เฉพาะ deployment ที่ยังไม่มี IdP" ([ARCHITECTURE.md:90](../ARCHITECTURE.md#L90)).
- **Impact:** The Master Prompt's Phase 1 task list asks the frontend architecture to plan "OAuth popup flow" as part of login. There is no such flow for user login — only for the separate, admin-only **Google Drive connection** feature (`POST /api/v1/google-drive/connect` → `GET /api/v1/google-drive/callback`, session-cookie-authenticated, see contract matrix §6).
- **Resolution for Phase 2+:** Build Login as a plain email/password form posting through the BFF. Build the Drive "Connect" button/flow separately in the Google Sheet Sync page using the real Drive OAuth endpoints. Do not build a generic OAuth login screen.

### GAP-02 — `internal/api/server.go` has multiple routes that do not exist in `api/openapi.yaml`

- **Evidence — routes present in code, absent from spec:**
  - `POST /api/v1/domains/{domainID}/check` (manual check trigger) — [server.go:114](../internal/api/server.go#L114)
  - `GET /api/v1/domains/{domainID}/monitoring-runs` — [server.go:115](../internal/api/server.go#L115)
  - `GET /api/v1/domains/{domainID}/monitoring-history` — [server.go:116](../internal/api/server.go#L116)
  - `GET /api/v1/monitoring-runs/{runID}` — [server.go:129](../internal/api/server.go#L129)
  - `GET /api/v1/incidents` — [server.go:130](../internal/api/server.go#L130)
  - `GET /api/v1/probes`, `POST /api/v1/probes/registration-tokens`, `POST /api/v1/probes/{probeID}/revoke` — [server.go:152-154](../internal/api/server.go#L152-L154)
- **Impact:** No OpenAPI-generated client can cover these; the Master Prompt explicitly calls this scenario out ("ระบุ route ที่ server.go มีแต่ OpenAPI ไม่มี เช่น monitoring check/history/runs, incidents และ probes"). These routes are exactly the ones needed for Domain Detail's monitoring tab, the Incidents page, and the Probe Nodes page.
- **Resolution for Phase 2+:** Hand-write TypeScript types for these six routes from the Go structs cited in [API_CONTRACT_MATRIX.md](./API_CONTRACT_MATRIX.md) §3–4, §8 rather than relying on generated-from-OpenAPI types. Flag this to the backend owner as an OpenAPI drift to fix independently of frontend work — **frontend must not edit `api/openapi.yaml`** per Master Prompt (backend is out of scope).

### GAP-03 — Duplicate/aliased recommendation routes

- **Evidence:** `GET /api/v1/domains/{domainID}/recommendation` and `GET /api/v1/domains/{domainID}/recommendations` both dispatch to `s.getDomainRecommendation` ([server.go:125,127](../internal/api/server.go#L125-L127)); `POST .../recommendation` and `POST .../recommendations/recompute` both dispatch to `s.generateDomainRecommendation` ([server.go:126,128](../internal/api/server.go#L126-L128)). Only the singular forms are in `openapi.yaml` ([openapi.yaml:372-386](../api/openapi.yaml#L372-L386)).
- **Impact:** Cosmetic, not a blocker — but a generated client would only know about one path per verb.
- **Resolution:** Frontend should call only the singular `/recommendation` GET and POST forms (they are the ones documented in OpenAPI and therefore the more likely long-term-stable path).

### GAP-04 — `reason` non-empty enforcement is inconsistent across mutating endpoints

- **Evidence:** `finance.RevokeOverride` explicitly rejects an empty reason (`fmt.Errorf("%w: reason", ErrValidation)`, [finance/service.go:280-282](../internal/finance/service.go#L280-L282)); `drive.Disconnect` does the same ([drive/service.go:240-242](../internal/drive/service.go#L240-L242)). The domain `archive`/`restore` handler (`s.changeLifecycle`) passes `request.Reason` straight through to `s.domains.Archive`/`Restore` without a visible non-empty check in [server.go:443-476](../internal/api/server.go#L443-L476) or in [domain/model.go](../internal/domain/model.go) (the domain package's own validation was not traced past `PatchInput`/lifecycle change signatures in this pass — `internal/domain/service.go` was not fully read).
- **Impact:** Unknown whether the backend rejects an empty archive/restore reason with `VALIDATION_FAILED` or silently accepts it. The Master Prompt requires a mandatory reason field on archive/restore/override/revoke/reject confirm dialogs regardless of backend enforcement — so this gap does not block the UI requirement, but the **error-handling path is unverified**.
- **Resolution:** Build the confirm dialogs with client-side mandatory reason validation as instructed regardless. During Phase 2/3 QA against a live backend, explicitly test submitting archive/restore with an empty reason and record the actual server response in this file.

### GAP-05 — ARCHITECTURE.md's RBAC matrix (§17.1) does not match the roles actually enforced in code

- **Evidence:** ARCHITECTURE.md §17.1 ([ARCHITECTURE.md:1436-1444](../ARCHITECTURE.md#L1436-L1444)) describes STAFF as "policy-limited" (i.e., partially allowed) for "Domain/price/schedule edit" and for "Import apply/archive/override recommendation." The actual `requireRoles(...)` calls show:
  - Domain override create/revoke: **ADMIN only**, STAFF has zero access ([server.go:122-123](../internal/api/server.go#L122-L123)).
  - Sheet import apply/reject: **ADMIN only**, STAFF has zero access ([server.go:139-140](../internal/api/server.go#L139-L140)).
  - Domain patch/archive/restore/check/cost-add: ADMIN **and** STAFF equally — no policy-based STAFF restriction exists in code, it's a flat allow ([server.go:109,111-114,120](../internal/api/server.go#L109-L120)).
- **Impact:** ARCHITECTURE.md is a pre-implementation design doc; the actual RBAC boundary is coarser (binary role check, not attribute/policy-based). The Master Prompt requires the UI to "respect RBAC" — it must respect the **actual** binary role gates, not the aspirational policy-limited language.
- **Resolution:** Frontend RBAC capability matrix (see [FRONTEND_ARCHITECTURE.md](./FRONTEND_ARCHITECTURE.md) §RBAC) is built strictly from `requireRoles(...)` call sites in `server.go`, listed per-route in [API_CONTRACT_MATRIX.md](./API_CONTRACT_MATRIX.md). UI must hide/disable STAFF from override-create/revoke and sheet-import apply/reject controls entirely, not show a "limited" variant.

### GAP-06 — ARCHITECTURE.md documents Incident detail/acknowledge and several Settings/Audit/Users routes that do not exist anywhere in the code

- **Evidence:** ARCHITECTURE.md §16.3 lists `GET /incidents/{incident_id}` and `POST /incidents/{incident_id}/acknowledge` ([ARCHITECTURE.md:1377-1379](../ARCHITECTURE.md#L1377-L1379)); §16.6 lists `GET/PATCH /settings`, `GET /audit-logs`, `GET /users`, `PATCH /users/{user_id}/roles`, `GET /metrics` ([ARCHITECTURE.md:1417-1428](../ARCHITECTURE.md#L1417-L1428)). None of these routes exist in `internal/api/server.go`'s route table ([server.go:94-156](../internal/api/server.go#L94-L156)), and none are in `openapi.yaml`.
- **Impact:** The Master Prompt requires an **Incidents** page and a **Settings / System status** page. Only `GET /api/v1/incidents` (list, with `status` filter) exists — there is no incident detail endpoint, no acknowledge action, no settings CRUD, no audit-log viewer, and no user/role management endpoint anywhere in the running backend.
- **Resolution:** Incidents page in Phase 2+ must be **list-only** (status filter, pagination) — no detail drill-down, no acknowledge button, since there's nothing to call. Settings / System status page must be scoped down to what real endpoints support: `/health`, `/ready`, `/api/v1/meta/locales`, and read-only Google Sheets/Drive connection status (§6 of contract matrix) — there is no generic settings, audit log, or user-management API. This must be flagged to the user/product owner as a scope reduction versus the architecture doc's aspiration, not silently built against fictitious endpoints.

### GAP-07 — Google Drive OAuth callback requires an authenticated session, which is unusual for an OAuth redirect target

- **Evidence:** `GET /api/v1/google-drive/callback` sits inside the `protected` route group with `requireRoles("ADMIN")` ([server.go:142](../internal/api/server.go#L142)), meaning the browser must already carry a valid `domainintel_session` cookie when Google redirects back to this URL.
- **Impact:** If the OAuth flow is implemented as a **popup**, the popup window must share the same origin/cookie jar as the main app (i.e., same site, not a cross-origin popup to the raw Go API) for the callback to authenticate. If the BFF proxies `/api/v1/google-drive/callback` through a Next.js Route Handler on the app's own origin, this works naturally. A full top-level redirect through the BFF is simpler to get right than a popup here.
- **Resolution:** Implement Drive connect as a BFF-proxied top-level redirect (not a raw popup to the Go backend's own origin) in Phase 2+, so the session cookie is present when the callback route is hit.

### GAP-08 — Idempotency-Key does not hash the request payload; reusing a key with a different payload silently returns the first result

- **Evidence:** `report.Service.Create` looks up an existing record by `(requested_by, idempotency_key)` only, before even validating `format` ([report/service.go:128-138](../internal/report/service.go#L128-L138)) — the stored/returned record reflects whatever `format`/`reporting_currency` was used on the **first** call with that key, not the current call's body.
- **Impact:** Directly relevant to the Master Prompt's explicit rule: "รองรับ Idempotency-Key สำหรับ import preview/apply และ report creation ห้าม reuse key กับ payload คนละชุด." The backend does not defend against this itself for reports — enforcement is a frontend responsibility.
- **Resolution:** Generate a new UUID `Idempotency-Key` client-side per distinct form submission (e.g., on every "Generate report" click with the current field values), never persist/reuse a key across edits to the format or currency selector. Same discipline applies to `previewSheetImport`/`previewExcelImport`/`applySheetImport`, whose idempotency behavior was not traced to the same depth in this pass — treat all four Idempotency-Key endpoints with the same "new key per distinct payload" rule defensively.

### GAP-09 — `docs/phase3-monitoring-orchestration.md` referenced by the Master Prompt does not exist; actual filename is `docs/phase3-monitoring-engine.md`

- **Evidence:** `ls docs/` shows `phase3-monitoring-engine.md`, not `phase3-monitoring-orchestration.md`.
- **Impact:** None beyond documentation hygiene — content was read from the correctly-named file.
- **Resolution:** No action needed; noted for traceability only.

### GAP-10 — Backend was not running during the Phase 1 audit; superseded

- **Status: resolved during Phase Frontend 6–7 QA.** The stack was brought up via `docker compose up -d postgres redis migration api` and seeded via `docker compose --profile tools run --rm seed-admin`, and the full authenticated Playwright suite (login, domains create/detail/manual-check, finance, recommendations, reports, sync/Drive/Excel, probes, settings, responsive) was run against it — 17/17 passing. Two real bugs were found and fixed this way that static reading could not have caught: the BFF login route was silently dropping the backend's session cookie (see `src/app/api/bff/auth/login/route.ts`), and `ExcelTab`'s Zod schema referenced the browser-only `FileList` global at render time, crashing SSR.
- **Resolution:** No further action — kept for traceability. Any future contract drift should still be spot-checked live rather than assumed from source reading alone.

### GAP-11 — `internal/domain.Normalizer` rejects any TLD not in the ICANN section of the public suffix list by default (`ALLOW_UNKNOWN_TLD=false`)

- **Evidence:** [internal/domain/normalize.go:57-63](../internal/domain/normalize.go#L57-L63) — `if !icann && !n.AllowUnknownTLD { return ...DOMAIN_INVALID }`. Both `domain.NewService` (used by `POST /domains`) and the sheets/Excel import path construct their `Normalizer` from the same `cfg.AllowUnknownTLD` ([internal/app/api.go:22](../internal/app/api.go#L22), [internal/app/phase5.go:46](../internal/app/phase5.go#L46)), which defaults to `false` ([internal/config/config.go:230](../internal/config/config.go#L230)).
- **Impact:** The RFC 2606 reserved TLD `.example` — a natural choice for E2E/test fixture domains, and used in an early draft of this frontend's own Playwright specs — is rejected as `DOMAIN_INVALID` / `INVALID_DOMAIN` against a default-configured backend. This is not a frontend bug; it is real backend validation behavior, confirmed by running the Excel import and Add-domain flows live and observing `INVALID_DOMAIN`/`DOMAIN_INVALID` responses for `.example` domains.
- **Resolution:** `web/e2e/*.spec.ts` and `web/e2e/fixtures/import-fixture.xlsx` use a namespaced `.com`-style test domain (e.g. `e2e-test-<timestamp>.com`) instead of `.example`. Anyone adding new E2E fixtures or manually testing domain creation against a default-configured backend should do the same, or set `ALLOW_UNKNOWN_TLD=true` in that backend's environment first.
