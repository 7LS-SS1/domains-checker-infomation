-- +goose Up

-- Human renewal decisions are intentionally independent of generated recommendations.
CREATE TABLE domain_renewal_decisions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    domain_id uuid NOT NULL REFERENCES domains(id) ON DELETE RESTRICT,
    decision text NOT NULL CHECK (decision IN ('RENEW', 'DO_NOT_RENEW', 'HOLD', 'UNDECIDED')),
    reason text NOT NULL DEFAULT '',
    decided_by uuid REFERENCES users(id) ON DELETE SET NULL,
    supersedes_id uuid REFERENCES domain_renewal_decisions(id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX domain_renewal_decisions_current_idx
    ON domain_renewal_decisions (domain_id, created_at DESC, id DESC);

-- +goose Down

DROP INDEX IF EXISTS domain_renewal_decisions_current_idx;
DROP TABLE IF EXISTS domain_renewal_decisions;
