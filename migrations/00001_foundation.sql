-- +goose Up

CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS citext;

CREATE TYPE domain_lifecycle_status AS ENUM ('active', 'inactive', 'archived');
CREATE TYPE source_presence_status AS ENUM ('present', 'missing_from_source', 'unknown');
CREATE TYPE business_priority AS ENUM ('low', 'medium', 'high', 'critical');
CREATE TYPE availability_status AS ENUM ('ACTIVE', 'UNAVAILABLE', 'DEGRADED', 'UNKNOWN');
CREATE TYPE dns_status AS ENUM ('OK', 'NXDOMAIN', 'SERVFAIL', 'REFUSED', 'TIMEOUT', 'NETWORK_ERROR', 'DISCREPANCY', 'UNKNOWN');
CREATE TYPE http_status AS ENUM ('OK', 'REDIRECT', 'CLIENT_ERROR', 'SERVER_ERROR', 'TIMEOUT', 'CONNECTION_ERROR', 'UNKNOWN');
CREATE TYPE redirect_status AS ENUM ('NONE', 'TEMPORARY', 'PERMANENT', 'LOOP', 'INVALID', 'HTTPS_DOWNGRADE', 'UNKNOWN');
CREATE TYPE isp_status AS ENUM ('NOT_DETECTED', 'SUSPECTED', 'HIGH_CONFIDENCE_BLOCK', 'UNKNOWN');
CREATE TYPE tls_status AS ENUM ('VALID', 'EXPIRING', 'EXPIRED', 'HOSTNAME_MISMATCH', 'INVALID', 'ERROR', 'NOT_APPLICABLE', 'UNKNOWN');
CREATE TYPE content_status AS ENUM ('VALID_HTML', 'VALID_NON_HTML', 'EMPTY', 'TOO_SMALL', 'NOT_MEANINGFUL', 'OVERSIZED_TRUNCATED', 'UNSUPPORTED_CONTENT', 'UNKNOWN');
CREATE TYPE outbox_status AS ENUM ('pending', 'dispatching', 'dispatched', 'failed');

-- +goose StatementBegin
CREATE FUNCTION set_updated_at() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TABLE users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email citext NOT NULL UNIQUE,
    display_name text NOT NULL,
    password_hash text NOT NULL,
    locale text NOT NULL DEFAULT 'th' CHECK (locale IN ('th', 'en')),
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    oidc_subject text UNIQUE,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    CHECK (length(trim(email::text)) BETWEEN 3 AND 320),
    CHECK (length(trim(display_name)) BETWEEN 1 AND 200)
);
CREATE TRIGGER users_set_updated_at BEFORE UPDATE ON users
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE roles (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    code text NOT NULL UNIQUE CHECK (code IN ('SYSTEM', 'ADMIN', 'STAFF', 'VIEWER')),
    name_th text NOT NULL,
    name_en text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

INSERT INTO roles (code, name_th, name_en) VALUES
    ('SYSTEM', 'ระบบ', 'System'),
    ('ADMIN', 'ผู้ดูแลระบบ', 'Administrator'),
    ('STAFF', 'เจ้าหน้าที่', 'Staff'),
    ('VIEWER', 'ผู้ดูข้อมูล', 'Viewer');

CREATE TABLE user_roles (
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    role_id uuid NOT NULL REFERENCES roles(id) ON DELETE RESTRICT,
    granted_by uuid REFERENCES users(id) ON DELETE RESTRICT,
    granted_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, role_id)
);

CREATE TABLE sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    token_hash bytea NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
    csrf_token_hash bytea NOT NULL CHECK (octet_length(csrf_token_hash) = 32),
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    revoked_at timestamptz,
    CHECK (expires_at > created_at)
);
CREATE INDEX sessions_active_lookup_idx ON sessions (token_hash, expires_at) WHERE revoked_at IS NULL;
CREATE INDEX sessions_user_idx ON sessions (user_id, created_at DESC);

