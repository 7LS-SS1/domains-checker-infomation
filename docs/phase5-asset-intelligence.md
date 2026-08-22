# Phase 5 — RDAP, Google Sheets and Financial Engine

เอกสารนี้อธิบาย contract ที่ implement แล้วใน Phase 5 ทั้งภาษาไทยและ English-facing API behavior. API errors ทุกตัวใช้ stable code พร้อม `message`, `messages.th`, `messages.en`, `locale` และ `request_id`.

## 1. RDAP registration intelligence

ระบบโหลด DNS bootstrap registry จาก IANA ที่ `https://data.iana.org/rdap/dns.json`, เลือก authoritative HTTPS base URL ตาม A-label TLD แล้วเรียก RDAP domain lookup ที่ `/domain/{domain}`. Bootstrap cache เก็บ payload hash, ETag, Last-Modified, fetch/expiry time และใช้ stale cache ได้ไม่เกิน policy พร้อม `bootstrap_stale=true`.

Network policy:

- HTTPS และ public IP/port 443 เท่านั้น; ใช้ SSRF-safe dialer ชุดเดียวกับ protocol checkers
- response body และ headers ถูกจำกัดขนาด
- rate limit แยกตาม RDAP host, retry จำกัด และเคารพ bounded `Retry-After`
- successful domain result มี cache TTL; `force=true` ข้าม domain cache แต่ไม่ข้าม safety policy
- raw payload เก็บ SHA-256 และ excerpt ไม่เกิน 256 KiB

ข้อมูลที่ normalize ได้แก่ registrar name, registrar IANA ID, registration/expiration/updated events, nameservers, DNSSEC และ domain statuses. Field ที่ RDAP ไม่ส่งจะเป็น `null`/empty และ `source_status=partial|unavailable`; ระบบไม่สร้างข้อมูลทดแทนเอง.

RDAP ไม่เขียนทับ registrar/expiration ที่มีอยู่โดยเงียบ หากค่าไม่ตรงกันจะสร้าง `provenance_conflicts` และเก็บ RDAP observation ใน `domain_field_provenance` เพื่อให้ Admin ตรวจสอบ.

Endpoints:

- `GET /api/v1/domains/{domain_id}/rdap`
- `POST /api/v1/domains/{domain_id}/rdap-check` body `{"force": false}`
- `GET /api/v1/domains/{domain_id}/provenance`

