# Frontend Architecture — Phase Frontend 1

This document is the architectural plan for the Next.js admin dashboard in `web/`. No application code exists yet — this is design only, per Phase Frontend 1 acceptance criteria. All API facts referenced here are sourced from [API_CONTRACT_MATRIX.md](./API_CONTRACT_MATRIX.md); all known gaps are in [API_GAPS.md](./API_GAPS.md).

---

## 1. BFF / Session / CSRF architecture

### 1.1 Why a BFF

The Go API has no CORS configuration for browser-origin requests (no `Access-Control-Allow-Origin` handling was found in [internal/api/middleware.go](../internal/api/middleware.go) or [server.go](../internal/api/server.go)), and per the Master Prompt, session-sensitive calls must never expose the raw session cookie or CSRF token to client JavaScript. All authenticated traffic is proxied through **Next.js Route Handlers** running server-side (`web/app/api/**/route.ts`), which:

1. Hold the Go API base URL (`INTERNAL_API_BASE_URL`, server-env only, never `NEXT_PUBLIC_*`).
2. Forward the `domainintel_session` cookie from the Go API to the browser as the **BFF's own** `HttpOnly`, `Secure` (prod), `SameSite=Lax` cookie — same name is fine since it never leaves the BFF's own origin boundary.
3. Store the `csrf_token` returned by `POST /api/v1/auth/login` in a **second** `HttpOnly` cookie (e.g. `bff_csrf`) — never in a JS-readable cookie, never in `localStorage`. Client code cannot read it directly.
4. On every mutating request (`POST/PATCH/PUT/DELETE`) from the browser to a BFF route, the BFF route handler reads `bff_csrf` server-side and attaches it as `X-CSRF-Token` to the outbound Go API call. The browser never sees the raw CSRF value — it only knows "call this same-origin `/api/bff/...` route" and the BFF does the header injection.
5. Forward `Accept-Language` on every proxied call, derived from the resolved locale (§6).

This satisfies the Master Prompt's BFF/CSRF rule exactly: the backend's CORS-less posture becomes irrelevant because the browser only ever talks to same-origin Next.js routes.

### 1.2 Login flow

```
Browser --POST--> /api/bff/auth/login {email,password}
  BFF route --POST--> GO_API/api/v1/auth/login
    <-- 200 {user,csrf_token,expires_at} + Set-Cookie: domainintel_session=...
  BFF sets:
    - Set-Cookie: domainintel_session=<forwarded>; HttpOnly; Secure; SameSite=Lax
    - Set-Cookie: bff_csrf=<csrf_token>; HttpOnly; Secure; SameSite=Lax
  <-- 200 {user, expires_at}   (csrf_token is NOT echoed to the browser body)
Browser stores nothing sensitive; redirects to /dashboard
```

On `401 INVALID_CREDENTIALS`, the BFF passes through the error envelope unchanged (already locale-selected server-side via `Accept-Language`) so the login form can render `error.message` + `error.request_id` directly.

### 1.3 Reload / session restore

On every top-level app shell render (root layout, server component), call a BFF route `GET /api/bff/auth/me` which forwards to `GET /api/v1/auth/me` using the incoming request's cookie. Three outcomes:

- **200** → hydrate `{id,email,display_name,locale,roles}` into a server-rendered auth context; render the authenticated shell.
- **401 UNAUTHORIZED** → redirect to `/login` (session cookie missing, expired, or revoked — the Go API does not distinguish these cases, see [middleware.go:129-149](../internal/api/middleware.go#L129-L149), it always returns the same `UNAUTHORIZED`). The BFF also clears its own cookies on this response to avoid a stale-cookie loop.
- **Network/5xx** → render a full-page "system unavailable" state (distinct from 401) with retry, matching the "stale/unknown/missing must be shown honestly" rule — never silently treat a network failure as logged-out.

### 1.4 Logout

`POST /api/bff/auth/logout` → BFF attaches `X-CSRF-Token` from `bff_csrf` → `POST /api/v1/auth/logout`. Regardless of the Go API's response, the BFF clears both its cookies (`domainintel_session`, `bff_csrf`) and the client redirects to `/login`. A failed upstream logout (e.g. network error) still logs the user out locally — never leave the browser holding cookies for a session the user believes is closed.

### 1.5 401 / 403 / expired-session handling for client-side (TanStack Query) calls

All client-triggered mutations/queries go through the BFF, never directly to the Go API. A shared TanStack Query wrapper:

- On any BFF response with `error.code === "UNAUTHORIZED"` → clear client auth cache, redirect to `/login?reason=expired`, show a toast on the login page ("session expired, please sign in again" — localized).
- On `error.code === "FORBIDDEN"` → do **not** redirect; render an in-place "Permission denied" state on the affected panel/page (per Master Prompt required states list) since the user is still validly authenticated, just not authorized for this specific action. This can legitimately happen even when the UI hid the control, because **the backend is the enforcement authority** (Master Prompt RBAC rule) — e.g., a STAFF session whose role was revoked mid-session.
- On `error.code === "CSRF_INVALID"` → treat as a BFF-internal bug (should never surface to the user since the BFF injects the token) — log client-side, show a generic retry-safe error, and have the BFF re-fetch a fresh session server-side on next load.
- On `error.code === "VERSION_CONFLICT"` → surface a specific "record changed, reload" affordance (Domain/Sheet-config patch flows use optimistic concurrency via `version`/`ETag`).

### 1.6 Google Drive OAuth (not a login flow — see [API_GAPS.md GAP-01, GAP-07](./API_GAPS.md#gap-01--no-oidcoauth-login-exists-master-prompts-oauth-popup-flow-requirement-has-no-backend-counterpart-for-user-authentication))

Implemented as a **top-level redirect through the BFF**, not a cross-origin popup, because `GET /api/v1/google-drive/callback` requires the admin's session cookie to be present (GAP-07):

```
Browser --POST--> /api/bff/google-drive/connect (BFF injects CSRF)
  --> GO_API returns {authorization_url, expires_at}
BFF returns {authorization_url} to browser
Browser does window.location.assign(authorization_url)  [full navigation, same tab or a same-origin-opened tab]
Google redirects browser --> /api/bff/google-drive/callback?state&code
  BFF forwards state/code (with session cookie) --> GO_API GET /api/v1/google-drive/callback
  BFF redirects browser --> /sync/google-drive?connected=true|false
```

A popup is only revisited if product explicitly wants it; it would need the popup opened from the BFF's own origin (not directly to `accounts.google.com` from a cross-origin JS call) and `postMessage` back to the opener on completion. Default to the redirect approach for Phase 2 since it needs no extra cross-window messaging code and matches how the callback endpoint actually authenticates.

---

## 2. Route map (Next.js App Router)

| Route                              | Page (Master Prompt list)      | Primary data                                                                                                                                      | Roles that can view                                                     |
| ---------------------------------- | ------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------- |
| `/login`                           | Login                          | `POST auth/login`                                                                                                                                 | public                                                                  |
| `/` → redirect `/dashboard`        | —                              | —                                                                                                                                                 | authenticated                                                           |
| `/dashboard`                       | Dashboard                      | `reports/summary`, `finance/summary`                                                                                                              | ADMIN,STAFF,VIEWER                                                      |
| `/domains`                         | Domains                        | `domains` (list, server-side filter/sort/paginate)                                                                                                | ADMIN,STAFF,VIEWER                                                      |
| `/domains/[domainId]`              | Domain Detail                  | `domains/{id}`, `.../monitoring-runs`, `.../monitoring-history`, `.../rdap`, `.../costs`, `.../overrides`, `.../provenance`, `.../recommendation` | ADMIN,STAFF,VIEWER (mutating actions gated per §5)                      |
| `/domains/[domainId]/runs/[runId]` | Domain Detail (run drill-down) | `monitoring-runs/{runId}`                                                                                                                         | ADMIN,STAFF,VIEWER                                                      |
| `/incidents`                       | Incidents                      | `incidents` (list + status filter only — no detail route exists, [GAP-06](./API_GAPS.md#gap-06))                                                  | ADMIN,STAFF,VIEWER                                                      |
| `/finance`                         | Finance                        | `finance/summary`, per-domain `costs`/`overrides` when drilling in                                                                                | ADMIN,STAFF,VIEWER (mutations ADMIN/STAFF or ADMIN-only per route)      |
| `/recommendations`                 | Recommendations                | `recommendations` (list), `recommendations/run`                                                                                                   | ADMIN,STAFF,VIEWER                                                      |
| `/reports`                         | Reports                        | `reports/summary`, `reports`, `reports/{id}`, `reports/{id}/download`                                                                             | ADMIN,STAFF,VIEWER                                                      |
| `/sync/google-sheets`              | Google Sheet Sync              | `google-sheets/config`, `google-sheets/previews`, `google-sheets/imports`                                                                         | ADMIN,STAFF,VIEWER (apply/reject ADMIN-only)                            |
| `/sync/excel-import`               | Excel Import                   | `google-sheets/excel/previews`, `google-sheets/imports/{id}`                                                                                      | ADMIN,STAFF apply ADMIN-only                                            |
| `/sync/google-drive`               | Google Drive Connect           | `google-drive/connect`, `.../connection`, `.../files`                                                                                             | ADMIN connect/disconnect; all roles view status                         |
| `/probes`                          | Probe Nodes                    | `probes`, `probes/registration-tokens`, `probes/{id}/revoke`                                                                                      | ADMIN,STAFF,VIEWER view; ADMIN mutate                                   |
| `/settings`                        | Settings / System status       | `/health`, `/ready`, `/api/v1/meta/locales`, Drive connection status                                                                              | ADMIN,STAFF,VIEWER (scope reduced — see [GAP-06](./API_GAPS.md#gap-06)) |

`/sync/google-sheets`, `/sync/excel-import`, `/sync/google-drive` may be tabs of one `/sync` shell page rather than three separate routes — a Phase 2 UX decision, not an API constraint.

---

## 3. Component hierarchy (high level)

```
app/layout.tsx                         (root: locale cookie read, font, theme reset)
  app/(authenticated)/layout.tsx       (server component: auth/me fetch, redirect if 401)
    AppShell
      Sidebar (desktop) / Drawer (mobile)     — nav items filtered by RBAC capability matrix (§5)
      TopBar
        Breadcrumb
        LanguageSelector (th/en)
        SystemReadinessIndicator          — polls BFF-proxied /ready, see §4 polling limits
        UserMenu (display_name, roles, logout)
      PageOutlet (route content)

Per-page composition pattern (example: Domains):
  DomainsPage (server component: initial page 1 fetch for fast paint)
    DomainsFilterBar (client: query/lifecycle/source_status/sort — writes to URL search params)
    DomainsTable (client: TanStack Table + TanStack Query, server-side pagination)
      DomainsTableDesktop (dense table, ≥ md breakpoint)
      DomainsCardList (mobile card adaptation, < md breakpoint)
    EmptyState / ErrorState / PermissionDeniedState / SkeletonRows (shared primitives, reused across all list pages)

Domain Detail:
  DomainDetailPage (server: initial domain fetch)
    DomainHeader (status badges, lifecycle actions: archive/restore/check — each behind ConfirmDialog with mandatory reason)
    Tabs: Overview | Monitoring | RDAP | Finance | Overrides | Provenance | Recommendation
      MonitoringTab (client: monitoring-runs table + monitoring-history chart, window selector 24h/7d/30d/90d)
      FinanceTab (client: costs table, add-cost form, override list)
      RecommendationTab (client: latest recommendation + manual_override banner if present)

Shared primitives (used everywhere): StatusBadge (icon+text+color per §7), ConfirmDialog (reason-required variant), MoneyValue (renders decimal string verbatim, currency-aware), DateValue (explicit TZ), StaleBadge, RequestIdDetail (error support affordance).
```

---

## 4. Data-fetching strategy, cache keys, polling limits

- **Server Components** fetch the first page of any list and the primary detail object at request time (no client waterfall for initial paint), using the BFF's internal fetch (still same-origin, still cookie-forwarded).
- **Client Components** use **TanStack Query** for anything requiring refetch, polling, optimistic-concurrency retry, or user-driven filter/sort/page changes.
- **Query key convention:** `[resource, ...scopingIds, paramsObject]`, e.g. `["domains", {query,lifecycle_status,source_status,page,page_size,sort,direction}]`, `["domain", domainId]`, `["domain-monitoring-history", domainId, window]`, `["incidents", {status,page,page_size}]`, `["finance-summary", reportingCurrency]`. Mutations invalidate the exact scoped keys they affect (e.g. `archiveDomain` invalidates `["domain", domainId]` and every `["domains", ...]` list key via a shared prefix invalidation).
- **Polling (only where the platform has no push mechanism — it doesn't, there's no WebSocket/SSE anywhere in the Go API):**
  - `SystemReadinessIndicator` (`/ready` proxy): every 30s, paused when the browser tab is hidden (`document.visibilityState`).
  - Domain Detail "Monitoring" tab, only while a manual check the user just triggered is `queued`/`running`: poll `GET monitoring-runs/{runID}` (via the returned `Location` header from `POST .../check`) every 3s, capped at 20 attempts (~60s) before falling back to a manual "Refresh" button — the Go monitor run timeout defaults to 60s (`MONITOR_RUN_TIMEOUT`, [config.go:287](../internal/config/config.go#L287)), so this cap is aligned to backend reality, not arbitrary.
  - Sheet/Excel import preview while `status=preview` transitioning to `applying`: poll `GET google-sheets/imports/{id}` every 3s, capped at 20 attempts.
  - No other page polls by default — everything else is fetch-on-navigation or explicit user-triggered refetch, per the Master Prompt's "no fake history/metrics" and "use real aggregate endpoints" rules; there is no justification for polling data that doesn't change server-side without a user action.
- **Never** fetch all `monitoring_runs`/`monitoring_results` client-side to compute uptime/aggregates in the browser — always use `GET .../monitoring-history?window=` which returns the pre-aggregated `HistoryAggregate` from Postgres ([monitor/model.go:209-227](../internal/monitor/model.go#L209-L227)). This is a hard rule from the Master Prompt and is directly enforceable because the aggregate endpoint already exists.

---

## 5. RBAC capability matrix (from `requireRoles(...)` call sites only — see [GAP-05](./API_GAPS.md#gap-05--architecturemds-rbac-matrix-171-does-not-match-the-roles-actually-enforced-in-code))

| Capability                                                                         | ADMIN | STAFF | VIEWER |
| ---------------------------------------------------------------------------------- | :---: | :---: | :----: |
| View domains/detail/monitoring/RDAP/costs/overrides/provenance/recommendation      |   ✓   |   ✓   |   ✓    |
| Create/patch/archive/restore domain, trigger manual check, RDAP re-check, add cost |   ✓   |   ✓   |   –    |
| Create/revoke manual override                                                      |   ✓   |   –   |   –    |
| Add exchange rate                                                                  |   ✓   |   –   |   –    |
| Preview Sheet/Excel import                                                         |   ✓   |   ✓   |   –    |
| Apply/reject Sheet/Excel import                                                    |   ✓   |   –   |   –    |
| Save Sheet config                                                                  |   ✓   |   –   |   –    |
| Connect/disconnect Google Drive                                                    |   ✓   |   –   |   –    |
| View Drive connection/files                                                        |   ✓   |   ✓   |   ✓    |
| Generate/recompute recommendation (single or bulk run)                             |   ✓   |   ✓   |   –    |
| View recommendations                                                               |   ✓   |   ✓   |   ✓    |
| Create report                                                                      |   ✓   |   ✓   |   –    |
| View/download report                                                               |   ✓   |   ✓   |   ✓    |
| View probes                                                                        |   ✓   |   ✓   |   ✓    |
| Create probe registration token, revoke probe                                      |   ✓   |   –   |   –    |
| View incidents                                                                     |   ✓   |   ✓   |   ✓    |

This table drives both (a) which sidebar/nav items and page-level action buttons render per role, and (b) which BFF routes the frontend even attempts to call for a given session — but the Go API is always the final authority (Master Prompt rule), so every mutating BFF call must still handle a `403 FORBIDDEN` gracefully (§1.5) rather than assume the client-side gate was sufficient.

---

## 6. i18n

- Default locale **Thai (`th`)**, switchable to English, persisted in a **non-sensitive** cookie (e.g. `NEXT_LOCALE`, not `HttpOnly`-required since it carries no secret) readable by both server and client for SSR-correct initial render.
- The BFF forwards `Accept-Language: <resolved-locale>` on every proxied call so the Go API's own `i18n.Parse`/`localeMiddleware` selects matching `messages.th`/`messages.en` server-side ([middleware.go:34-42](../internal/api/middleware.go#L34-L42)) — **error copy is never duplicated client-side**, always rendered from `error.message`.
- UI chrome strings (labels, buttons, table headers, empty-state copy) live in the frontend's own translation catalog, namespaced by feature area to keep bundles reviewable and translators scoped:
  `common` (nav, buttons, table chrome, status labels), `auth`, `dashboard`, `domains`, `domainDetail`, `incidents`, `finance`, `recommendations`, `reports`, `sync` (sheets/excel/drive), `probes`, `settings`, `errors` (fallback copy for network/unexpected failures the server never sends a code for).
- Numbers/decimals/dates are **never** run through `Intl.NumberFormat` in a way that parses-then-reformats a decimal string as a JS number for money — money strings are displayed as-is (grouped with a display-only thousands separator applied via string manipulation, not `parseFloat`), per the Master Prompt's decimal-safety rule. Dates use an explicit UTC→display-timezone conversion library (e.g. a `Temporal`-polyfill or `date-fns-tz`) with the timezone always shown alongside the value (e.g. "2026-08-30 14:00 ICT").

---

## 7. Design tokens & status mapping

| Status family         | Color token        | Values mapped                                                                                                                                                                     | Icon requirement                       |
| --------------------- | ------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------- |
| Healthy/online        | `--status-emerald` | `availability_status=ACTIVE`, `probe_status=ONLINE`, `dns/http/tls=OK/VALID`, `recommendation=RENEW`                                                                              | check-circle                           |
| Degraded/review       | `--status-amber`   | `availability_status=DEGRADED`, `probe_status=DEGRADED`, `tls_status=EXPIRING`, `recommendation=REVIEW`, `incident_status=acknowledged`, `redirect_status=TEMPORARY`              | alert-triangle                         |
| Offline/error         | `--status-rose`    | `availability_status=UNAVAILABLE`, `probe_status=OFFLINE/REVOKED`, any `*_ERROR`/`CLIENT_ERROR`/`SERVER_ERROR`/`EXPIRED`/`INVALID`, `recommendation=DROP`, `incident_status=open` | x-circle                               |
| Unknown/stale/missing | `--status-slate`   | any `UNKNOWN` enum value, `source_status=missing_from_source`, no `last_checked_at`                                                                                               | help-circle (never a blank/empty cell) |
| ISP risk              | `--status-violet`  | `isp_status=SUSPECTED`, `isp_status=HIGH_CONFIDENCE_BLOCK`                                                                                                                        | shield-alert                           |

Every status render is `<Icon/> <ColorDot/> <TextLabel/>` — never color alone (WCAG 2.2 AA + Master Prompt rule). `UNKNOWN`/`missing_from_source`/absent `last_checked_at` must render as a visibly distinct slate "Unknown" badge, never coerced to look like a passing/zero state — this is directly testable against the enum list in [API_CONTRACT_MATRIX.md §9](./API_CONTRACT_MATRIX.md#9-canonical-enums-from-migrations-ground-truth-for-statusbadgesfilters).

Typography: a Thai-optimized variable font stack (e.g. `Noto Sans Thai` paired with a Latin sans, loaded via `next/font` with `display: swap`) — exact family selection deferred to Phase 2 implementation after checking license/self-hosting requirements (no external font CDN per artifact/security norms carried into this project's own asset policy).

---

## 8. Error model (client-side)

Single shared `ApiError` shape mirrored from the Go envelope ([response.go:12-23](../internal/api/response.go#L12-L23)):

```ts
type ApiError = {
  code: string;
  message: string; // already locale-selected server-side
  requestId: string;
  details?: Record<string, unknown>;
};
```

Every BFF route re-throws this shape unchanged on non-2xx (never re-wraps or re-translates it). Every data-fetching component distinguishes, per the Master Prompt's required-states list:

`loading` (skeleton) → `empty` (zero results, distinct copy from error) → `error` (network/5xx, retry affordance) → `permission-denied` (403, distinct from generic error) → `stale` (data present but past a freshness threshold, e.g. `last_checked_at` older than the domain's monitoring interval — shown as a badge, not a blocking state) → `success`.

No page is allowed to collapse `empty` and `error` into the same UI, and no page may render partial/stale data without the `stale` indicator visible.

---

## 9. Responsive behavior

- Breakpoints follow Tailwind defaults (`sm 640 / md 768 / lg 1024 / xl 1280`); dense data tables switch to card/list layout below `md`, per Master Prompt.
- Sidebar is persistent ≥ `lg`, becomes a drawer (hamburger-triggered, focus-trapped, `Esc`-closeable) below `lg`.
- No component may cause horizontal page scroll; wide tables/JSON evidence viewers/redirect-chain diagrams scroll within their own `overflow-x-auto` container only.
- Confirm dialogs (archive/restore/override/revoke/reject) are full-screen sheets on mobile, centered modals ≥ `md`.