CREATE TABLE currencies (
    code char(3) PRIMARY KEY CHECK (code = upper(code)),
    name_th text NOT NULL,
    name_en text NOT NULL,
    decimal_places smallint NOT NULL DEFAULT 2 CHECK (decimal_places BETWEEN 0 AND 6),
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO currencies (code, name_th, name_en) VALUES
    ('THB', 'บาทไทย', 'Thai Baht'),
    ('USD', 'ดอลลาร์สหรัฐ', 'US Dollar');

CREATE TABLE registrars (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL UNIQUE,
    iana_id bigint UNIQUE CHECK (iana_id IS NULL OR iana_id > 0),
    country_code char(2),
    billing_entity text,
    tax_policy jsonb NOT NULL DEFAULT '{}'::jsonb,
    api_capability jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    CHECK (length(trim(name)) BETWEEN 1 AND 200),
    CHECK (country_code IS NULL OR country_code = upper(country_code))
);
CREATE TRIGGER registrars_set_updated_at BEFORE UPDATE ON registrars
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE domains (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    original_input text NOT NULL,
    domain_ascii text NOT NULL UNIQUE,
    domain_unicode text NOT NULL,
    registrable_domain text NOT NULL,
    registrar_id uuid REFERENCES registrars(id) ON DELETE RESTRICT,
    lifecycle_status domain_lifecycle_status NOT NULL DEFAULT 'active',
    source_status source_presence_status NOT NULL DEFAULT 'present',
    source_type text NOT NULL DEFAULT 'manual' CHECK (source_type IN ('manual', 'google_sheet', 'registrar_api', 'rdap', 'import')),
    business_priority business_priority NOT NULL DEFAULT 'medium',
    monitoring_enabled boolean NOT NULL DEFAULT true,
    monitoring_enabled_before_archive boolean NOT NULL DEFAULT true,
    expected_content_mode text NOT NULL DEFAULT 'HTML' CHECK (expected_content_mode IN ('HTML', 'ANY', 'STATUS_ONLY')),
    purchase_date date,
    expiration_at timestamptz,
    notes text NOT NULL DEFAULT '',
    current_availability_status availability_status NOT NULL DEFAULT 'UNKNOWN',
    current_dns_status dns_status NOT NULL DEFAULT 'UNKNOWN',
    current_http_status http_status NOT NULL DEFAULT 'UNKNOWN',
    current_redirect_status redirect_status NOT NULL DEFAULT 'UNKNOWN',
    current_isp_status isp_status NOT NULL DEFAULT 'UNKNOWN',
    current_tls_status tls_status NOT NULL DEFAULT 'UNKNOWN',
    current_content_status content_status NOT NULL DEFAULT 'UNKNOWN',
    current_confidence_score smallint NOT NULL DEFAULT 0 CHECK (current_confidence_score BETWEEN 0 AND 99),
    last_checked_at timestamptz,
    created_by uuid REFERENCES users(id) ON DELETE RESTRICT,
    archived_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    CHECK (domain_ascii = lower(domain_ascii)),
    CHECK (length(domain_ascii) BETWEEN 3 AND 253),
    CHECK (length(domain_unicode) BETWEEN 1 AND 253),
    CHECK (length(registrable_domain) BETWEEN 3 AND 253),
    CHECK ((lifecycle_status = 'archived') = (archived_at IS NOT NULL))
);
CREATE TRIGGER domains_set_updated_at BEFORE UPDATE ON domains
FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE INDEX domains_lifecycle_monitoring_expiry_idx ON domains (lifecycle_status, monitoring_enabled, expiration_at);
CREATE INDEX domains_current_status_checked_idx ON domains (current_availability_status, last_checked_at DESC);
CREATE INDEX domains_registrar_idx ON domains (registrar_id);
CREATE INDEX domains_registrable_idx ON domains (registrable_domain);
CREATE INDEX domains_source_status_idx ON domains (source_status, lifecycle_status);

CREATE TABLE monitor_schedules (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    domain_id uuid NOT NULL UNIQUE REFERENCES domains(id) ON DELETE RESTRICT,
    interval_seconds integer NOT NULL DEFAULT 3600 CHECK (interval_seconds IN (300, 900, 1800, 3600, 21600, 86400)),
    priority text NOT NULL DEFAULT 'normal' CHECK (priority IN ('critical', 'normal', 'low')),
    enabled boolean NOT NULL DEFAULT true,
    jitter_seconds integer NOT NULL DEFAULT 0 CHECK (jitter_seconds BETWEEN 0 AND 3600),
    next_due_at timestamptz NOT NULL DEFAULT now(),
    last_due_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0)
);
CREATE TRIGGER monitor_schedules_set_updated_at BEFORE UPDATE ON monitor_schedules
FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE INDEX monitor_schedules_due_idx ON monitor_schedules (next_due_at, priority) WHERE enabled = true;

