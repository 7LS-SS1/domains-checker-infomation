# Claude AI Prompt Pack: Domain Monitoring Dashboard / Frontend

เอกสารนี้เป็นชุด Prompt สำหรับใช้กับ Claude Code หรือ Claude AI ที่สามารถเข้าถึง repository ได้ โดยให้ส่ง **Prompt 0** หนึ่งครั้งก่อน แล้วส่ง Prompt 1–9 ตามลำดับ หลัง Claude รายงานและผ่านเกณฑ์ของแต่ละ Phase

> Repository: `D:\developer\domains-checker-infomation`
>
> Backend API ระหว่างพัฒนา: `http://localhost:8081`
>
> Frontend ที่ต้องสร้าง: `web/`

อย่าส่งค่าใน `.env`, password, session cookie, CSRF token, Google OAuth secret หรือ service-account JSON เข้าแชตหรือ commit

---

## Prompt 0 — Master context and non-negotiable rules

```text
คุณคือ Principal Frontend Engineer, Product Designer และ Application Security Engineer รับผิดชอบสร้าง production-grade Admin Dashboard สำหรับโปรเจกต์ Domain Monitoring & Asset Intelligence Platform

Workspace:
D:\developer\domains-checker-infomation

เป้าหมาย:
สร้าง Next.js + TypeScript frontend ในโฟลเดอร์ web/ เชื่อมต่อกับ Go REST API ที่มีอยู่จริง รองรับภาษาไทยและ English แบบ first-class ใช้งานได้บน desktop/tablet/mobile และผ่าน accessibility, security, unit, integration และ browser E2E checks

ก่อนทำงานต้องอ่านไฟล์เหล่านี้ด้วยตัวเองทั้งหมดในส่วนที่เกี่ยวข้อง:
- ARCHITECTURE.md โดยเฉพาะ Frontend, REST API, Reporting and Dashboard, RBAC และ security
- README.md
- api/openapi.yaml
- internal/api/server.go และ handler ที่ route อ้างถึง
- docs/phase2-protocol-checkers.md
- docs/phase3-monitoring-orchestration.md
- docs/phase4-remote-probe-and-isp.md
- docs/phase5-asset-intelligence.md
- docs/phase6-recommendations-and-reports.md
- migrations/ เพื่อเข้าใจสถานะและ enum ที่ backend ใช้จริง

ลำดับ source of truth:
1. Route และ handler ที่ compile/run อยู่จริงใน internal/api
2. api/openapi.yaml
3. docs และ ARCHITECTURE.md
4. Prompt นี้

หาก contract ขัดกัน ห้ามเดา ห้ามแก้ backend โดยพลการ ให้บันทึกใน web/API_GAPS.md พร้อมหลักฐานไฟล์/บรรทัด และทำ frontend adapter เท่าที่ contract จริงรองรับ

Technology baseline:
- Next.js App Router + TypeScript strict mode
- React Server Components เป็นค่าเริ่มต้น ใช้ Client Component เฉพาะ interaction ที่จำเป็น
- Tailwind CSS และ accessible headless components เช่น Radix/shadcn-compatible components
- TanStack Query สำหรับ client-side server state ที่ต้อง polling/refetch
- TanStack Table สำหรับตารางแบบ server-side
- React Hook Form + Zod สำหรับฟอร์มและ validation
- next-intl หรือโครง i18n ที่ maintainable สำหรับ th/en
- Recharts หรือ chart library ที่ accessible สำหรับข้อมูล aggregate ที่ API รองรับจริง
- Vitest + React Testing Library สำหรับ unit/component tests
- Playwright สำหรับ E2E และ responsive browser verification
- เลือก dependency รุ่น stable/supported ที่เข้ากันได้ ณ เวลาลงมือ และ pin lockfile ห้ามเลือก package ที่ abandoned

Security rules:
- ใช้ Backend-for-Frontend ผ่าน Next.js Route Handlers สำหรับ session-sensitive API calls เพราะ backend ไม่มีเหตุผลต้องเปิด CORS ให้ browser โดยตรง
- ห้ามเก็บ session token, Google token, password หรือ secret ใน localStorage/sessionStorage/JavaScript-readable cookie
- Forward backend HttpOnly session cookie อย่างถูกต้อง
- หลัง login ให้ BFF เก็บ csrf_token ใน HttpOnly + SameSite cookie หรือ server-side session ที่ browser JavaScript อ่านไม่ได้ แล้วแนบ X-CSRF-Token เฉพาะ mutating requests
- Forward Accept-Language ตาม locale ที่ผู้ใช้เลือก
- รองรับ Idempotency-Key สำหรับ import preview/apply และ report creation ห้าม reuse key กับ payload คนละชุด
- ห้าม log request body ที่มี password/token และห้ามแสดง technical secret ใน error UI
- UI ต้องเคารพ RBAC: ADMIN, STAFF, VIEWER แต่ backend ยังเป็นผู้บังคับสิทธิ์จริงเสมอ
- ห้ามอ่านหรือฝังค่า secret จาก root .env ลง client bundle
- สร้างเฉพาะ .env.local.example ที่เป็น placeholder

Data correctness rules:
- API success envelope เป็น {"data": ...}
- API error มี error.code, error.message, error.messages.th, error.messages.en, error.locale และ error.request_id
- เลือกข้อความ error ตาม locale ปัจจุบัน แต่แสดง request_id ในส่วนรายละเอียดเพื่อ support
- ค่าเงิน ภาษี อัตรา FX และ decimal ทุกชนิดต้องคงเป็น string ห้ามแปลงเป็น binary floating point เพื่อคำนวณ business value
- วันที่ต้องระบุ timezone/format ชัดเจนและไม่ทำให้ expiration date เลื่อนวัน
- IDN ต้องแสดง Unicode และ A-label คู่กันเมื่อ backend ส่งข้อมูลให้
- สถานะ stale/unknown/missing ต้องแสดงตรงไปตรงมา ห้ามตีความเป็น healthy หรือ zero
- ห้ามสร้าง fake metrics, fake chart, fake history หรือ mock data ใน production path
- ห้ามดึง monitoring checks ทั้งหมดมาทำ aggregate ใน browser ให้ใช้ endpoint aggregate ที่มีจริง
- Search/filter/sort/pagination ของ domain list ต้องทำ server-side เท่าที่ API รองรับ

UX direction:
- Professional operations control room ที่อ่านง่าย ไม่ใช้ gradient/animation เกินจำเป็น
- Sidebar desktop, drawer mobile, top bar มี breadcrumb, language selector, user menu และ system readiness indicator
- Default locale เป็นไทย เปลี่ยน English ได้ และจำ preference ด้วย non-sensitive locale cookie
- Status colors: ONLINE=emerald, DEGRADED/REVIEW=amber, OFFLINE/error=rose, UNKNOWN=slate, ISP risk=violet; ต้องมี icon/text ไม่ใช้สีอย่างเดียว
- Typography รองรับอักษรไทยชัดเจน
- Dense data table สำหรับ desktop และ card/list adaptation บน mobile โดยไม่เกิด horizontal page overflow
- ทุกหน้าและ dialog ต้องมี loading, skeleton, empty, error, permission denied, stale และ retry states ตามความเหมาะสม
- Confirm dialog พร้อม reason ที่บังคับกรอกสำหรับ archive, restore, override, revoke, reject และ destructive/effective actions
- Keyboard navigation, visible focus, semantic headings, form labels, aria-live และ contrast ต้องผ่าน WCAG 2.2 AA

Required pages:
1. Login
2. Dashboard
3. Domains
4. Domain Detail
5. Incidents
6. Finance
7. Recommendations
8. Reports
9. Google Sheet Sync / Excel Import / Google Drive Connect
10. Probe Nodes
11. Settings / System status

Working discipline:
- ตรวจ git status ก่อนเริ่มและรักษาไฟล์/การแก้ไขเดิมของผู้ใช้
- ห้ามแก้ root .env และห้าม commit secret
- ทำงานแบบ additive ใน web/ และแก้ root docker-compose.yml/README เฉพาะ Phase ที่สั่ง
- หลังแต่ละ Phase ให้รัน format, lint, typecheck และ tests ที่เกี่ยวข้อง
- รายงานไฟล์ที่เพิ่ม/แก้ คำสั่งที่รัน ผลจริง และข้อจำกัดที่ยังไม่ได้ตรวจ
- ห้ามกล่าวว่าเสร็จสมบูรณ์ถ้ายังไม่ได้ browser QA กับ backend จริง
- หยุดเมื่อจบ Prompt แต่ละ Phaseและรอ Prompt ถัดไป ห้ามข้าม Phase

ตอบรับโดยสรุป source files ที่จะอ่านและกฎสำคัญ 10 ข้อเท่านั้น ยังไม่แก้ไฟล์จนกว่าจะได้รับ Prompt 1
```

