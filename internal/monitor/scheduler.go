package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type dueSchedule struct {
	ID              uuid.UUID
	DomainID        uuid.UUID
	IntervalSeconds int
	Priority        string
	JitterSeconds   int
	NextDueAt       time.Time
}

func (s *Service) ScheduleDue(ctx context.Context) (int, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, fmt.Errorf("begin schedule claim: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, `
		SELECT schedule.id, schedule.domain_id, schedule.interval_seconds, schedule.priority,
		       schedule.jitter_seconds, schedule.next_due_at
		FROM monitor_schedules schedule
		JOIN domains domain ON domain.id = schedule.domain_id
		WHERE schedule.enabled = true AND schedule.next_due_at <= now()
		  AND domain.monitoring_enabled = true AND domain.lifecycle_status = 'active'
		ORDER BY CASE schedule.priority WHEN 'critical' THEN 0 WHEN 'normal' THEN 1 ELSE 2 END,
		         schedule.next_due_at
		LIMIT $1
		FOR UPDATE OF schedule SKIP LOCKED
	`, s.cfg.SchedulerBatchSize)
	if err != nil {
		return 0, fmt.Errorf("query due schedules: %w", err)
	}
	schedules := make([]dueSchedule, 0, s.cfg.SchedulerBatchSize)
	for rows.Next() {
		var schedule dueSchedule
		if err := rows.Scan(&schedule.ID, &schedule.DomainID, &schedule.IntervalSeconds, &schedule.Priority, &schedule.JitterSeconds, &schedule.NextDueAt); err != nil {
			rows.Close()
			return 0, fmt.Errorf("scan due schedule: %w", err)
		}
		schedules = append(schedules, schedule)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, fmt.Errorf("iterate due schedules: %w", err)
	}
	rows.Close()
	policySnapshot, err := json.Marshal(s.policySnapshot())
	if err != nil {
		return 0, fmt.Errorf("marshal scheduled monitoring policy: %w", err)
	}
	now := s.now().UTC()
	created := 0
	for _, schedule := range schedules {
		slot := schedule.NextDueAt.UTC().Truncate(time.Second)
		deduplicationKey := fmt.Sprintf("monitor:%s:%s:%s:local", schedule.DomainID, slot.Format(time.RFC3339), s.cfg.MonitorPolicyVersion)
		runID := uuid.New()
		command, err := tx.Exec(ctx, `
			INSERT INTO monitoring_runs (
				id, domain_id, trigger_type, status, priority, deduplication_key, policy_version,
				policy_snapshot, scheduled_for, deadline_at
			) VALUES ($1,$2,'scheduled','queued',$3,$4,$5,$6::jsonb,$7,$8)
			ON CONFLICT (deduplication_key) DO NOTHING
		`, runID, schedule.DomainID, schedule.Priority, deduplicationKey, s.cfg.MonitorPolicyVersion,
			string(policySnapshot), slot, now.Add(s.cfg.MonitorRunTimeout))
		if err != nil {
			return created, fmt.Errorf("create scheduled monitoring run: %w", err)
		}
		if command.RowsAffected() == 1 {
			if _, err := s.outbox.EnqueueTx(ctx, tx, "outbox:"+deduplicationKey, JobEventType, s.cfg.OutboxStream, Job{RunID: runID.String()}); err != nil {
				return created, err
			}
			created++
		}
		if _, err := tx.Exec(ctx, `
			UPDATE monitor_schedules
			SET last_due_at = $2,
			    last_run_id = CASE WHEN $3 THEN $4 ELSE last_run_id END,
			    next_due_at = $5::timestamptz + (interval_seconds * interval '1 second') + (jitter_seconds * interval '1 second'),
			    version = version + 1
			WHERE id = $1
		`, schedule.ID, slot, command.RowsAffected() == 1, runID, now); err != nil {
			return created, fmt.Errorf("advance monitor schedule: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit due schedules: %w", err)
	}
	return created, nil
}