CREATE TABLE registrar_prices (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    registrar_id uuid NOT NULL REFERENCES registrars(id) ON DELETE RESTRICT,
    tld text NOT NULL,
    price_type text NOT NULL CHECK (price_type IN ('purchase', 'renewal', 'transfer')),
    amount numeric(20,6) NOT NULL CHECK (amount >= 0),
    currency_code char(3) NOT NULL REFERENCES currencies(code) ON DELETE RESTRICT,
    price_source text NOT NULL CHECK (price_source IN ('registrar_api', 'google_sheet', 'manual', 'estimate')),
    effective_from date NOT NULL,
    effective_to date,
    source_reference text,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (tld = lower(tld)),
    CHECK (effective_to IS NULL OR effective_to >= effective_from)
);
CREATE INDEX registrar_prices_lookup_idx ON registrar_prices (registrar_id, tld, price_type, effective_from DESC);

CREATE TABLE domain_costs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    domain_id uuid NOT NULL REFERENCES domains(id) ON DELETE RESTRICT,
    cost_type text NOT NULL CHECK (cost_type IN ('purchase', 'renewal')),
    amount numeric(20,6) NOT NULL CHECK (amount >= 0),
    currency_code char(3) NOT NULL REFERENCES currencies(code) ON DELETE RESTRICT,
    price_source text NOT NULL CHECK (price_source IN ('registrar_api', 'google_sheet', 'manual', 'estimate')),
    tax_rate numeric(20,10) CHECK (tax_rate IS NULL OR tax_rate BETWEEN 0 AND 1),
    tax_inclusive boolean NOT NULL DEFAULT false,
    billing_cycle_months integer NOT NULL DEFAULT 12 CHECK (billing_cycle_months BETWEEN 1 AND 120),
    effective_from date NOT NULL,
    effective_to date,
    source_reference text,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (effective_to IS NULL OR effective_to >= effective_from)
);
CREATE INDEX domain_costs_lookup_idx ON domain_costs (domain_id, cost_type, effective_from DESC);

CREATE TABLE exchange_rates (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    base_currency char(3) NOT NULL REFERENCES currencies(code) ON DELETE RESTRICT,
    quote_currency char(3) NOT NULL REFERENCES currencies(code) ON DELETE RESTRICT,
    rate numeric(20,10) NOT NULL CHECK (rate > 0),
    source text NOT NULL,
    observed_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (base_currency, quote_currency, source, observed_at),
    CHECK (base_currency <> quote_currency)
);
CREATE INDEX exchange_rates_lookup_idx ON exchange_rates (base_currency, quote_currency, observed_at DESC);