---

## Prompt 1 — Audit, contract matrix and frontend architecture

```text
เริ่ม Phase Frontend 1: Audit and Architecture

ทำงานใน D:\developer\domains-checker-infomation และปฏิบัติตาม Master Prompt

งาน:
1. ตรวจ git status และโครงสร้าง repository โดยไม่แก้ไฟล์เดิมของผู้ใช้
2. อ่าน source of truth ที่กำหนด โดยเฉพาะ internal/api/server.go, handlers และ api/openapi.yaml
3. ตรวจ backend ที่ http://localhost:8081 ด้วย /, /health, /ready และ /api/v1/meta/locales ถ้ารันอยู่ ห้ามเปิดเผย credential
4. สร้าง route-to-page contract matrix ครบทุก human admin route โดยระบุ:
   - method/path
   - roles
   - CSRF requirement
   - Idempotency-Key requirement
   - request/response type หรือ contract gap
   - หน้า/องค์ประกอบที่ใช้ route
5. ระบุ route ที่ server.go มีแต่ OpenAPI ไม่มี เช่น monitoring check/history/runs, incidents และ probes
6. วาง BFF/session/CSRF architecture รวม login, reload, logout, 401, 403, expired session และ OAuth popup flow
7. วาง component hierarchy, route map, data-fetching strategy, cache keys, polling limits, error model, i18n namespaces และ responsive behavior
8. วาง design tokens, status mapping และ RBAC capability matrix

สร้างเอกสารเท่านั้น:
- web/FRONTEND_ARCHITECTURE.md
- web/API_CONTRACT_MATRIX.md
- web/IMPLEMENTATION_PLAN.md
- web/API_GAPS.md

Acceptance criteria:
- ไม่มี application runtime code ใน Phase นี้
- ทุก API อ้างอิงหลักฐานจาก source จริง
- แยก confirmed contract กับ assumption ชัดเจน
- ไม่มี secret หรือค่าจาก .env ในเอกสาร
- Implementation plan แบ่ง milestone และ test gate ชัดเจน

เมื่อเสร็จ ให้สรุปข้อค้นพบที่เสี่ยงที่สุด 5 ข้อและหยุดรออนุมัติ
```

