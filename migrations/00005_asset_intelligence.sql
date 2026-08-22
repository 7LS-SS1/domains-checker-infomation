-- +goose Up

CREATE TABLE rdap_bootstrap_cache (
    registry_key text PRIMARY KEY,
    source_url text NOT NULL,
    etag text,
    last_modified text,
    payload jsonb NOT NULL,
    payload_hash bytea NOT NULL CHECK (octet_length(payload_hash) = 32),
    fetched_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    last_error_code text,
    last_error_at timestamptz,
    CHECK (length(trim(registry_key)) > 0),
    CHECK (expires_at > fetched_at)
);

ALTER TABLE rdap_checks
    ADD COLUMN response_headers jsonb NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN bootstrap_stale boolean NOT NULL DEFAULT false,
    ADD COLUMN cache_until timestamptz,
    ADD COLUMN confidence_score smallint NOT NULL DEFAULT 0 CHECK (confidence_score BETWEEN 0 AND 99),
    ADD COLUMN conflicts jsonb NOT NULL DEFAULT '[]'::jsonb;

CREATE TABLE google_sheet_configs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    spreadsheet_id text NOT NULL,
    sheet_name text NOT NULL,
    sheet_range text NOT NULL DEFAULT 'A:Z',
    column_mapping jsonb NOT NULL,
    enabled boolean NOT NULL DEFAULT false,
    sync_interval_minutes integer NOT NULL DEFAULT 60 CHECK (sync_interval_minutes BETWEEN 5 AND 10080),
    next_sync_at timestamptz,
    last_sync_at timestamptz,
    updated_by uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    CHECK (length(trim(spreadsheet_id)) BETWEEN 1 AND 500),
    CHECK (length(trim(sheet_name)) BETWEEN 1 AND 200),
    CHECK (length(trim(sheet_range)) BETWEEN 1 AND 500)
);
CREATE TRIGGER google_sheet_configs_set_updated_at BEFORE UPDATE ON google_sheet_configs
FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE UNIQUE INDEX google_sheet_configs_singleton_idx ON google_sheet_configs ((true));
CREATE INDEX google_sheet_configs_due_idx ON google_sheet_configs (next_sync_at) WHERE enabled = true;

ALTER TABLE google_sheet_imports
    ADD COLUMN config_id uuid REFERENCES google_sheet_configs(id) ON DELETE RESTRICT,
    ADD COLUMN idempotency_key text,
    ADD COLUMN apply_idempotency_key text,
    ADD COLUMN applied_by uuid REFERENCES users(id) ON DELETE RESTRICT,
    ADD COLUMN rejected_by uuid REFERENCES users(id) ON DELETE RESTRICT,
    ADD COLUMN trigger_type text NOT NULL DEFAULT 'manual' CHECK (trigger_type IN ('manual', 'scheduled')),
    ADD COLUMN valid_rows_applied integer NOT NULL DEFAULT 0 CHECK (valid_rows_applied >= 0);
CREATE UNIQUE INDEX google_sheet_imports_preview_idempotency_idx
    ON google_sheet_imports (idempotency_key) WHERE idempotency_key IS NOT NULL;
CREATE UNIQUE INDEX google_sheet_imports_apply_idempotency_idx
    ON google_sheet_imports (apply_idempotency_key) WHERE apply_idempotency_key IS NOT NULL;
CREATE UNIQUE INDEX google_sheet_imports_open_snapshot_idx
    ON google_sheet_imports (spreadsheet_id, sheet_name, source_hash) WHERE status = 'preview' AND source_hash IS NOT NULL;

CREATE TABLE domain_field_provenance (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    domain_id uuid NOT NULL REFERENCES domains(id) ON DELETE RESTRICT,
    field_name text NOT NULL,
    value jsonb,
    source text NOT NULL CHECK (source IN ('manual', 'google_sheet', 'registrar_api', 'rdap', 'estimate')),
    source_reference text,
    observed_at timestamptz NOT NULL,
    is_current boolean NOT NULL DEFAULT true,
    supersedes_id uuid REFERENCES domain_field_provenance(id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (length(trim(field_name)) > 0)
);
CREATE UNIQUE INDEX domain_field_provenance_current_idx
    ON domain_field_provenance (domain_id, field_name) WHERE is_current = true;
CREATE INDEX domain_field_provenance_history_idx
    ON domain_field_provenance (domain_id, field_name, observed_at DESC);

CREATE TABLE provenance_conflicts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    domain_id uuid NOT NULL REFERENCES domains(id) ON DELETE RESTRICT,
    field_name text NOT NULL,
    current_source text NOT NULL,
    current_value jsonb,
    observed_source text NOT NULL,
    observed_value jsonb,
    source_reference text,
    status text NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'resolved', 'ignored')),
    detected_at timestamptz NOT NULL DEFAULT now(),
    resolved_at timestamptz,
    resolved_by uuid REFERENCES users(id) ON DELETE RESTRICT,
    resolution_reason text,
    CHECK ((status = 'open') = (resolved_at IS NULL))
);
CREATE INDEX provenance_conflicts_domain_idx
    ON provenance_conflicts (domain_id, status, detected_at DESC);

