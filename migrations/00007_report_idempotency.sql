-- +goose Up

ALTER TABLE reports ADD COLUMN idempotency_key text;
CREATE UNIQUE INDEX reports_requester_idempotency_idx ON reports (requested_by, idempotency_key) WHERE idempotency_key IS NOT NULL;

-- +goose Down

DROP INDEX IF EXISTS reports_requester_idempotency_idx;
ALTER TABLE reports DROP COLUMN IF EXISTS idempotency_key;