---

## Prompt 2 — Scaffold, design system, i18n and secure authentication

```text
เริ่ม Phase Frontend 2: Foundation and Authentication

อ่านเอกสารจาก Phase 1 และปฏิบัติตาม Master Prompt หากพบ blocker จริงให้รายงานก่อนเปลี่ยน architecture

งาน:
1. Scaffold Next.js App Router + TypeScript strict ใน web/ พร้อม package manager lockfile
2. ตั้ง lint, format, typecheck, unit test และ Playwright config
3. สร้าง design tokens, accessible primitives, icon strategy, Thai/English typography, light/dark/system theme โดย theme ต้องไม่ลด contrast
4. สร้าง responsive application shell:
   - sidebar/drawer
   - top bar
   - breadcrumb
   - language selector
   - user menu
   - readiness indicator
5. สร้าง i18n th/en ให้ข้อความ UI อยู่ใน catalog ห้าม hardcode กระจัดกระจาย
6. สร้าง typed API envelope/error parser และ server-only backend client
7. สร้าง BFF Route Handlers ที่ allowlist path/method ป้องกัน open proxy และ forward cookie/Accept-Language อย่างปลอดภัย
8. Implement login/logout/auth bootstrap:
   - Login form รองรับ validation, show/hide password, loading และ localized error
   - Forward backend Set-Cookie
   - เก็บ CSRF token แบบ HttpOnly/server-side ห้าม localStorage
   - auth/me route guard
   - 401 กลับ login พร้อม returnTo ที่ validate แล้ว
   - 403 เป็น permission page ไม่วน redirect
9. สร้าง capability helpers จาก ADMIN/STAFF/VIEWER และซ่อน/disable action อย่างถูกต้อง
10. สร้าง global loading/error/not-found pages และ toast ที่ accessible
11. สร้าง .env.local.example เฉพาะ:
    API_INTERNAL_URL=http://127.0.0.1:8081
    ห้ามใส่ secret หรือ admin credential

ต้องมี tests อย่างน้อย:
- success/error envelope parser
- localized backend error selection
- RBAC capability matrix
- returnTo validation ป้องกัน open redirect
- login form validation
- BFF path allowlist
- mutating request แนบ CSRF แต่ GET ไม่แนบ

Verification:
- install dependencies
- format check
- lint
- typecheck
- unit/component tests
- production build
- Playwright login smoke กับ backend จริงผ่าน environment credential ที่ไม่ commit ถ้ามี

ห้ามสร้าง mock dashboard เพื่อให้ดูเหมือนเสร็จ ใช้ skeleton/empty state ที่ honest ได้ เมื่อผ่านให้รายงานผลและหยุด
```

