package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"domainmonitor/internal/audit"
	"domainmonitor/internal/config"
	"domainmonitor/internal/queue"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	pool   *pgxpool.Pool
	outbox *queue.Store
	audit  *audit.Store
	cfg    config.Config
	now    func() time.Time
}

func NewService(pool *pgxpool.Pool, auditStore *audit.Store, cfg config.Config) *Service {
	return &Service{pool: pool, outbox: queue.NewStore(pool), audit: auditStore, cfg: cfg, now: time.Now}
}

func (s *Service) CreateManualRun(ctx context.Context, domainID, userID uuid.UUID, requestID, idempotencyKey string) (Run, bool, error) {
	return s.createManualRun(ctx, domainID, userID, requestID, idempotencyKey, "DOMAIN_MANUAL_CHECK_REQUESTED")
}

// CreateManualISPCheckRun queues the same fresh local monitoring run as
// CreateManualRun, tagged with a distinct audit action so operators can tell
// a forced ISP re-check apart from a generic manual recheck. Remote-probe
// dispatch itself needs no separate trigger here: the worker calls
// probe.Service.DispatchPending immediately after this run completes, and
// DispatchPending has no local-availability gate — any completed run with an
// ONLINE probe in the required region gets a fresh remote-probe job, which is
// what ultimately refreshes the domain's ISP classification.
func (s *Service) CreateManualISPCheckRun(ctx context.Context, domainID, userID uuid.UUID, requestID, idempotencyKey string) (Run, bool, error) {
	return s.createManualRun(ctx, domainID, userID, requestID, idempotencyKey, "DOMAIN_ISP_CHECK_FORCED")
}

func (s *Service) createManualRun(ctx context.Context, domainID, userID uuid.UUID, requestID, idempotencyKey, auditAction string) (Run, bool, error) {
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" || len(idempotencyKey) > 200 {
		return Run{}, false, ErrInvalidIdempotencyKey
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Run{}, false, fmt.Errorf("begin manual monitoring transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var domainASCII, lifecycle string
	if err := tx.QueryRow(ctx, `SELECT domain_ascii, lifecycle_status::text FROM domains WHERE id = $1 FOR UPDATE`, domainID).Scan(&domainASCII, &lifecycle); err != nil {
		if err == pgx.ErrNoRows {
			return Run{}, false, ErrDomainNotFound
		}
		return Run{}, false, fmt.Errorf("load domain for manual run: %w", err)
	}
	if lifecycle != "active" {
		return Run{}, false, ErrDomainInactive
	}
	now := s.now().UTC()
	deduplicationKey := fmt.Sprintf("manual:%s:%s:%s", domainID, userID, idempotencyKey)
	policySnapshot, err := json.Marshal(s.policySnapshot())
	if err != nil {
		return Run{}, false, fmt.Errorf("marshal monitoring policy: %w", err)
	}
	runID := uuid.New()
	command, err := tx.Exec(ctx, `
		INSERT INTO monitoring_runs (
			id, domain_id, trigger_type, status, priority, deduplication_key, policy_version,
			policy_snapshot, requested_by, scheduled_for, deadline_at
		) VALUES ($1,$2,'manual','queued','critical',$3,$4,$5::jsonb,$6,$7,$8)
		ON CONFLICT (deduplication_key) DO NOTHING
	`, runID, domainID, deduplicationKey, s.cfg.MonitorPolicyVersion, string(policySnapshot), userID, now, now.Add(s.cfg.MonitorRunTimeout))
	if err != nil {
		return Run{}, false, fmt.Errorf("create manual monitoring run: %w", err)
	}
	created := command.RowsAffected() == 1
	if !created {
		if err := tx.QueryRow(ctx, `SELECT id FROM monitoring_runs WHERE deduplication_key = $1`, deduplicationKey).Scan(&runID); err != nil {
			return Run{}, false, fmt.Errorf("load idempotent monitoring run: %w", err)
		}
	} else {
		if _, err := s.outbox.EnqueueTx(ctx, tx, "outbox:"+deduplicationKey, JobEventType, s.cfg.OutboxStream, Job{RunID: runID.String()}); err != nil {
			return Run{}, false, err
		}
		if err := s.audit.AppendTx(ctx, tx, audit.Entry{
			ActorUserID: &userID, Action: auditAction, ResourceType: "monitoring_run",
			ResourceID: &runID, RequestID: requestID, Metadata: map[string]any{"domain_id": domainID, "domain": domainASCII},
		}); err != nil {
			return Run{}, false, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Run{}, false, fmt.Errorf("commit manual monitoring run: %w", err)
	}
	run, err := s.GetRun(ctx, runID)
	return run, created, err
}

func (s *Service) policySnapshot() map[string]any {
	return map[string]any{
		"version":                  s.cfg.MonitorPolicyVersion,
		"run_timeout_ms":           s.cfg.MonitorRunTimeout.Milliseconds(),
		"dns_timeout_ms":           s.cfg.DNSAttemptTimeout.Milliseconds(),
		"http_chain_timeout_ms":    s.cfg.HTTPChainTimeout.Milliseconds(),
		"max_attempts":             s.cfg.MonitorMaxAttempts,
		"max_redirects":            s.cfg.HTTPMaxRedirects,
		"max_body_bytes":           s.cfg.HTTPMaxBodyBytes,
		"excerpt_bytes":            s.cfg.HTTPExcerptBytes,
		"incident_open_failures":   s.cfg.IncidentOpenFailures,
		"incident_close_successes": s.cfg.IncidentCloseSuccess,
	}
}