Reference contracts: [IANA RDAP DNS bootstrap registry](https://www.iana.org/assignments/rdap-dns/rdap-dns.xhtml) and [RFC 9082 RDAP query format](https://www.rfc-editor.org/rfc/rfc9082.html).

## 2. Google Sheets inventory sync

Initial semantic columns:

```text
domain
registrar
purchase_price
renewal_price
currency
tax_rate
purchase_date
expiration_date
business_priority
notes
active
```

Importer ผูกด้วยชื่อ header/semantic mapping ไม่ผูก column index. รองรับ aliases ภาษาไทย/อังกฤษและ saved mapping; duplicate/ambiguous header จะไม่ถูกเดา. `domain` เป็น required column.

Credential modes:

1. `GOOGLE_SERVICE_ACCOUNT_CREDENTIALS_FILE` — แนะนำสำหรับ scheduled sync; scope เป็น read-only Sheets
2. `GOOGLE_SHEETS_API_KEY` — public sheet เท่านั้น
3. `GOOGLE_SHEETS_ACCESS_TOKEN` — short-lived development token

Secret ไม่ถูกบันทึกใน PostgreSQL, API response หรือ audit log. Service account JSON ต้อง mount เป็นไฟล์นอก image.

Preview algorithm:

1. Fetch `spreadsheets.values.get` with row-major, unformatted values.
2. Hash source snapshot and resolve semantic mapping.
3. Normalize domain/date/bool/currency/exact decimal.
4. Mark each row as `ADD`, `MODIFY`, `UNCHANGED`, `MISSING` or `INVALID`.
5. Duplicate normalized domains ทำให้ทุก duplicate row เป็น `INVALID`.
6. Persist preview and per-row diff/history before any inventory mutation.

Apply behavior:

- `Idempotency-Key` บังคับทั้ง preview และ apply
- valid rows ถูก apply ใน PostgreSQL transaction เดียว; invalid rows ถูก exclude และนับไว้
- `MISSING` เปลี่ยน `source_status=missing_from_source`, ปิด schedule และไม่ hard delete
- `active=false` ทำให้ domain จาก source เป็น inactive/monitoring disabled โดยไม่ลบ history
- domain ที่ reappear กลับเป็น `present`; manual archive ยังมี precedence
- active manual override ป้องกัน Sheet จากการเปลี่ยน effective expiration/business priority; Sheet observation และ price history ยังถูกเก็บเป็น provenance
- scheduled sync ใช้ config lease/due time และสร้าง **preview เท่านั้น** เพื่อคง approval gate; Admin apply หรือ reject ภายหลัง

Endpoints:

- `GET|PUT /api/v1/google-sheets/config`
- `POST /api/v1/google-sheets/previews`
- `GET /api/v1/google-sheets/imports`
- `GET /api/v1/google-sheets/imports/{import_id}`
- `POST /api/v1/google-sheets/imports/{import_id}/apply`
- `POST /api/v1/google-sheets/imports/{import_id}/reject`

Google contract: [Sheets API `spreadsheets.values.get`](https://developers.google.com/workspace/sheets/api/reference/rest/v4/spreadsheets.values/get).

## 3. Financial engine

ทุก amount/rate ผ่าน exact rational decimal และ JSON เป็น string; ไม่ใช้ `float64` ใน business calculation. PostgreSQL ใช้ `NUMERIC(20,6)` สำหรับ money และ `NUMERIC(20,10)` สำหรับ rate.

Tax policies:

- `exclusive`: `tax = amount × tax_rate`, `cycle_total = amount + tax`
- `inclusive`: `tax = amount - amount/(1+tax_rate)`, `cycle_total = amount`
- `exempt`: `tax = 0`
- `unknown`: tax/total/annual เป็น incomplete ไม่ตีความ tax เป็นศูนย์

Annual normalization:

```text
annual_estimate = cycle_total × (12 / billing_cycle_months)
```

ระบบส่งทั้ง cycle total, annual estimate และ formula. Price source precedence คือ `registrar_api`, `google_sheet`, `manual`, `estimate`; explicit manual override ถูกเก็บแยกและมี precedence สูงสุดระหว่างช่วงที่มีผล.

FX conversion ใช้เพื่อ reporting เท่านั้น: original amount/currency ไม่ถูกแก้. Report ใช้ rate ล่าสุดที่ไม่เกิน `FINANCE_FX_MAX_AGE`, พร้อม source/timestamp. Missing/stale rate ทำให้ total เป็น incomplete และเพิ่ม warning/count.

Budget summary มี acquisition/current cost, renewal cost, estimated tax, normalized annual budget, unknown cost/tax/FX counts และ windows `next_30_days`, `next_60_days`, `next_90_days`, `this_year`.

Endpoints:

- `GET|POST /api/v1/domains/{domain_id}/costs`
- `GET /api/v1/finance/summary?reporting_currency=THB`
- `POST /api/v1/finance/exchange-rates`
- `GET|POST /api/v1/domains/{domain_id}/overrides`
- `DELETE /api/v1/domains/{domain_id}/overrides/{override_id}`

Manual override fields in Phase 5: `recommendation`, `renewal_price`, `tax_rate`, `expiration_date`, `business_priority`. ทุก record เก็บ original/override value, user, reason, effective time, optional expiry และ revocation time.

## 4. Verification coverage

- exact `0.1 + 0.2`, tax modes, 6/12-month billing and stale FX unit tests
- missing RDAP fields, normalized events/registrar/DNSSEC, 429 and 503 failure tests
- Sheet semantic mapping, ambiguous headers, duplicates and invalid rows
- PostgreSQL integration: add/modify/missing, duplicate/invalid isolation, preview/apply idempotency, scheduled preview, override, price history, budget summary and RDAP conflict persistence
- migration 00005 is additive and preserves Phase 1–4 records

