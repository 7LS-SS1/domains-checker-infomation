-- +goose Up

ALTER TABLE domains
    ADD COLUMN consecutive_failure_count integer NOT NULL DEFAULT 0 CHECK (consecutive_failure_count >= 0),
    ADD COLUMN consecutive_success_count integer NOT NULL DEFAULT 0 CHECK (consecutive_success_count >= 0),
    ADD COLUMN current_failure_stage text CHECK (current_failure_stage IS NULL OR current_failure_stage IN ('normalize', 'dns', 'tcp', 'tls', 'http', 'content', 'remote', 'persistence')),
    ADD COLUMN current_error_code text;

ALTER TABLE monitoring_runs
    ADD COLUMN execution_attempts integer NOT NULL DEFAULT 0 CHECK (execution_attempts BETWEEN 0 AND 100),
    ADD COLUMN worker_id text,
    ADD COLUMN heartbeat_at timestamptz;

ALTER TABLE monitor_schedules
    ADD COLUMN last_run_id uuid REFERENCES monitoring_runs(id) ON DELETE RESTRICT;

INSERT INTO monitor_schedules (domain_id, enabled, jitter_seconds, next_due_at)
SELECT id, monitoring_enabled AND lifecycle_status = 'active',
       mod((('x' || substr(md5(id::text), 1, 8))::bit(32)::bigint), 60)::integer,
       now()
FROM domains
ON CONFLICT (domain_id) DO NOTHING;

CREATE INDEX monitoring_runs_recovery_idx
    ON monitoring_runs (heartbeat_at)
    WHERE status = 'running';

CREATE INDEX monitor_schedules_last_run_idx
    ON monitor_schedules (last_run_id)
    WHERE last_run_id IS NOT NULL;

-- +goose Down

DROP INDEX IF EXISTS monitor_schedules_last_run_idx;
DROP INDEX IF EXISTS monitoring_runs_recovery_idx;

ALTER TABLE monitor_schedules DROP COLUMN IF EXISTS last_run_id;

ALTER TABLE monitoring_runs
    DROP COLUMN IF EXISTS heartbeat_at,
    DROP COLUMN IF EXISTS worker_id,
    DROP COLUMN IF EXISTS execution_attempts;

ALTER TABLE domains
    DROP COLUMN IF EXISTS current_error_code,
    DROP COLUMN IF EXISTS current_failure_stage,
    DROP COLUMN IF EXISTS consecutive_success_count,
    DROP COLUMN IF EXISTS consecutive_failure_count;