---

## Prompt 3 — Dashboard overview and operational navigation

```text
เริ่ม Phase Frontend 3: Dashboard Overview

สร้างหน้า Dashboard โดยใช้ข้อมูลจริงจาก endpoint ที่มีอยู่ เช่น:
- GET /api/v1/reports/summary
- GET /api/v1/finance/summary?reporting_currency=THB
- GET /api/v1/incidents
- GET /api/v1/recommendations
- GET /health และ /ready ผ่าน BFF/system endpoint ที่ออกแบบไว้

งาน UI:
1. KPI cards: total, active, unavailable, redirects, ISP suspected/high confidence, DNS/TLS errors และ expiring within 90 days
2. Recommendation distribution: RENEW, DROP, REVIEW, PROFIT_OPPORTUNITY
3. Budget overview ที่รักษา exact decimal string และ completeness warnings
4. Active/recent incidents section
5. Operational status: API, PostgreSQL, Redis และ last refresh time
6. Quick actions ตาม role: Add domain, Run recommendations, Import Excel, Connect Google Drive, Create report
7. Reporting currency selectorที่ไม่ทำให้ decimal ถูกแปลงผิด
8. Auto-refresh แบบ bounded เฉพาะข้อมูล operational พร้อม pause เมื่อ tab hidden และปุ่ม refresh manual

กฎ:
- หาก API ไม่มี time-series ห้ามสร้าง line chart ปลอม ใช้ KPI/distribution/list ที่ข้อมูลรองรับ
- unknown/stale/incomplete ต้องมี badge และคำอธิบาย
- Quick action ที่ role ใช้ไม่ได้ต้องไม่เรียก API
- Dashboard ต้อง usable ที่ 1440px, 900px และ 390px

Tests:
- summary mapping
- exact-money rendering
- incomplete warning
- role-based quick actions
- loading/empty/error/retry
- responsive browser screenshots และตรวจ no horizontal overflow/console error

รัน quality gates ทั้งหมดแล้วหยุดรายงาน
```

---

## Prompt 4 — Domains, domain detail, monitoring and incidents