CREATE TABLE system_settings (
    key text PRIMARY KEY,
    value jsonb NOT NULL,
    description_th text NOT NULL,
    description_en text NOT NULL,
    updated_by uuid REFERENCES users(id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    CHECK (length(trim(key)) BETWEEN 1 AND 200)
);
CREATE TRIGGER system_settings_set_updated_at BEFORE UPDATE ON system_settings
FOR EACH ROW EXECUTE FUNCTION set_updated_at();
INSERT INTO system_settings (key, value, description_th, description_en) VALUES
    ('locales.supported', '["th","en"]'::jsonb, 'ภาษาที่ระบบรองรับ', 'Supported system locales'),
    ('locales.default', '"th"'::jsonb, 'ภาษาเริ่มต้น', 'Default system locale'),
    ('domain.allow_unknown_tld', 'false'::jsonb, 'อนุญาต TLD ที่ไม่รู้จัก', 'Allow unknown TLDs'),
    ('incident.open_failure_count', '3'::jsonb, 'จำนวนครั้งที่ล้มเหลวก่อนเปิด incident', 'Failures required to open an incident'),
    ('incident.close_success_count', '2'::jsonb, 'จำนวนครั้งที่สำเร็จก่อนปิด incident', 'Successes required to close an incident');

CREATE TABLE audit_logs (
    id uuid PRIMARY KEY,
    actor_user_id uuid REFERENCES users(id) ON DELETE RESTRICT,
    action text NOT NULL,
    resource_type text NOT NULL,
    resource_id uuid,
    request_id text NOT NULL,
    reason text NOT NULL DEFAULT '',
    before_redacted jsonb,
    after_redacted jsonb,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    prev_hash bytea NOT NULL DEFAULT ''::bytea CHECK (octet_length(prev_hash) IN (0, 32)),
    entry_hash bytea NOT NULL UNIQUE CHECK (octet_length(entry_hash) = 32),
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX audit_logs_resource_idx ON audit_logs (resource_type, resource_id, created_at DESC);
CREATE INDEX audit_logs_actor_idx ON audit_logs (actor_user_id, created_at DESC);
CREATE INDEX audit_logs_request_idx ON audit_logs (request_id);

-- +goose StatementBegin
CREATE FUNCTION prevent_audit_log_mutation() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'audit_logs are append-only';
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER audit_logs_append_only
BEFORE UPDATE OR DELETE ON audit_logs
FOR EACH ROW EXECUTE FUNCTION prevent_audit_log_mutation();

CREATE TABLE manual_overrides (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    domain_id uuid NOT NULL REFERENCES domains(id) ON DELETE RESTRICT,
    field_name text NOT NULL,
    original_value jsonb,
    override_value jsonb NOT NULL,
    reason text NOT NULL,
    created_by uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    effective_from timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (length(trim(reason)) > 0),
    CHECK (expires_at IS NULL OR expires_at > effective_from)
);
CREATE INDEX manual_overrides_active_idx ON manual_overrides (domain_id, field_name, effective_from DESC) WHERE revoked_at IS NULL;

CREATE TABLE job_outbox (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    idempotency_key text NOT NULL UNIQUE,
    event_type text NOT NULL,
    stream_name text NOT NULL,
    payload jsonb NOT NULL,
    status outbox_status NOT NULL DEFAULT 'pending',
    attempts integer NOT NULL DEFAULT 0 CHECK (attempts BETWEEN 0 AND 100),
    available_at timestamptz NOT NULL DEFAULT now(),
    locked_at timestamptz,
    locked_by text,
    dispatched_at timestamptz,
    last_error_code text,
    last_error_message text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (length(trim(idempotency_key)) > 0),
    CHECK (length(trim(event_type)) > 0),
    CHECK (length(trim(stream_name)) > 0)
);
CREATE TRIGGER job_outbox_set_updated_at BEFORE UPDATE ON job_outbox
FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE INDEX job_outbox_dispatch_idx ON job_outbox (available_at, created_at) WHERE status = 'pending';
CREATE INDEX job_outbox_stale_idx ON job_outbox (locked_at) WHERE status = 'dispatching';

-- +goose Down

DROP TABLE IF EXISTS job_outbox;
DROP TABLE IF EXISTS manual_overrides;
DROP TRIGGER IF EXISTS audit_logs_append_only ON audit_logs;
DROP FUNCTION IF EXISTS prevent_audit_log_mutation();
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS system_settings;
DROP TABLE IF EXISTS exchange_rates;
DROP TABLE IF EXISTS domain_costs;
DROP TABLE IF EXISTS registrar_prices;
DROP TABLE IF EXISTS monitor_schedules;
DROP TABLE IF EXISTS domains;
DROP TABLE IF EXISTS registrars;
DROP TABLE IF EXISTS currencies;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS user_roles;
DROP TABLE IF EXISTS roles;
DROP TABLE IF EXISTS users;
DROP FUNCTION IF EXISTS set_updated_at();
DROP TYPE IF EXISTS outbox_status;
DROP TYPE IF EXISTS content_status;
DROP TYPE IF EXISTS tls_status;
DROP TYPE IF EXISTS isp_status;
DROP TYPE IF EXISTS redirect_status;
DROP TYPE IF EXISTS http_status;
DROP TYPE IF EXISTS dns_status;
DROP TYPE IF EXISTS availability_status;
DROP TYPE IF EXISTS business_priority;
DROP TYPE IF EXISTS source_presence_status;
DROP TYPE IF EXISTS domain_lifecycle_status;
