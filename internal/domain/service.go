package domain

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"domainmonitor/internal/audit"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	pool       *pgxpool.Pool
	store      *Store
	audit      *audit.Store
	normalizer Normalizer
}

func NewService(pool *pgxpool.Pool, store *Store, auditStore *audit.Store, normalizer Normalizer) *Service {
	return &Service{pool: pool, store: store, audit: auditStore, normalizer: normalizer}
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (Domain, error) {
	return s.store.Get(ctx, id)
}

func (s *Service) Provenance(ctx context.Context, id uuid.UUID) ([]Provenance, error) {
	return s.store.ListProvenance(ctx, id)
}

func (s *Service) List(ctx context.Context, filter ListFilter) (Page, error) {
	if filter.LifecycleStatus != "" {
		switch filter.LifecycleStatus {
		case "active", "inactive", "archived":
		default:
			return Page{}, &ValidationError{Field: "lifecycle_status", ReasonCode: "INVALID_LIFECYCLE_STATUS", Reason: "must be active, inactive, or archived"}
		}
	}
	if filter.SourceStatus != "" {
		switch filter.SourceStatus {
		case "present", "missing_from_source", "unknown":
		default:
			return Page{}, &ValidationError{Field: "source_status", ReasonCode: "INVALID_SOURCE_STATUS", Reason: "must be present, missing_from_source, or unknown"}
		}
	}
	if len(filter.Query) > 253 {
		return Page{}, &ValidationError{Field: "query", ReasonCode: "VALUE_TOO_LONG", Reason: "must not exceed 253 characters"}
	}
	return s.store.List(ctx, filter)
}

func (s *Service) Create(ctx context.Context, actor Actor, input CreateInput) (Domain, error) {
	normalized, err := s.normalizer.Normalize(input.Domain)
	if err != nil {
		return Domain{}, err
	}
	priority, err := validatePriority(input.BusinessPriority)
	if err != nil {
		return Domain{}, err
	}
	contentMode, err := validateContentMode(input.ExpectedContentMode)
	if err != nil {
		return Domain{}, err
	}
	item := Domain{
		ID:                  uuid.New(),
		OriginalInput:       normalized.OriginalInput,
		ASCII:               normalized.ASCII,
		Unicode:             normalized.Unicode,
		RegistrableDomain:   normalized.RegistrableDomain,
		RegistrarID:         input.RegistrarID,
		BusinessPriority:    priority,
		MonitoringEnabled:   input.MonitoringEnabled,
		ExpectedContentMode: contentMode,
		ExpirationAt:        input.ExpirationAt,
		Notes:               strings.TrimSpace(input.Notes),
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Domain{}, fmt.Errorf("begin create domain transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	created, err := s.store.InsertTx(ctx, tx, item, actor.UserID)
	if err != nil {
		return Domain{}, err
	}
	if err := s.store.EnsureScheduleTx(ctx, tx, created.ID, created.MonitoringEnabled); err != nil {
		return Domain{}, err
	}
	if err := s.audit.AppendTx(ctx, tx, audit.Entry{
		ActorUserID:  &actor.UserID,
		Action:       "DOMAIN_ADDED",
		ResourceType: "domain",
		ResourceID:   &created.ID,
		RequestID:    actor.RequestID,
		After:        created,
	}); err != nil {
		return Domain{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Domain{}, fmt.Errorf("commit create domain transaction: %w", err)
	}
	return created, nil
}

func (s *Service) Patch(ctx context.Context, actor Actor, id uuid.UUID, input PatchInput) (Domain, error) {
	if input.Version < 1 {
		return Domain{}, &ValidationError{Field: "version", ReasonCode: "VALUE_MUST_BE_POSITIVE", Reason: "must be positive"}
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Domain{}, fmt.Errorf("begin update domain transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	before, err := s.store.GetTx(ctx, tx, id)
	if err != nil {
		return Domain{}, err
	}
	if before.Version != input.Version {
		return Domain{}, ErrConflict
	}
	after := before
	if input.Domain != nil {
		normalized, err := s.normalizer.Normalize(*input.Domain)
		if err != nil {
			return Domain{}, err
		}
		after.OriginalInput = normalized.OriginalInput
		after.ASCII = normalized.ASCII
		after.Unicode = normalized.Unicode
		after.RegistrableDomain = normalized.RegistrableDomain
	}
	if input.ClearRegistrar {
		after.RegistrarID = nil
	} else if input.RegistrarID != nil {
		after.RegistrarID = input.RegistrarID
	}
	if input.BusinessPriority != nil {
		after.BusinessPriority, err = validatePriority(*input.BusinessPriority)
		if err != nil {
			return Domain{}, err
		}
	}
	if input.MonitoringEnabled != nil {
		after.MonitoringEnabled = *input.MonitoringEnabled
	}
	if input.ExpectedContentMode != nil {
		after.ExpectedContentMode, err = validateContentMode(*input.ExpectedContentMode)
		if err != nil {
			return Domain{}, err
		}
	}
	if input.ClearExpiration {
		after.ExpirationAt = nil
	} else if input.ExpirationAt != nil {
		after.ExpirationAt = input.ExpirationAt
	}
	if input.Notes != nil {
		after.Notes = strings.TrimSpace(*input.Notes)
	}
	renewalDecisionChanged := false
	if input.RenewalDecision != nil {
		if err := validateRenewalDecision(*input.RenewalDecision); err != nil {
			return Domain{}, err
		}
		renewalDecisionChanged = strings.ToUpper(strings.TrimSpace(*input.RenewalDecision)) != before.RenewalDecision
	}
	updated, err := s.store.UpdateTx(ctx, tx, after, input.Version)
	if err != nil {
		return Domain{}, err
	}
	if renewalDecisionChanged {
		if err := s.store.InsertRenewalDecisionTx(ctx, tx, updated.ID, *input.RenewalDecision, input.Reason, actor.UserID); err != nil {
			return Domain{}, err
		}
		updated, err = s.store.GetTx(ctx, tx, updated.ID)
		if err != nil {
			return Domain{}, err
		}
	}
	if err := s.store.EnsureScheduleTx(ctx, tx, updated.ID, updated.MonitoringEnabled && updated.LifecycleStatus == "active"); err != nil {
		return Domain{}, err
	}
	if err := s.audit.AppendTx(ctx, tx, audit.Entry{
		ActorUserID:  &actor.UserID,
		Action:       "DOMAIN_UPDATED",
		ResourceType: "domain",
		ResourceID:   &updated.ID,
		RequestID:    actor.RequestID,
		Reason:       strings.TrimSpace(input.Reason),
		Before:       before,
		After:        updated,
	}); err != nil {
		return Domain{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Domain{}, fmt.Errorf("commit update domain transaction: %w", err)
	}
	return updated, nil
}

func (s *Service) Archive(ctx context.Context, actor Actor, id uuid.UUID, version int64, reason string) (Domain, error) {
	if strings.TrimSpace(reason) == "" {
		return Domain{}, &ValidationError{Field: "reason", ReasonCode: "VALUE_REQUIRED", Reason: "is required"}
	}
	return s.changeLifecycle(ctx, actor, id, version, reason, "DOMAIN_ARCHIVED", s.store.ArchiveTx)
}

func (s *Service) Restore(ctx context.Context, actor Actor, id uuid.UUID, version int64, reason string) (Domain, error) {
	if strings.TrimSpace(reason) == "" {
		return Domain{}, &ValidationError{Field: "reason", ReasonCode: "VALUE_REQUIRED", Reason: "is required"}
	}
	return s.changeLifecycle(ctx, actor, id, version, reason, "DOMAIN_RESTORED", s.store.RestoreTx)
}

func (s *Service) changeLifecycle(
	ctx context.Context,
	actor Actor,
	id uuid.UUID,
	version int64,
	reason string,
	action string,
	change func(context.Context, pgx.Tx, uuid.UUID, int64) (Domain, error),
) (Domain, error) {
	if version < 1 {
		return Domain{}, &ValidationError{Field: "version", ReasonCode: "VALUE_MUST_BE_POSITIVE", Reason: "must be positive"}
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Domain{}, fmt.Errorf("begin lifecycle transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	before, err := s.store.GetTx(ctx, tx, id)
	if err != nil {
		return Domain{}, err
	}
	if before.Version != version {
		return Domain{}, ErrConflict
	}
	after, err := change(ctx, tx, id, version)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return Domain{}, ErrConflict
		}
		return Domain{}, err
	}
	if err := s.store.EnsureScheduleTx(ctx, tx, after.ID, after.MonitoringEnabled && after.LifecycleStatus == "active"); err != nil {
		return Domain{}, err
	}
	if err := s.audit.AppendTx(ctx, tx, audit.Entry{
		ActorUserID:  &actor.UserID,
		Action:       action,
		ResourceType: "domain",
		ResourceID:   &id,
		RequestID:    actor.RequestID,
		Reason:       strings.TrimSpace(reason),
		Before:       before,
		After:        after,
	}); err != nil {
		return Domain{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Domain{}, fmt.Errorf("commit lifecycle transaction: %w", err)
	}
	return after, nil
}

func validatePriority(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "medium", nil
	}
	switch value {
	case "low", "medium", "high", "critical":
		return value, nil
	default:
		return "", &ValidationError{Field: "business_priority", ReasonCode: "INVALID_BUSINESS_PRIORITY", Reason: "must be low, medium, high, or critical"}
	}
}

func validateContentMode(value string) (string, error) {
	value = strings.ToUpper(strings.TrimSpace(value))
	if value == "" {
		return "HTML", nil
	}
	switch value {
	case "HTML", "ANY", "STATUS_ONLY":
		return value, nil
	default:
		return "", &ValidationError{Field: "expected_content_mode", ReasonCode: "INVALID_CONTENT_MODE", Reason: "must be HTML, ANY, or STATUS_ONLY"}
	}
}

func validateRenewalDecision(value string) error {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "RENEW", "DO_NOT_RENEW", "HOLD", "UNDECIDED":
		return nil
	default:
		return &ValidationError{Field: "renewal_decision", ReasonCode: "INVALID_RENEWAL_DECISION", Reason: "must be RENEW, DO_NOT_RENEW, HOLD, or UNDECIDED"}
	}
}
