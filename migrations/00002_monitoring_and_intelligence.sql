-- +goose Up

CREATE TYPE monitoring_run_status AS ENUM ('queued', 'running', 'partial', 'completed', 'failed', 'cancelled');
CREATE TYPE probe_status AS ENUM ('ONLINE', 'DEGRADED', 'OFFLINE', 'REVOKED', 'UPGRADE_REQUIRED');
CREATE TYPE confidence_level AS ENUM ('LOW', 'MEDIUM', 'HIGH');
CREATE TYPE incident_status AS ENUM ('open', 'acknowledged', 'closed');
CREATE TYPE recommendation_action AS ENUM ('RENEW', 'DROP', 'REVIEW');
CREATE TYPE opportunity_level AS ENUM ('UNKNOWN', 'LOW', 'MEDIUM', 'HIGH');
CREATE TYPE sheet_import_status AS ENUM ('preview', 'applying', 'applied', 'rejected', 'failed');
CREATE TYPE report_status AS ENUM ('queued', 'generating', 'completed', 'failed', 'expired');

CREATE TABLE probe_nodes (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL UNIQUE,
    region_code text NOT NULL,
    country_code char(2) NOT NULL,
    network_name text,
    public_asn bigint CHECK (public_asn IS NULL OR public_asn > 0),
    public_key bytea,
    status probe_status NOT NULL DEFAULT 'OFFLINE',
    version text NOT NULL DEFAULT 'unknown',
    capabilities jsonb NOT NULL DEFAULT '{}'::jsonb,
    last_seen_at timestamptz,
    clock_offset_ms integer,
    registered_at timestamptz NOT NULL DEFAULT now(),
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (region_code = upper(region_code)),
    CHECK (country_code = upper(country_code)),
    CHECK ((status = 'REVOKED') = (revoked_at IS NOT NULL))
);
CREATE TRIGGER probe_nodes_set_updated_at BEFORE UPDATE ON probe_nodes
FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE INDEX probe_nodes_health_idx ON probe_nodes (status, region_code, last_seen_at DESC);

CREATE TABLE monitoring_runs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    domain_id uuid NOT NULL REFERENCES domains(id) ON DELETE RESTRICT,
    trigger_type text NOT NULL CHECK (trigger_type IN ('scheduled', 'manual', 'import', 'recheck')),
    status monitoring_run_status NOT NULL DEFAULT 'queued',
    priority text NOT NULL DEFAULT 'normal' CHECK (priority IN ('critical', 'normal', 'low')),
    deduplication_key text NOT NULL UNIQUE,
    policy_version text NOT NULL,
    policy_snapshot jsonb NOT NULL,
    requested_by uuid REFERENCES users(id) ON DELETE RESTRICT,
    scheduled_for timestamptz NOT NULL,
    deadline_at timestamptz NOT NULL,
    started_at timestamptz,
    completed_at timestamptz,
    cancelled_at timestamptz,
    last_error_code text,
    last_error_message text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (deadline_at > scheduled_for),
    CHECK (completed_at IS NULL OR started_at IS NOT NULL)
);
CREATE TRIGGER monitoring_runs_set_updated_at BEFORE UPDATE ON monitoring_runs
FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE INDEX monitoring_runs_domain_time_idx ON monitoring_runs (domain_id, scheduled_for DESC);
CREATE INDEX monitoring_runs_status_queue_idx ON monitoring_runs (status, priority, scheduled_for) WHERE status IN ('queued', 'running', 'partial');

