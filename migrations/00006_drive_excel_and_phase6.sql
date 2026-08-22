-- +goose Up

ALTER TYPE recommendation_action ADD VALUE IF NOT EXISTS 'PROFIT_OPPORTUNITY';

CREATE TABLE google_drive_connections (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL UNIQUE REFERENCES users(id) ON DELETE RESTRICT,
    google_email citext,
    access_token_ciphertext bytea,
    access_token_nonce bytea,
    refresh_token_ciphertext bytea,
    refresh_token_nonce bytea,
    token_expires_at timestamptz,
    scopes text[] NOT NULL DEFAULT '{}',
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'revoked', 'error')),
    last_error_code text,
    connected_at timestamptz NOT NULL DEFAULT now(),
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    CHECK ((status = 'active') = (revoked_at IS NULL)),
    CHECK ((access_token_ciphertext IS NULL) = (access_token_nonce IS NULL)),
    CHECK ((refresh_token_ciphertext IS NULL) = (refresh_token_nonce IS NULL))
);
CREATE TRIGGER google_drive_connections_set_updated_at BEFORE UPDATE ON google_drive_connections
FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE INDEX google_drive_connections_status_idx ON google_drive_connections (status, updated_at DESC);

CREATE TABLE google_oauth_states (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    state_hash bytea NOT NULL UNIQUE CHECK (octet_length(state_hash) = 32),
    verifier_ciphertext bytea NOT NULL,
    verifier_nonce bytea NOT NULL,
    expires_at timestamptz NOT NULL,
    consumed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (expires_at > created_at)
);
CREATE INDEX google_oauth_states_expiry_idx ON google_oauth_states (expires_at) WHERE consumed_at IS NULL;

ALTER TABLE google_sheet_configs
    ADD COLUMN connection_id uuid REFERENCES google_drive_connections(id) ON DELETE RESTRICT;

DROP INDEX IF EXISTS google_sheet_imports_open_snapshot_idx;
ALTER TABLE google_sheet_imports
    ADD COLUMN source_kind text NOT NULL DEFAULT 'google_sheets'
        CHECK (source_kind IN ('google_sheets', 'google_drive', 'excel')),
    ADD COLUMN source_metadata jsonb NOT NULL DEFAULT '{}'::jsonb;
CREATE UNIQUE INDEX google_sheet_imports_open_snapshot_idx
    ON google_sheet_imports (source_kind, spreadsheet_id, sheet_name, source_hash)
    WHERE status = 'preview' AND source_hash IS NOT NULL;

CREATE TABLE report_payloads (
    report_id uuid PRIMARY KEY REFERENCES reports(id) ON DELETE RESTRICT,
    content_type text NOT NULL CHECK (content_type IN ('application/json', 'text/csv')),
    content bytea NOT NULL CHECK (octet_length(content) <= 16777216),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX recommendations_latest_idx ON recommendations (domain_id, generated_at DESC, created_at DESC);

INSERT INTO system_settings (key, value, description_th, description_en) VALUES
    ('recommendation.policy_version', '"recommendation-2026-08-v1"'::jsonb, 'เวอร์ชันกฎคำแนะนำ Phase 6', 'Phase 6 recommendation rule version'),
    ('recommendation.expiring_soon_days', '90'::jsonb, 'จำนวนวันก่อนหมดอายุที่ถือว่าใกล้หมดอายุ', 'Days before expiry considered expiring soon')
ON CONFLICT (key) DO NOTHING;

-- +goose Down

DELETE FROM system_settings WHERE key IN ('recommendation.policy_version', 'recommendation.expiring_soon_days');
DROP INDEX IF EXISTS recommendations_latest_idx;
DROP TABLE IF EXISTS report_payloads;
DROP INDEX IF EXISTS google_sheet_imports_open_snapshot_idx;
ALTER TABLE google_sheet_imports
    DROP COLUMN IF EXISTS source_metadata,
    DROP COLUMN IF EXISTS source_kind;
CREATE UNIQUE INDEX google_sheet_imports_open_snapshot_idx
    ON google_sheet_imports (spreadsheet_id, sheet_name, source_hash)
    WHERE status = 'preview' AND source_hash IS NOT NULL;
ALTER TABLE google_sheet_configs DROP COLUMN IF EXISTS connection_id;
DROP TABLE IF EXISTS google_oauth_states;
DROP TABLE IF EXISTS google_drive_connections;
-- PostgreSQL enum values are intentionally retained on rollback.