```text
เริ่ม Phase Frontend 4: Domain Operations

สร้างหน้า Domains, Domain Detail และ Incidents เชื่อม API จริง

Domains page:
- ตาราง server-side search/filter/pagination
- Columns ตามข้อมูลที่ backend ส่งจริง: Domain, lifecycle/source status, effective status, confidence, HTTP/redirect, ISP status, registrar, expiry/days remaining, renewal cost, recommendation, last checked
- ถ้า field ใดไม่มีใน list response ห้ามเดา ให้แสดงเฉพาะ confirmed field หรือ explicit unknown และบันทึก API gap
- รองรับ query, lifecycle_status, source_status, page, page_size ตาม contract
- URL query state ต้อง share/bookmark ได้
- Add domain form ตาม CreateDomain schema
- Row actions ตาม role

Domain Detail:
- Overview แยก current effective state กับ latest observed result
- Domain edit ด้วย optimistic version และจัดการ 409 conflict แบบ reload/compare ไม่ overwrite เงียบ
- Manual check: POST /domains/{id}/check และ polling run แบบ bounded
- Monitoring runs/history และ run detail
- DNS local/DoH, HTTP, TLS, redirect chain, local/remote evidence ตาม response จริง
- RDAP latest/force check, registrar, nameservers, DNSSEC, expiry และ provenance conflicts
- Costs พร้อม exact decimals
- Manual overrides history/create/revoke พร้อม reason และ confirmation
- Recommendation latest/generate พร้อม bilingual reason codes/evidence
- Archive/restore พร้อม version + reason และไม่ใช้ hard delete
- Raw technical result อยู่ใน collapsible JSON viewer มี redaction/copy-safe behavior

Incidents:
- list/filter/empty/error states ตาม contract จริง
- severity/status/domain/timestamps/evidence links เท่าที่ API ส่ง
- link กลับ domain/run detail

ต้องรองรับ:
- ONLINE, DEGRADED, OFFLINE/UNAVAILABLE, UNKNOWN และ stale
- IDN Unicode/A-label
- keyboard accessible table/dialog/tabs
- mobile layout ไม่บังคับผู้ใช้ลากทั้งหน้าในแนวนอน

Tests:
- domain query serialization
- optimistic conflict handling
- archive/restore reason validation
- monitoring polling cancellation/timeout
- raw evidence redaction presentation
- browser E2E: list -> create test domain if explicitly allowed -> detail -> manual check -> run/history

อย่าสร้างหรือลบ test domain บนข้อมูลจริงโดยไม่มี environment flag E2E_ALLOW_MUTATION=true เมื่อเสร็จให้รัน gates และหยุด
```

---

## Prompt 5 — Finance, recommendations and reports

```text
เริ่ม Phase Frontend 5: Asset Intelligence

Finance page:
- GET /finance/summary พร้อม reporting currency
- แสดง acquisition/current cost, renewal cost, estimated tax, annual budget, 30/60/90-day/this-year windows และ completeness warnings
- exact decimal/currency formatting ต้องไม่คำนวณด้วย Number/parseFloat
- Admin form เพิ่ม audited exchange rate พร้อม rate string, source, observed_at และ reason
- Domain cost workflow อยู่ใน Domain Detail และ link จาก Finance

Recommendations page:
- GET /recommendations พร้อม filter action และ bounded limit
- แสดง generated action แยกจาก effective/manual override
- แสดง confidence, opportunity level, policy version, reason codes ไทย/English และ evidence references
- STAFF/ADMIN run bulk recommendations โดยยืนยัน limit ก่อนเรียก POST /recommendations/run
- ห้ามอธิบาย PROFIT_OPPORTUNITY ว่าเป็นราคาประเมิน
- REVIEW ต้องสื่อว่า evidence/cost อาจไม่ครบ ไม่ใช่ความล้มเหลวของระบบ

Reports page:
- GET /reports/summary
- Create immutable JSON/CSV report ด้วย Idempotency-Key ใหม่ต่อ intent
- แสดง report metadata, snapshot/as-of, SHA-256, warnings, row count และ format ตาม response จริง
- Download ผ่าน authenticated BFF แบบ streaming ห้ามโหลดไฟล์ใหญ่ทั้งหมดเข้าหน่วยความจำโดยไม่จำเป็น
- ตรวจ Content-Disposition และ X-Content-SHA256 อย่างปลอดภัย
- ห้ามสร้างปุ่ม PDF/XLSX เพราะ backend ยังไม่รองรับ

Tests:
- decimal-string preservation
- recommendation labels/reasons th/en
- generated vs effective action
- idempotency behavior
- safe authenticated download filename
- permission states
- E2E finance/recommendation/report flow เท่าที่ role อนุญาต

รัน gates และหยุดรายงาน
```

