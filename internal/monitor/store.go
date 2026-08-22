package monitor

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const runColumns = `
	r.id, r.domain_id, d.domain_ascii, r.trigger_type, r.status::text, r.priority,
	r.deduplication_key, r.policy_version, r.policy_snapshot, r.requested_by,
	r.scheduled_for, r.deadline_at, r.started_at, r.completed_at,
	r.last_error_code, r.last_error_message, r.execution_attempts, r.created_at, r.updated_at`

func (s *Service) GetRun(ctx context.Context, runID uuid.UUID) (Run, error) {
	return scanRun(s.pool.QueryRow(ctx, `
		SELECT `+runColumns+`
		FROM monitoring_runs r
		JOIN domains d ON d.id = r.domain_id
		WHERE r.id = $1
	`, runID))
}

func (s *Service) ClaimExecution(ctx context.Context, runID uuid.UUID, workerID string) (Execution, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Execution{}, fmt.Errorf("begin run claim: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var execution Execution
	var heartbeatAt *time.Time
	err = tx.QueryRow(ctx, `
		SELECT `+runColumns+`,
		       d.expected_content_mode,
		       d.current_availability_status::text, d.current_dns_status::text,
		       d.current_http_status::text, d.current_redirect_status::text,
		       d.current_isp_status::text, d.current_tls_status::text, d.current_content_status::text,
		       d.current_confidence_score, d.consecutive_failure_count, d.consecutive_success_count,
		       r.heartbeat_at
		FROM monitoring_runs r
		JOIN domains d ON d.id = r.domain_id
		WHERE r.id = $1
		FOR UPDATE OF r
	`, runID).Scan(
		&execution.Run.ID, &execution.Run.DomainID, &execution.Run.DomainASCII, &execution.Run.TriggerType,
		&execution.Run.Status, &execution.Run.Priority, &execution.Run.DeduplicationKey, &execution.Run.PolicyVersion,
		&execution.Run.PolicySnapshot, &execution.Run.RequestedBy, &execution.Run.ScheduledFor, &execution.Run.DeadlineAt,
		&execution.Run.StartedAt, &execution.Run.CompletedAt, &execution.Run.LastErrorCode, &execution.Run.LastErrorMessage,
		&execution.Run.ExecutionAttempts, &execution.Run.CreatedAt, &execution.Run.UpdatedAt,
		&execution.Target.ExpectedContentMode, &execution.Target.CurrentAvailability, &execution.Target.CurrentDNS,
		&execution.Target.CurrentHTTP, &execution.Target.CurrentRedirect, &execution.Target.CurrentISP,
		&execution.Target.CurrentTLS, &execution.Target.CurrentContent, &execution.Target.CurrentConfidence,
		&execution.Target.FailureStreak, &execution.Target.SuccessStreak, &heartbeatAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Execution{}, ErrRunNotFound
		}
		return Execution{}, fmt.Errorf("load monitoring run for claim: %w", err)
	}
	execution.Target.DomainASCII = execution.Run.DomainASCII
	now := s.now().UTC()
	if execution.Run.Status == "completed" || execution.Run.Status == "cancelled" {
		return Execution{}, ErrRunCompleted
	}
	if !execution.Run.DeadlineAt.After(now) {
		if _, err := tx.Exec(ctx, `
			UPDATE monitoring_runs
			SET status = 'failed', completed_at = now(), last_error_code = 'RUN_DEADLINE_EXCEEDED',
			    last_error_message = 'Monitoring run expired before execution could complete.', worker_id = NULL, heartbeat_at = NULL
			WHERE id = $1
		`, runID); err != nil {
			return Execution{}, fmt.Errorf("expire monitoring run: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return Execution{}, fmt.Errorf("commit expired monitoring run: %w", err)
		}
		return Execution{}, ErrRunExpired
	}
	if execution.Run.Status == "running" && heartbeatAt != nil && heartbeatAt.After(now.Add(-s.cfg.MonitorQueueLease)) {
		return Execution{}, ErrRunBusy
	}
	var domainClaimed bool
	if err := tx.QueryRow(ctx, `SELECT pg_try_advisory_xact_lock(hashtextextended($1::text, 0))`, execution.Run.DomainID).Scan(&domainClaimed); err != nil {
		return Execution{}, fmt.Errorf("lock domain monitoring execution: %w", err)
	}
	if !domainClaimed {
		return Execution{}, ErrRunBusy
	}
	var anotherRunActive bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM monitoring_runs
			WHERE domain_id = $1 AND id <> $2 AND status = 'running'
			  AND heartbeat_at > now() - ($3 * interval '1 millisecond')
		)
	`, execution.Run.DomainID, runID, s.cfg.MonitorQueueLease.Milliseconds()).Scan(&anotherRunActive); err != nil {
		return Execution{}, fmt.Errorf("check active domain monitoring run: %w", err)
	}
	if anotherRunActive {
		return Execution{}, ErrRunBusy
	}
	if execution.Run.ExecutionAttempts >= s.cfg.OutboxMaxAttempts {
		if _, err := tx.Exec(ctx, `
			UPDATE monitoring_runs SET status = 'failed', completed_at = now(),
			last_error_code = 'JOB_DELIVERY_ATTEMPTS_EXHAUSTED',
			last_error_message = 'Monitoring worker delivery attempts were exhausted.' WHERE id = $1
		`, runID); err != nil {
			return Execution{}, fmt.Errorf("fail exhausted monitoring run: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return Execution{}, fmt.Errorf("commit exhausted monitoring run: %w", err)
		}
		return Execution{}, ErrRunExpired
	}
	if _, err := tx.Exec(ctx, `
		UPDATE monitoring_runs
		SET status = 'running', started_at = COALESCE(started_at, now()), worker_id = $2,
		    heartbeat_at = now(), execution_attempts = execution_attempts + 1,
		    last_error_code = NULL, last_error_message = NULL
		WHERE id = $1
	`, runID, workerID); err != nil {
		return Execution{}, fmt.Errorf("claim monitoring run: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Execution{}, fmt.Errorf("commit monitoring run claim: %w", err)
	}
	execution.Run.Status = "running"
	execution.Run.ExecutionAttempts++
	return execution, nil
}

func (s *Service) FailRun(ctx context.Context, runID uuid.UUID, code, message string) error {
	message = strings.TrimSpace(message)
	if len(message) > 1000 {
		message = message[:1000]
	}
	_, err := s.pool.Exec(ctx, `
		UPDATE monitoring_runs
		SET status = 'failed', completed_at = now(), last_error_code = $2,
		    last_error_message = $3, worker_id = NULL, heartbeat_at = NULL
		WHERE id = $1 AND status <> 'completed'
	`, runID, code, message)
	if err != nil {
		return fmt.Errorf("fail monitoring run: %w", err)
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRun(row rowScanner) (Run, error) {
	var run Run
	err := row.Scan(
		&run.ID, &run.DomainID, &run.DomainASCII, &run.TriggerType, &run.Status, &run.Priority,
		&run.DeduplicationKey, &run.PolicyVersion, &run.PolicySnapshot, &run.RequestedBy,
		&run.ScheduledFor, &run.DeadlineAt, &run.StartedAt, &run.CompletedAt,
		&run.LastErrorCode, &run.LastErrorMessage, &run.ExecutionAttempts, &run.CreatedAt, &run.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Run{}, ErrRunNotFound
		}
		return Run{}, fmt.Errorf("scan monitoring run: %w", err)
	}
	return run, nil
}