CREATE TABLE monitoring_results (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    monitoring_run_id uuid NOT NULL REFERENCES monitoring_runs(id) ON DELETE RESTRICT,
    domain_id uuid NOT NULL REFERENCES domains(id) ON DELETE RESTRICT,
    probe_node_id uuid REFERENCES probe_nodes(id) ON DELETE RESTRICT,
    vantage_type text NOT NULL CHECK (vantage_type IN ('local', 'remote')),
    vantage_key text NOT NULL,
    observed_availability availability_status NOT NULL DEFAULT 'UNKNOWN',
    dns_status dns_status NOT NULL DEFAULT 'UNKNOWN',
    http_status http_status NOT NULL DEFAULT 'UNKNOWN',
    redirect_status redirect_status NOT NULL DEFAULT 'UNKNOWN',
    isp_status isp_status NOT NULL DEFAULT 'UNKNOWN',
    tls_status tls_status NOT NULL DEFAULT 'UNKNOWN',
    content_status content_status NOT NULL DEFAULT 'UNKNOWN',
    initial_http_status_code integer CHECK (initial_http_status_code BETWEEN 100 AND 599),
    final_http_status_code integer CHECK (final_http_status_code BETWEEN 100 AND 599),
    final_target_status availability_status,
    failure_stage text CHECK (failure_stage IS NULL OR failure_stage IN ('normalize', 'dns', 'tcp', 'tls', 'http', 'content', 'remote', 'persistence')),
    error_code text,
    error_message text,
    confidence_score smallint NOT NULL DEFAULT 0 CHECK (confidence_score BETWEEN 0 AND 99),
    confidence_level confidence_level NOT NULL DEFAULT 'LOW',
    policy_version text NOT NULL,
    checked_at timestamptz NOT NULL,
    completed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (monitoring_run_id, vantage_key)
);
CREATE INDEX monitoring_results_domain_time_idx ON monitoring_results (domain_id, checked_at DESC);
CREATE INDEX monitoring_results_run_idx ON monitoring_results (monitoring_run_id, vantage_type);
CREATE INDEX monitoring_results_status_time_idx ON monitoring_results (observed_availability, checked_at DESC);

CREATE TABLE dns_checks (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    monitoring_result_id uuid NOT NULL REFERENCES monitoring_results(id) ON DELETE RESTRICT,
    resolver_type text NOT NULL CHECK (resolver_type IN ('LOCAL_SYSTEM', 'CLOUDFLARE_DOH', 'REMOTE_SYSTEM')),
    resolver_endpoint text NOT NULL,
    query_name text NOT NULL,
    query_type text NOT NULL CHECK (query_type IN ('A', 'AAAA', 'CNAME', 'NS')),
    attempt_no integer NOT NULL CHECK (attempt_no BETWEEN 1 AND 20),
    rcode text,
    truncated boolean NOT NULL DEFAULT false,
    authoritative boolean NOT NULL DEFAULT false,
    duration_us bigint NOT NULL CHECK (duration_us >= 0),
    error_code text,
    error_message text,
    raw_evidence jsonb NOT NULL DEFAULT '{}'::jsonb,
    checked_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (monitoring_result_id, resolver_type, query_type, attempt_no)
);
CREATE INDEX dns_checks_result_idx ON dns_checks (monitoring_result_id, resolver_type, query_type);
CREATE INDEX dns_checks_rcode_time_idx ON dns_checks (rcode, checked_at DESC);