---

## Prompt 6 — Google Drive, Google Sheets and Excel import

```text
เริ่ม Phase Frontend 6: Import Center

สร้างหน้า Import Center ที่มี 3 entry points ชัดเจน:
1. Connect Google Drive และเลือก Google Sheet
2. Google Sheet scheduled/manual preview
3. Upload Excel .xlsx

Google Drive:
- GET /google-drive/connection แสดง connected account/status โดยไม่แสดง token
- POST /google-drive/connect รับ authorization URL
- เปิด OAuth ด้วย popup ที่ user gesture; ระหว่าง popup เปิดให้ poll connection แบบ bounded และปิด popup จาก opener เมื่อ connection active
- รองรับ popup blocked, user cancel, OAuth error, expired state และ DRIVE_NOT_CONFIGURED
- GET /google-drive/files พร้อม page_token และเลือกเฉพาะ Google Sheets ที่ API ส่ง
- DELETE connection ต้อง ADMIN + reason + CSRF confirmation และอธิบายผลก่อนทำ
- ห้ามขอหรือแสดง GOOGLE_SHEETS_ACCESS_TOKEN ใน UI

Google Sheet config:
- GET/PUT /google-sheets/config
- เลือก connection_id, spreadsheet_id, sheet_name, range, interval, enabled และ semantic column_mapping
- sync interval อยู่ใน 5–10080 นาที
- ใช้ optimistic version และจัดการ 409
- Preview ต้องมี Idempotency-Key

Excel upload:
- รับเฉพาะ .xlsx ใน UI แต่ backend validation เป็น authority
- fields: file, source_name, sheet_name, column_mapping JSON
- แสดง file size และ upload state ห้ามเก็บไฟล์ใน browser หลังจบ flowเกินจำเป็น
- รองรับ 413/422 และ localized error details

Preview/review/apply:
- แสดง counts และ rows: ADD, MODIFY, UNCHANGED, MISSING, INVALID
- diff แบบ before/after และ validation reason
- filter ตาม action และค้นหา domain
- invalid rows ห้ามทำให้ดูเหมือนจะถูก apply
- MISSING ต้องเตือนว่าจะเปลี่ยน source status/ปิด schedule ไม่ใช่ hard delete
- Apply เฉพาะ ADMIN พร้อม reason + Idempotency-Key + confirmation summary
- Reject พร้อม reason
- Import history/detail ใช้ endpoint จริง

Security:
- ห้าม expose OAuth access/refresh token
- ห้ามส่ง root secret ไป client
- ป้องกัน spreadsheet formula injection ใน UI/export preview display
- sanitize filenames และ render cell values เป็น text เท่านั้น

Tests:
- OAuth popup/poll lifecycle
- Drive unconfigured/error/connected states
- Excel type/size client hint และ backend rejection
- mapping validation
- ADD/MODIFY/UNCHANGED/MISSING/INVALID rendering
- apply/reject RBAC, reason และ idempotency
- Playwright ทั้ง Google fixture/mocked contract test และ real backend Excel fixture ที่ไม่มี secret

รัน gates และหยุดรายงาน ห้ามอ้างว่า real Google OAuth ผ่านถ้ายังไม่ได้ทดสอบ consent จริง
```

