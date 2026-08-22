-- +goose Up

CREATE TYPE remote_probe_job_status AS ENUM ('queued', 'leased', 'completed', 'failed', 'expired', 'cancelled');

CREATE TABLE probe_registration_tokens (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    token_hash bytea NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
    name text NOT NULL,
    region_code text NOT NULL,
    country_code char(2) NOT NULL,
    network_name text,
    expires_at timestamptz NOT NULL,
    used_at timestamptz,
    created_by uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (region_code = upper(region_code)),
    CHECK (country_code = upper(country_code)),
    CHECK (expires_at > created_at)
);
CREATE INDEX probe_registration_tokens_expiry_idx
    ON probe_registration_tokens (expires_at) WHERE used_at IS NULL;

CREATE TABLE probe_auth_challenges (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    probe_node_id uuid NOT NULL REFERENCES probe_nodes(id) ON DELETE CASCADE,
    challenge bytea NOT NULL UNIQUE CHECK (octet_length(challenge) = 32),
    expires_at timestamptz NOT NULL,
    used_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (expires_at > created_at)
);
CREATE INDEX probe_auth_challenges_expiry_idx
    ON probe_auth_challenges (probe_node_id, expires_at DESC) WHERE used_at IS NULL;

CREATE TABLE probe_access_tokens (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    probe_node_id uuid NOT NULL REFERENCES probe_nodes(id) ON DELETE CASCADE,
    token_hash bytea NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (expires_at > created_at)
);
CREATE INDEX probe_access_tokens_active_idx
    ON probe_access_tokens (probe_node_id, expires_at DESC) WHERE revoked_at IS NULL;

CREATE TABLE probe_heartbeats (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    probe_node_id uuid NOT NULL REFERENCES probe_nodes(id) ON DELETE CASCADE,
    status probe_status NOT NULL,
    protocol_version text NOT NULL,
    agent_version text NOT NULL,
    clock_offset_ms integer NOT NULL,
    queue_capacity integer NOT NULL CHECK (queue_capacity BETWEEN 0 AND 10000),
    capabilities jsonb NOT NULL DEFAULT '{}'::jsonb,
    received_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX probe_heartbeats_node_time_idx ON probe_heartbeats (probe_node_id, received_at DESC);

CREATE TABLE remote_probe_jobs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    monitoring_run_id uuid NOT NULL REFERENCES monitoring_runs(id) ON DELETE RESTRICT,
    domain_id uuid NOT NULL REFERENCES domains(id) ON DELETE RESTRICT,
    probe_node_id uuid NOT NULL REFERENCES probe_nodes(id) ON DELETE RESTRICT,
    status remote_probe_job_status NOT NULL DEFAULT 'queued',
    target jsonb NOT NULL,
    policy_version text NOT NULL,
    policy jsonb NOT NULL,
    nonce text NOT NULL UNIQUE,
    issued_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    claimed_at timestamptz,
    lease_until timestamptz,
    attempts integer NOT NULL DEFAULT 0 CHECK (attempts BETWEEN 0 AND 20),
    completed_at timestamptz,
    last_error_code text,
    last_error_message text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (monitoring_run_id, probe_node_id),
    CHECK (expires_at > issued_at),
    CHECK (lease_until IS NULL OR claimed_at IS NOT NULL),
    CHECK ((status = 'completed') = (completed_at IS NOT NULL))
);
CREATE TRIGGER remote_probe_jobs_set_updated_at BEFORE UPDATE ON remote_probe_jobs
FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE INDEX remote_probe_jobs_claim_idx
    ON remote_probe_jobs (probe_node_id, status, issued_at) WHERE status IN ('queued', 'leased');
CREATE INDEX remote_probe_jobs_run_idx ON remote_probe_jobs (monitoring_run_id);

ALTER TABLE remote_probe_results
    ADD CONSTRAINT remote_probe_results_job_fk
    FOREIGN KEY (job_id) REFERENCES remote_probe_jobs(id) ON DELETE RESTRICT;
CREATE UNIQUE INDEX remote_probe_results_nonce_idx ON remote_probe_results (probe_node_id, nonce);

-- The original Phase 2 table allowed nullable keys so architecture could land before enrollment.
-- Phase 4 enforces an Ed25519 key for every newly registered, usable probe in application code.
CREATE INDEX probe_nodes_eligible_idx
    ON probe_nodes (region_code, status, last_seen_at DESC)
    WHERE status IN ('ONLINE', 'DEGRADED') AND revoked_at IS NULL;

-- +goose Down

DROP INDEX IF EXISTS probe_nodes_eligible_idx;
DROP INDEX IF EXISTS remote_probe_results_nonce_idx;
ALTER TABLE remote_probe_results DROP CONSTRAINT IF EXISTS remote_probe_results_job_fk;
DROP TABLE IF EXISTS remote_probe_jobs;
DROP TABLE IF EXISTS probe_heartbeats;
DROP TABLE IF EXISTS probe_access_tokens;
DROP TABLE IF EXISTS probe_auth_challenges;
DROP TABLE IF EXISTS probe_registration_tokens;
DROP TYPE IF EXISTS remote_probe_job_status;
