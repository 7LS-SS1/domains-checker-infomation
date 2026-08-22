package probe

import (
	"context"
	"encoding/json"
	"fmt"

	"domainmonitor/internal/probeprotocol"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type dispatchCandidate struct {
	runID, domainID, probeID   uuid.UUID
	domainASCII, policyVersion string
}

func (s *Service) DispatchPending(ctx context.Context) (int, error) {
	now := s.now().UTC()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, fmt.Errorf("begin remote probe dispatch: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `
		UPDATE probe_nodes SET status = 'OFFLINE'
		WHERE status IN ('ONLINE','DEGRADED') AND (last_seen_at IS NULL OR last_seen_at < $1)
	`, now.Add(-s.cfg.ProbeStaleAfter)); err != nil {
		return 0, fmt.Errorf("mark stale probes offline: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE remote_probe_jobs SET status = 'expired', last_error_code = 'PROBE_JOB_EXPIRED'
		WHERE status IN ('queued','leased') AND expires_at <= $1
	`, now); err != nil {
		return 0, fmt.Errorf("expire remote probe jobs: %w", err)
	}
	rows, err := tx.Query(ctx, `
		WITH selected_probe AS (
			SELECT id FROM probe_nodes
			WHERE region_code = $1 AND status = 'ONLINE' AND revoked_at IS NULL AND last_seen_at >= $2
			ORDER BY last_seen_at DESC, id LIMIT 1
		)
		SELECT run.id, run.domain_id, selected_probe.id, domain.domain_ascii, run.policy_version
		FROM monitoring_runs run
		JOIN domains domain ON domain.id = run.domain_id
		CROSS JOIN selected_probe
		WHERE run.status = 'completed' AND run.completed_at >= $3
		  AND EXISTS (
			SELECT 1 FROM monitoring_results local_result
			WHERE local_result.monitoring_run_id = run.id AND local_result.vantage_type = 'local'
		  )
		  AND NOT EXISTS (
			SELECT 1 FROM remote_probe_jobs job
			WHERE job.monitoring_run_id = run.id AND job.probe_node_id = selected_probe.id
		  )
		ORDER BY run.completed_at
		LIMIT $4
		FOR UPDATE OF run SKIP LOCKED
	`, s.cfg.ProbeRequiredRegion, now.Add(-s.cfg.ProbeStaleAfter), now.Add(-s.cfg.ProbeEvidenceFresh), s.cfg.ProbeDispatchBatch)
	if err != nil {
		return 0, fmt.Errorf("select remote probe candidates: %w", err)
	}
	candidates := []dispatchCandidate{}
	for rows.Next() {
		var item dispatchCandidate
		if err := rows.Scan(&item.runID, &item.domainID, &item.probeID, &item.domainASCII, &item.policyVersion); err != nil {
			rows.Close()
			return 0, fmt.Errorf("scan remote probe candidate: %w", err)
		}
		candidates = append(candidates, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()
	policy := probeprotocol.Policy{
		DeadlineMS: s.cfg.HTTPChainTimeout.Milliseconds(), MaxRedirects: s.cfg.HTTPMaxRedirects,
		MaxBodyBytes: s.cfg.HTTPMaxBodyBytes, StoreExcerptBytes: s.cfg.HTTPExcerptBytes,
	}
	policyJSON, _ := json.Marshal(policy)
	count := 0
	for _, candidate := range candidates {
		target := probeprotocol.Target{DomainASCII: candidate.domainASCII, Schemes: []string{"https", "http"}, Ports: []int{443, 80}}
		targetJSON, _ := json.Marshal(target)
		jobID := uuid.New()
		nonce, _, err := probeprotocol.NewSecret()
		if err != nil {
			return count, err
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO remote_probe_jobs
			(id, monitoring_run_id, domain_id, probe_node_id, status, target, policy_version, policy,
			 nonce, issued_at, expires_at)
			VALUES ($1,$2,$3,$4,'queued',$5::jsonb,$6,$7::jsonb,$8,$9,$10)
		`, jobID, candidate.runID, candidate.domainID, candidate.probeID, string(targetJSON), candidate.policyVersion,
			string(policyJSON), nonce, now, now.Add(s.cfg.ProbeJobTTL))
		if err != nil {
			return count, fmt.Errorf("insert remote probe job: %w", err)
		}
		count++
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit remote probe dispatch: %w", err)
	}
	return count, nil
}

func (s *Service) Claim(ctx context.Context, principal Principal) (probeprotocol.Job, error) {
	if principal.Status != "ONLINE" {
		return probeprotocol.Job{}, ErrForbidden
	}
	now := s.now().UTC()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return probeprotocol.Job{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `
		UPDATE remote_probe_jobs SET status = 'queued', claimed_at = NULL, lease_until = NULL
		WHERE probe_node_id = $1 AND status = 'leased' AND lease_until <= $2 AND expires_at > $2 AND attempts < 20
	`, principal.ProbeID, now); err != nil {
		return probeprotocol.Job{}, fmt.Errorf("requeue expired remote lease: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE remote_probe_jobs SET status = 'expired', last_error_code = 'PROBE_JOB_EXPIRED'
		WHERE probe_node_id = $1 AND status IN ('queued','leased') AND expires_at <= $2
	`, principal.ProbeID, now); err != nil {
		return probeprotocol.Job{}, fmt.Errorf("expire remote jobs before claim: %w", err)
	}
	var job probeprotocol.Job
	var targetJSON, policyJSON []byte
	err = tx.QueryRow(ctx, `
		SELECT id, monitoring_run_id, target, policy_version, policy, issued_at, expires_at, nonce
		FROM remote_probe_jobs
		WHERE probe_node_id = $1 AND status = 'queued' AND expires_at > $2
		ORDER BY issued_at
		LIMIT 1 FOR UPDATE SKIP LOCKED
	`, principal.ProbeID, now).Scan(&job.JobID, &job.RunID, &targetJSON, &job.PolicyVersion, &policyJSON, &job.IssuedAt, &job.ExpiresAt, &job.Nonce)
	if err == pgx.ErrNoRows {
		return probeprotocol.Job{}, ErrNoJob
	}
	if err != nil {
		return probeprotocol.Job{}, fmt.Errorf("claim remote probe job: %w", err)
	}
	if err := json.Unmarshal(targetJSON, &job.Target); err != nil {
		return probeprotocol.Job{}, fmt.Errorf("decode remote target: %w", err)
	}
	if err := json.Unmarshal(policyJSON, &job.Policy); err != nil {
		return probeprotocol.Job{}, fmt.Errorf("decode remote policy: %w", err)
	}
	leaseUntil := now.Add(s.cfg.ProbeJobLease)
	if leaseUntil.After(job.ExpiresAt) {
		leaseUntil = job.ExpiresAt
	}
	command, err := tx.Exec(ctx, `
		UPDATE remote_probe_jobs SET status = 'leased', claimed_at = $2, lease_until = $3, attempts = attempts + 1
		WHERE id = $1 AND status = 'queued'
	`, job.JobID, now, leaseUntil)
	if err != nil {
		return probeprotocol.Job{}, fmt.Errorf("lease remote probe job: %w", err)
	}
	if command.RowsAffected() != 1 {
		return probeprotocol.Job{}, ErrConflict
	}
	if err := tx.Commit(ctx); err != nil {
		return probeprotocol.Job{}, err
	}
	return job, nil
}