---

## Prompt 7 — Probe nodes, settings and system administration

```text
เริ่ม Phase Frontend 7: Probe Nodes and Settings

Probe Nodes page:
- GET /probes แสดง node, region, status, version/capabilities, heartbeat/staleness ตาม response จริง
- ADMIN สร้าง registration token ด้วย POST /probes/registration-tokens
- Secret/token ที่สร้างให้แสดงครั้งเดียว พร้อม copy button, warning และห้าม log/persist ใน browser storage
- ADMIN revoke probe ด้วย reason/confirmation
- สถานะ ONLINE, DEGRADED, OFFLINE, REVOKED, UPGRADE_REQUIRED ต้องมีข้อความและ icon ไม่ใช้สีอย่างเดียว
- แสดง required region และ stale status เฉพาะเมื่อ API ส่งหรือ config endpointรองรับ ห้ามอ่าน root .env จาก browser

Settings/System page:
- แสดง profile/roles/locale จาก auth/me
- language preference th/en
- API health/readiness และ supported locales
- แสดง app/frontend build version จาก public build metadata
- Google connection summary และ link ไป Import Center
- ห้ามสร้าง UI แก้ environment secrets
- ถ้า backend ไม่มี endpoint แก้ user/profile/schedule global ให้เป็น read-only พร้อมข้อความชัดเจน ห้ามสร้าง API ปลอม

Navigation/RBAC audit:
- VIEWER อ่านอย่างเดียว
- STAFF ทำ operational actions ที่ server.go อนุญาต
- ADMIN เห็น admin-only actions
- ทดสอบ direct URL access ด้วย ไม่ใช่ซ่อนเมนูอย่างเดียว

Tests:
- probe state mapping/staleness
- one-time token handling
- revoke confirmation/reason
- role-based page/action access
- system status degradation
- th/en completeness

รัน gates และหยุดรายงาน
```

---

## Prompt 8 — Docker integration, E2E, accessibility and visual QA

```text
เริ่ม Phase Frontend 8: Integration and Quality Gate

ทำให้ Frontend รันร่วมกับ Docker Compose เดิมได้โดยรักษา backend services:

1. สร้าง production multi-stage web/Dockerfile ที่รันเป็น non-root
2. เพิ่ม web service ใน root docker-compose.yml แบบ additive
3. ภายใน Compose ใช้ server-only API_INTERNAL_URL=http://api:8080
4. เปิด frontend ที่ 127.0.0.1:3000 โดยไม่ expose secret
5. เพิ่ม healthcheck และ depends_on ที่เหมาะสม
6. เพิ่ม .dockerignore และตรวจว่า .env/.env.local/keys ไม่เข้า image context
7. อัปเดต README ด้วยคำสั่ง startup, frontend URL, test และ troubleshooting

Full verification:
- clean dependency install จาก lockfile
- format/lint/typecheck/unit/component
- production build
- Docker Compose build/up
- backend /health และ /ready 200
- frontend health/root/login 200
- authenticated Playwright flows กับ backend จริง โดยใช้ E2E_EMAIL/E2E_PASSWORD จาก environment เท่านั้น
- Login/logout/session expiry/401/403
- Dashboard
- Domains list/detail
- Finance/recommendations/reports
- Excel preview review flow
- Google Drive unconfigured หรือ fixture flow โดยรายงาน boundary ตามจริง
- Probe list

Browser matrix:
- Desktop 1440x900
- Tablet 900x900
- Mobile 390x844

ตรวจทุกขนาด:
- no page-level horizontal overflow
- no clipped dialog/menu/table control
- console ไม่มี uncaught error/hydration warning
- network ไม่มี request loop
- keyboard navigation และ visible focus
- axe/WCAG 2.2 AA ไม่มี serious/critical violation
- loading/empty/error/stale/permission states
- ภาษาไทยไม่ตัดหรือทับกัน และ English ไม่ทำ layout แตก

Performance:
- ตรวจ bundle และ client components ที่ไม่จำเป็น
- no waterfall ที่แก้ได้ง่าย
- pagination/filter ไม่โหลดข้อมูลทั้งหมด
- polling bounded, abort on unmount และ pause hidden tab
- image/icon/font strategy ไม่ทำให้ LCP แย่โดยไม่จำเป็น

หากพบ defect ให้แก้และ rerun test ที่เกี่ยวข้องจนผ่าน ห้ามปิด test หรือผ่อน assertion เพื่อให้เขียว

ส่งรายงาน PASS/FAIL พร้อม command, evidence และข้อจำกัดจริง แล้วหยุด
```

