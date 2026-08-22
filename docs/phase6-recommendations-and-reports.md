# Phase 6: Recommendations and Reports

Phase 6 adds explainable, deterministic asset recommendations and reproducible summary reports. The recommendation engine never uses an LLM as the primary decision-maker and never invents a monetary domain valuation.

## Google Drive and Excel import entry points

- `POST /api/v1/google-drive/connect` returns an OAuth authorization URL for a Connect button. OAuth uses a one-time hashed state, PKCE S256, offline access, and a 10-minute expiry.
- `GET /api/v1/google-drive/callback` exchanges the authorization code. Access/refresh tokens are encrypted with AES-256-GCM before PostgreSQL persistence and are never included in API/audit responses.
- `GET /api/v1/google-drive/files` returns at most 100 Google Sheets per page. The default `drive.file` scope deliberately shows only files explicitly shared with or opened by the app.
- `PUT /api/v1/google-sheets/config` accepts the selected `connection_id` and `spreadsheet_id`. Scheduled sync still creates a preview; it never applies inventory changes automatically.
- `POST /api/v1/google-sheets/excel/previews` accepts multipart `.xlsx` upload fields `file`, optional `source_name`, `sheet_name`, and JSON `column_mapping`. The workbook enters the same `ADD/MODIFY/UNCHANGED/MISSING/INVALID` review/apply pipeline.

Excel uploads reject `.xls`, `.xlsm`, malformed ZIP data, oversized compressed/uncompressed workbooks, and worksheets exceeding configured row/column limits. Files are not retained after the normalized preview is persisted. Reusing a stable `source_name` enables missing-row detection across later workbook versions; no row is hard-deleted.

## Rule-based recommendation policy

Policy version: `recommendation-2026-08-v1`.

Outputs are `RENEW`, `DROP`, `REVIEW`, or `PROFIT_OPPORTUNITY`. Every immutable result stores:

- the complete input snapshot used by the rule engine;
- stable reason codes plus Thai and English reasons;
- evidence references, confidence score/level, policy version, opportunity level, and superseded recommendation;
- the generated action and, when present, a separate effective action from an audited manual override.

The policy is intentionally conservative:

- incomplete monitoring evidence, missing renewal cost, ISP-block evidence, open incidents, downtime, or DNS/TLS/availability failures produce `REVIEW`;
- `DROP` requires the combined evidence of inactive lifecycle, low business priority, known renewal cost, and a permanent redirect;
- healthy high/critical-priority or expiring active domains can produce `RENEW`;
- `PROFIT_OPPORTUNITY` is an indicator level based on name length, hyphen/digit pattern, TLD, monitoring history, and business context. It is not a price estimate.

Bulk generation uses one bounded set-based input query (maximum 1,000 domains) and transactional immutable inserts. Manual recommendation overrides remain in `manual_overrides` with original/effective value, actor, reason, timestamps, and optional expiry.

## Reports

`GET /api/v1/reports/summary` returns:

- total/active/unavailable domains and permanent redirects;
- suspected and high-confidence ISP blocking;
- DNS/TLS error and expiring-within-90-days counts;
- exact-string renewal cost, estimated tax, and annual budget with completeness warnings;
- effective RENEW, DROP, REVIEW, and profit-opportunity counts.

`POST /api/v1/reports` requires `Idempotency-Key` and persists the same snapshot as `json` or `csv`. Reusing the key for the same authenticated requester returns the original report. `GET /api/v1/reports/{id}/download` returns the immutable bounded payload and its SHA-256 header. Renderers are isolated behind the report service so XLSX/PDF can be added without changing summary calculations.

## Required OAuth configuration

Interactive Drive connection is optional. When enabled, configure all of:

- `GOOGLE_OAUTH_CLIENT_ID`
- `GOOGLE_OAUTH_CLIENT_SECRET`
- `GOOGLE_OAUTH_REDIRECT_URL`
- `GOOGLE_OAUTH_TOKEN_ENCRYPTION_KEY` (standard base64 for exactly 32 random bytes)

Production redirect URLs must use HTTPS. Localhost HTTP is accepted only for development. Rotating the encryption key requires an explicit token re-encryption/reconnection procedure; changing it directly makes existing connections unreadable.

## Verification boundary

Automated integration coverage uses a local OAuth/Drive fixture to verify state/PKCE flow, encrypted-at-rest tokens, file listing, Excel preview/apply, recommendation generation, report persistence, SHA-256, and JSON validity. A real Google account flow still requires deployment-specific OAuth credentials, consent-screen configuration, redirect registration, and browser testing.