CREATE TABLE dns_answers (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    dns_check_id uuid NOT NULL REFERENCES dns_checks(id) ON DELETE RESTRICT,
    answer_order integer NOT NULL CHECK (answer_order >= 0),
    rr_name text NOT NULL,
    rr_type text NOT NULL,
    rr_value text NOT NULL,
    ttl_seconds integer NOT NULL CHECK (ttl_seconds >= 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (dns_check_id, answer_order)
);
CREATE INDEX dns_answers_lookup_idx ON dns_answers (dns_check_id, rr_type);

CREATE TABLE http_checks (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    monitoring_result_id uuid NOT NULL REFERENCES monitoring_results(id) ON DELETE RESTRICT,
    scheme text NOT NULL CHECK (scheme IN ('https', 'http')),
    resolver_mode text NOT NULL CHECK (resolver_mode IN ('system', 'doh_pinned', 'remote_system')),
    request_url text NOT NULL,
    effective_url text,
    protocol text,
    attempt_no integer NOT NULL CHECK (attempt_no BETWEEN 1 AND 20),
    initial_status_code integer CHECK (initial_status_code BETWEEN 100 AND 599),
    final_status_code integer CHECK (final_status_code BETWEEN 100 AND 599),
    dns_duration_us bigint CHECK (dns_duration_us IS NULL OR dns_duration_us >= 0),
    connect_duration_us bigint CHECK (connect_duration_us IS NULL OR connect_duration_us >= 0),
    tls_duration_us bigint CHECK (tls_duration_us IS NULL OR tls_duration_us >= 0),
    ttfb_duration_us bigint CHECK (ttfb_duration_us IS NULL OR ttfb_duration_us >= 0),
    total_duration_us bigint NOT NULL CHECK (total_duration_us >= 0),
    content_type text,
    declared_content_length bigint,
    body_size bigint NOT NULL DEFAULT 0 CHECK (body_size >= 0),
    body_sha256 bytea CHECK (body_sha256 IS NULL OR octet_length(body_sha256) = 32),
    hash_complete boolean NOT NULL DEFAULT false,
    body_excerpt bytea,
    title text,
    content_status content_status NOT NULL DEFAULT 'UNKNOWN',
    server_header text,
    selected_headers jsonb NOT NULL DEFAULT '{}'::jsonb,
    error_code text,
    error_message text,
    checked_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (monitoring_result_id, scheme, resolver_mode, attempt_no),
    CHECK (declared_content_length IS NULL OR declared_content_length >= -1),
    CHECK (body_excerpt IS NULL OR octet_length(body_excerpt) <= 65536)
);
CREATE INDEX http_checks_result_idx ON http_checks (monitoring_result_id, scheme, resolver_mode);
CREATE INDEX http_checks_status_time_idx ON http_checks (final_status_code, checked_at DESC);

CREATE TABLE redirect_hops (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    http_check_id uuid NOT NULL REFERENCES http_checks(id) ON DELETE RESTRICT,
    hop_number integer NOT NULL CHECK (hop_number BETWEEN 0 AND 20),
    source_url text NOT NULL,
    status_code integer NOT NULL CHECK (status_code IN (301, 302, 303, 307, 308)),
    location text NOT NULL,
    resolved_target text,
    cross_domain boolean NOT NULL DEFAULT false,
    https_downgrade boolean NOT NULL DEFAULT false,
    duration_us bigint NOT NULL CHECK (duration_us >= 0),
    error_code text,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (http_check_id, hop_number)
);

CREATE TABLE tls_checks (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    http_check_id uuid NOT NULL REFERENCES http_checks(id) ON DELETE RESTRICT,
    server_name text NOT NULL,
    remote_address text,
    tls_version text,
    cipher_suite text,
    certificate_subject text,
    certificate_issuer text,
    certificate_serial_hash bytea,
    sans jsonb NOT NULL DEFAULT '[]'::jsonb,
    valid_from timestamptz,
    valid_until timestamptz,
    certificate_expiration_days integer,
    hostname_valid boolean,
    chain_valid boolean,
    tls_status tls_status NOT NULL DEFAULT 'UNKNOWN',
    diagnostic_only boolean NOT NULL DEFAULT false,
    error_code text,
    error_message text,
    checked_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX tls_checks_http_idx ON tls_checks (http_check_id, checked_at);
CREATE INDEX tls_checks_expiry_idx ON tls_checks (valid_until) WHERE valid_until IS NOT NULL;

CREATE TABLE remote_probe_results (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    monitoring_result_id uuid NOT NULL UNIQUE REFERENCES monitoring_results(id) ON DELETE RESTRICT,
    probe_node_id uuid NOT NULL REFERENCES probe_nodes(id) ON DELETE RESTRICT,
    job_id uuid NOT NULL UNIQUE,
    protocol_version text NOT NULL,
    nonce text NOT NULL,
    payload_hash bytea NOT NULL CHECK (octet_length(payload_hash) = 32),
    signature bytea NOT NULL,
    signature_valid boolean NOT NULL,
    probe_started_at timestamptz NOT NULL,
    probe_finished_at timestamptz NOT NULL,
    server_received_at timestamptz NOT NULL DEFAULT now(),
    clock_skew_ms integer,
    raw_envelope jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (probe_finished_at >= probe_started_at)
);
CREATE INDEX remote_probe_results_probe_time_idx ON remote_probe_results (probe_node_id, server_received_at DESC);

CREATE TABLE domain_status_history (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    domain_id uuid NOT NULL REFERENCES domains(id) ON DELETE RESTRICT,
    dimension text NOT NULL CHECK (dimension IN ('availability', 'dns', 'http', 'redirect', 'isp', 'tls', 'content')),
    previous_value text,
    current_value text NOT NULL,
    confidence_score smallint NOT NULL CHECK (confidence_score BETWEEN 0 AND 99),
    policy_version text NOT NULL,
    reason_codes jsonb NOT NULL DEFAULT '[]'::jsonb,
    supporting_run_ids uuid[] NOT NULL DEFAULT '{}',
    effective_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX domain_status_history_timeline_idx ON domain_status_history (domain_id, effective_at DESC);

CREATE TABLE classification_reasons (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    monitoring_run_id uuid NOT NULL REFERENCES monitoring_runs(id) ON DELETE RESTRICT,
    monitoring_result_id uuid REFERENCES monitoring_results(id) ON DELETE RESTRICT,
    dimension text NOT NULL,
    reason_code text NOT NULL,
    score_delta smallint NOT NULL DEFAULT 0,
    evidence_refs jsonb NOT NULL DEFAULT '[]'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX classification_reasons_run_idx ON classification_reasons (monitoring_run_id, dimension);

CREATE TABLE incidents (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    domain_id uuid NOT NULL REFERENCES domains(id) ON DELETE RESTRICT,
    status incident_status NOT NULL DEFAULT 'open',
    failure_stage text,
    error_code text,
    open_failure_count integer NOT NULL CHECK (open_failure_count > 0),
    close_success_count integer NOT NULL DEFAULT 0 CHECK (close_success_count >= 0),
    opened_at timestamptz NOT NULL,
    acknowledged_at timestamptz,
    acknowledged_by uuid REFERENCES users(id) ON DELETE RESTRICT,
    closed_at timestamptz,
    opened_by_run_id uuid NOT NULL REFERENCES monitoring_runs(id) ON DELETE RESTRICT,
    closed_by_run_id uuid REFERENCES monitoring_runs(id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK ((status = 'closed') = (closed_at IS NOT NULL))
);
CREATE TRIGGER incidents_set_updated_at BEFORE UPDATE ON incidents
FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE UNIQUE INDEX incidents_one_open_per_domain_idx ON incidents (domain_id) WHERE status IN ('open', 'acknowledged');
CREATE INDEX incidents_status_time_idx ON incidents (status, opened_at DESC);

CREATE TABLE incident_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    incident_id uuid NOT NULL REFERENCES incidents(id) ON DELETE RESTRICT,
    event_type text NOT NULL CHECK (event_type IN ('opened', 'failure_observed', 'success_observed', 'acknowledged', 'closed', 'note')),
    monitoring_run_id uuid REFERENCES monitoring_runs(id) ON DELETE RESTRICT,
    actor_user_id uuid REFERENCES users(id) ON DELETE RESTRICT,
    details jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX incident_events_timeline_idx ON incident_events (incident_id, created_at);

CREATE TABLE rdap_checks (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    domain_id uuid NOT NULL REFERENCES domains(id) ON DELETE RESTRICT,
    bootstrap_url text,
    rdap_url text,
    http_status integer CHECK (http_status BETWEEN 100 AND 599),
    registrar_name text,
    registrar_iana_id bigint,
    registration_at timestamptz,
    expiration_at timestamptz,
    updated_at_source timestamptz,
    nameservers jsonb NOT NULL DEFAULT '[]'::jsonb,
    dnssec boolean,
    domain_statuses jsonb NOT NULL DEFAULT '[]'::jsonb,
    source_status text NOT NULL CHECK (source_status IN ('available', 'partial', 'unavailable')),
    raw_payload_hash bytea CHECK (raw_payload_hash IS NULL OR octet_length(raw_payload_hash) = 32),
    raw_excerpt bytea,
    archive_reference text,
    error_code text,
    error_message text,
    checked_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (raw_excerpt IS NULL OR octet_length(raw_excerpt) <= 262144)
);
CREATE INDEX rdap_checks_domain_time_idx ON rdap_checks (domain_id, checked_at DESC);

CREATE TABLE recommendations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    domain_id uuid NOT NULL REFERENCES domains(id) ON DELETE RESTRICT,
    action recommendation_action NOT NULL,
    opportunity_level opportunity_level NOT NULL DEFAULT 'UNKNOWN',
    confidence_score smallint NOT NULL CHECK (confidence_score BETWEEN 0 AND 99),
    confidence_level confidence_level NOT NULL,
    policy_version text NOT NULL,
    input_snapshot jsonb NOT NULL,
    reason_codes jsonb NOT NULL,
    reasons_th jsonb NOT NULL,
    reasons_en jsonb NOT NULL,
    evidence_refs jsonb NOT NULL,
    supersedes_id uuid REFERENCES recommendations(id) ON DELETE RESTRICT,
    generated_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX recommendations_domain_time_idx ON recommendations (domain_id, generated_at DESC);
CREATE INDEX recommendations_action_time_idx ON recommendations (action, generated_at DESC);

CREATE TABLE google_sheet_imports (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    spreadsheet_id text NOT NULL,
    sheet_name text NOT NULL,
    status sheet_import_status NOT NULL DEFAULT 'preview',
    source_revision text,
    source_hash bytea CHECK (source_hash IS NULL OR octet_length(source_hash) = 32),
    column_mapping jsonb NOT NULL,
    total_rows integer NOT NULL DEFAULT 0 CHECK (total_rows >= 0),
    added_count integer NOT NULL DEFAULT 0 CHECK (added_count >= 0),
    modified_count integer NOT NULL DEFAULT 0 CHECK (modified_count >= 0),
    unchanged_count integer NOT NULL DEFAULT 0 CHECK (unchanged_count >= 0),
    missing_count integer NOT NULL DEFAULT 0 CHECK (missing_count >= 0),
    invalid_count integer NOT NULL DEFAULT 0 CHECK (invalid_count >= 0),
    requested_by uuid REFERENCES users(id) ON DELETE RESTRICT,
    previewed_at timestamptz NOT NULL DEFAULT now(),
    applied_at timestamptz,
    rejected_at timestamptz,
    error_code text,
    error_message text,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX google_sheet_imports_time_idx ON google_sheet_imports (previewed_at DESC);

CREATE TABLE google_sheet_import_rows (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    import_id uuid NOT NULL REFERENCES google_sheet_imports(id) ON DELETE RESTRICT,
    row_number integer NOT NULL CHECK (row_number > 0),
    matched_domain_id uuid REFERENCES domains(id) ON DELETE RESTRICT,
    action text NOT NULL CHECK (action IN ('ADD', 'MODIFY', 'UNCHANGED', 'MISSING', 'INVALID')),
    valid boolean NOT NULL,
    raw_values jsonb NOT NULL,
    normalized_values jsonb,
    validation_errors jsonb NOT NULL DEFAULT '[]'::jsonb,
    diff jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (import_id, row_number)
);
CREATE INDEX google_sheet_import_rows_action_idx ON google_sheet_import_rows (import_id, action, valid);

CREATE TABLE reports (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    report_type text NOT NULL,
    format text NOT NULL CHECK (format IN ('csv', 'json')),
    status report_status NOT NULL DEFAULT 'queued',
    filters jsonb NOT NULL DEFAULT '{}'::jsonb,
    snapshot_at timestamptz NOT NULL,
    policy_versions jsonb NOT NULL DEFAULT '{}'::jsonb,
    exchange_rate_snapshot jsonb NOT NULL DEFAULT '{}'::jsonb,
    completeness_warnings jsonb NOT NULL DEFAULT '[]'::jsonb,
    row_count bigint CHECK (row_count IS NULL OR row_count >= 0),
    storage_reference text,
    file_sha256 bytea CHECK (file_sha256 IS NULL OR octet_length(file_sha256) = 32),
    requested_by uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    requested_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz,
    expires_at timestamptz,
    error_code text,
    error_message text,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX reports_requester_time_idx ON reports (requested_by, requested_at DESC);
CREATE INDEX reports_status_idx ON reports (status, requested_at);

-- +goose Down

DROP TABLE IF EXISTS reports;
DROP TABLE IF EXISTS google_sheet_import_rows;
DROP TABLE IF EXISTS google_sheet_imports;
DROP TABLE IF EXISTS recommendations;
DROP TABLE IF EXISTS rdap_checks;
DROP TABLE IF EXISTS incident_events;
DROP TABLE IF EXISTS incidents;
DROP TABLE IF EXISTS classification_reasons;
DROP TABLE IF EXISTS domain_status_history;
DROP TABLE IF EXISTS remote_probe_results;
DROP TABLE IF EXISTS tls_checks;
DROP TABLE IF EXISTS redirect_hops;
DROP TABLE IF EXISTS http_checks;
DROP TABLE IF EXISTS dns_answers;
DROP TABLE IF EXISTS dns_checks;
DROP TABLE IF EXISTS monitoring_results;
DROP TABLE IF EXISTS monitoring_runs;
DROP TABLE IF EXISTS probe_nodes;
DROP TYPE IF EXISTS report_status;
DROP TYPE IF EXISTS sheet_import_status;
DROP TYPE IF EXISTS opportunity_level;
DROP TYPE IF EXISTS recommendation_action;
DROP TYPE IF EXISTS incident_status;
DROP TYPE IF EXISTS confidence_level;
DROP TYPE IF EXISTS probe_status;
DROP TYPE IF EXISTS monitoring_run_status;