---

## Prompt 9 — Final audit, cleanup and delivery

```text
เริ่ม Phase Frontend 9: Final Production Audit

ตรวจ frontend ทั้งหมดแบบ Principal Engineer โดยไม่ถือว่าผลงานเดิมถูกต้องอัตโนมัติ

Audit checklist:
- เทียบทุก route/action กับ internal/api/server.go และ api/openapi.yaml
- ไม่มี fake API, fake data, TODO ที่กระทบ flow หลัก หรือ dead navigation
- ไม่มี secret, real credential, cookie/token dump หรือ .env ถูก track
- BFF ไม่เป็น open proxy และไม่ยอม forward arbitrary host/header
- session/CSRF/idempotency/RBAC ถูกต้อง
- locale th/en ครบ ไม่มี key หลุดบนหน้าจอ
- exact decimal และ date-only semantics ไม่เสีย
- reason/confirmation/optimistic conflict ครบ
- Google tokens ไม่สัมผัส browser JS
- downloads/uploads ปลอดภัย
- loading/empty/error/stale/unknown/permission states ครบ
- responsive/accessibility/browser QA ผ่าน
- Docker startup จาก documented command ใช้ได้จริง

รัน final commands ทั้งหมด:
- frontend format check
- lint
- typecheck
- unit/component tests
- production build
- Playwright E2E
- Docker Compose config/build/up
- backend and frontend smoke tests
- git diff --check
- secret filename/pattern scan โดยไม่พิมพ์ secret

สร้าง/อัปเดต:
- web/README.md
- web/API_GAPS.md ให้เหลือเฉพาะ gap จริง
- FRONTEND_DELIVERY_REPORT.md ที่ root ระบุ architecture, pages, tests, browser sizes, known limitations และคำสั่งรัน

อย่า commit หรือ push จนกว่าผู้ใช้จะสั่ง

Final response ต้อง:
1. นำด้วยผลลัพธ์
2. ระบุไฟล์หลักที่เพิ่ม/แก้
3. ระบุ test ที่ผ่านพร้อมจำนวนถ้ามี
4. ระบุ URL ใช้งาน
5. ระบุข้อจำกัดที่ยังไม่ได้ verify อย่างตรงไปตรงมา
6. ระบุ git status

หากทุก gate ผ่านจริงจึงใช้คำว่า production-ready หากมี gate ใดไม่ได้รัน ให้ใช้คำว่า ready for the verified scope เท่านั้น
```

---

## Optional prompt — ให้ Claude ทำต่อหลังถูกแก้ไขหรือ context หาย

```text
อ่าน CLAUDE_FRONTEND_PROMPTS.md, web/FRONTEND_ARCHITECTURE.md, web/IMPLEMENTATION_PLAN.md, web/API_CONTRACT_MATRIX.md, web/API_GAPS.md และ git diff ปัจจุบันก่อน

อย่าเริ่มใหม่หรือทับงานเดิม ให้ระบุ Phase ล่าสุดที่เสร็จจากหลักฐานไฟล์และ test output จากนั้นทำเฉพาะ Phase ถัดไปตาม CLAUDE_FRONTEND_PROMPTS.md

รักษา uncommitted changes ของผู้ใช้ ห้ามอ่าน/พิมพ์/commit secret และห้ามอ้าง test เก่าที่ไม่ได้ rerun หลังการแก้ล่าสุด
```