ALTER TABLE domain_costs
    ADD COLUMN tax_mode text NOT NULL DEFAULT 'unknown'
        CHECK (tax_mode IN ('exclusive', 'inclusive', 'exempt', 'unknown'));
UPDATE domain_costs
SET tax_mode = CASE
    WHEN tax_rate IS NULL THEN 'unknown'
    WHEN tax_inclusive THEN 'inclusive'
    ELSE 'exclusive'
END;
ALTER TABLE domain_costs ADD CONSTRAINT domain_costs_tax_policy_check CHECK (
    (tax_mode = 'unknown' AND tax_rate IS NULL) OR
    (tax_mode = 'exempt' AND COALESCE(tax_rate, 0) = 0) OR
    (tax_mode IN ('exclusive', 'inclusive') AND tax_rate IS NOT NULL)
);

CREATE INDEX domain_costs_current_precedence_idx
    ON domain_costs (domain_id, cost_type, price_source, effective_from DESC)
    WHERE effective_to IS NULL;
CREATE INDEX manual_overrides_effective_idx
    ON manual_overrides (domain_id, field_name, effective_from DESC)
    WHERE revoked_at IS NULL;

INSERT INTO system_settings (key, value, description_th, description_en) VALUES
    ('finance.reporting_currency', '"THB"'::jsonb, 'สกุลเงินเริ่มต้นสำหรับรายงาน', 'Default reporting currency'),
    ('finance.fx_max_age_hours', '48'::jsonb, 'อายุสูงสุดของอัตราแลกเปลี่ยนเป็นชั่วโมง', 'Maximum exchange-rate age in hours'),
    ('rdap.bootstrap_ttl_hours', '24'::jsonb, 'อายุแคช IANA RDAP bootstrap เป็นชั่วโมง', 'IANA RDAP bootstrap cache TTL in hours');

-- +goose Down

DELETE FROM system_settings WHERE key IN ('finance.reporting_currency', 'finance.fx_max_age_hours', 'rdap.bootstrap_ttl_hours');
DROP INDEX IF EXISTS manual_overrides_effective_idx;
DROP INDEX IF EXISTS domain_costs_current_precedence_idx;
ALTER TABLE domain_costs DROP CONSTRAINT IF EXISTS domain_costs_tax_policy_check;
ALTER TABLE domain_costs DROP COLUMN IF EXISTS tax_mode;
DROP TABLE IF EXISTS provenance_conflicts;
DROP TABLE IF EXISTS domain_field_provenance;
DROP INDEX IF EXISTS google_sheet_imports_open_snapshot_idx;
DROP INDEX IF EXISTS google_sheet_imports_apply_idempotency_idx;
DROP INDEX IF EXISTS google_sheet_imports_preview_idempotency_idx;
ALTER TABLE google_sheet_imports
    DROP COLUMN IF EXISTS valid_rows_applied,
    DROP COLUMN IF EXISTS trigger_type,
    DROP COLUMN IF EXISTS rejected_by,
    DROP COLUMN IF EXISTS applied_by,
    DROP COLUMN IF EXISTS apply_idempotency_key,
    DROP COLUMN IF EXISTS idempotency_key,
    DROP COLUMN IF EXISTS config_id;
DROP TABLE IF EXISTS google_sheet_configs;
ALTER TABLE rdap_checks
    DROP COLUMN IF EXISTS conflicts,
    DROP COLUMN IF EXISTS confidence_score,
    DROP COLUMN IF EXISTS cache_until,
    DROP COLUMN IF EXISTS bootstrap_stale,
    DROP COLUMN IF EXISTS response_headers;
DROP TABLE IF EXISTS rdap_bootstrap_cache;
