package recommendation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"domainmonitor/internal/audit"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound   = errors.New("recommendation not found")
	ErrValidation = errors.New("recommendation validation failed")
)

type Actor struct {
	UserID    uuid.UUID
	RequestID string
}

type Override struct {
	ID        uuid.UUID `json:"id"`
	Action    string    `json:"action"`
	Reason    string    `json:"reason"`
	CreatedBy uuid.UUID `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}

type Record struct {
	ID               uuid.UUID       `json:"id"`
	DomainID         uuid.UUID       `json:"domain_id"`
	Domain           string          `json:"domain"`
	Action           string          `json:"action"`
	EffectiveAction  string          `json:"effective_action"`
	OpportunityLevel string          `json:"opportunity_level"`
	ConfidenceScore  int             `json:"confidence_score"`
	ConfidenceLevel  string          `json:"confidence_level"`
	PolicyVersion    string          `json:"policy_version"`
	InputSnapshot    json.RawMessage `json:"input_snapshot"`
	ReasonCodes      []string        `json:"reason_codes"`
	ReasonsTH        []string        `json:"reasons_th"`
	ReasonsEN        []string        `json:"reasons_en"`
	EvidenceRefs     []string        `json:"evidence_refs"`
	SupersedesID     *uuid.UUID      `json:"supersedes_id,omitempty"`
	GeneratedAt      time.Time       `json:"generated_at"`
	ManualOverride   *Override       `json:"manual_override,omitempty"`
}

type Service struct {
	pool  *pgxpool.Pool
	audit *audit.Store
	now   func() time.Time
}

func NewService(pool *pgxpool.Pool, auditStore *audit.Store) *Service {
	return &Service{pool: pool, audit: auditStore, now: time.Now}
}

func (s *Service) Generate(ctx context.Context, actor Actor, domainID uuid.UUID) (Record, error) {
	inputs, err := s.loadInputs(ctx, &domainID, 1)
	if err != nil {
		return Record{}, err
	}
	if len(inputs) == 0 {
		return Record{}, ErrNotFound
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Record{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = s.persist(ctx, tx, actor, inputs[0])
	if err != nil {
		return Record{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Record{}, err
	}
	return s.GetLatest(ctx, domainID)
}

func (s *Service) Run(ctx context.Context, actor Actor, limit int) ([]Record, error) {
	if limit == 0 {
		limit = 500
	}
	if limit < 1 || limit > 1000 {
		return nil, fmt.Errorf("%w: limit", ErrValidation)
	}
	inputs, err := s.loadInputs(ctx, nil, limit)
	if err != nil {
		return nil, err
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	items := make([]Record, 0, len(inputs))
	for _, input := range inputs {
		record, persistErr := s.persist(ctx, tx, actor, input)
		if persistErr != nil {
			return nil, persistErr
		}
		items = append(items, record)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Service) persist(ctx context.Context, tx pgx.Tx, actor Actor, input Input) (Record, error) {
	result := Evaluate(input, s.now().UTC())
	domainID, err := uuid.Parse(input.DomainID)
	if err != nil {
		return Record{}, err
	}
	var supersedes *uuid.UUID
	err = tx.QueryRow(ctx, `SELECT id FROM recommendations WHERE domain_id=$1 ORDER BY generated_at DESC,created_at DESC LIMIT 1 FOR UPDATE`, domainID).Scan(&supersedes)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return Record{}, err
	}
	inputJSON, _ := json.Marshal(input)
	reasonCodes, _ := json.Marshal(result.ReasonCodes)
	reasonsTH, _ := json.Marshal(result.ReasonsTH)
	reasonsEN, _ := json.Marshal(result.ReasonsEN)
	evidence, _ := json.Marshal(result.EvidenceRefs)
	record := Record{ID: uuid.New(), DomainID: domainID, Domain: input.Domain, Action: result.Action, EffectiveAction: result.Action, OpportunityLevel: result.OpportunityLevel, ConfidenceScore: result.ConfidenceScore, ConfidenceLevel: result.ConfidenceLevel, PolicyVersion: result.PolicyVersion, InputSnapshot: inputJSON, ReasonCodes: result.ReasonCodes, ReasonsTH: result.ReasonsTH, ReasonsEN: result.ReasonsEN, EvidenceRefs: result.EvidenceRefs, SupersedesID: supersedes, GeneratedAt: s.now().UTC()}
	_, err = tx.Exec(ctx, `INSERT INTO recommendations (id,domain_id,action,opportunity_level,confidence_score,confidence_level,policy_version,input_snapshot,reason_codes,reasons_th,reasons_en,evidence_refs,supersedes_id,generated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8::jsonb,$9::jsonb,$10::jsonb,$11::jsonb,$12::jsonb,$13,$14)`, record.ID, record.DomainID, record.Action, record.OpportunityLevel, record.ConfidenceScore, record.ConfidenceLevel, record.PolicyVersion, inputJSON, reasonCodes, reasonsTH, reasonsEN, evidence, supersedes, record.GeneratedAt)
	if err != nil {
		return Record{}, err
	}
	if err := s.audit.AppendTx(ctx, tx, audit.Entry{ActorUserID: &actor.UserID, Action: "RECOMMENDATION_GENERATED", ResourceType: "recommendation", ResourceID: &record.ID, RequestID: actor.RequestID, After: map[string]any{"domain_id": domainID, "action": record.Action, "opportunity_level": record.OpportunityLevel, "confidence_score": record.ConfidenceScore, "policy_version": record.PolicyVersion}, Metadata: map[string]any{"reason_codes": record.ReasonCodes}}); err != nil {
		return Record{}, err
	}
	return record, nil
}

func (s *Service) GetLatest(ctx context.Context, domainID uuid.UUID) (Record, error) {
	return scanRecord(s.pool.QueryRow(ctx, recommendationSelect+` WHERE r.domain_id=$1 ORDER BY r.generated_at DESC,r.created_at DESC LIMIT 1`, domainID))
}

func (s *Service) List(ctx context.Context, action string, limit int) ([]Record, error) {
	action = strings.ToUpper(strings.TrimSpace(action))
	if action != "" && action != "RENEW" && action != "DROP" && action != "REVIEW" && action != "PROFIT_OPPORTUNITY" {
		return nil, fmt.Errorf("%w: action", ErrValidation)
	}
	if limit == 0 {
		limit = 100
	}
	if limit < 1 || limit > 500 {
		return nil, fmt.Errorf("%w: limit", ErrValidation)
	}
	rows, err := s.pool.Query(ctx, recommendationSelect+` WHERE ($1='' OR r.action::text=$1) ORDER BY r.generated_at DESC,r.created_at DESC LIMIT $2`, action, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Record{}
	for rows.Next() {
		item, scanErr := scanRecord(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

const recommendationSelect = `
	SELECT r.id,r.domain_id,d.domain_ascii,r.action::text,r.opportunity_level::text,r.confidence_score,r.confidence_level::text,r.policy_version,r.input_snapshot,r.reason_codes,r.reasons_th,r.reasons_en,r.evidence_refs,r.supersedes_id,r.generated_at,
	       o.id,o.override_value #>> '{}',o.reason,o.created_by,o.effective_from
	FROM recommendations r JOIN domains d ON d.id=r.domain_id
	LEFT JOIN LATERAL (
		SELECT id,override_value,reason,created_by,effective_from FROM manual_overrides
		WHERE domain_id=r.domain_id AND field_name='recommendation' AND revoked_at IS NULL AND effective_from<=now() AND (expires_at IS NULL OR expires_at>now())
		ORDER BY effective_from DESC LIMIT 1
	) o ON true`

func scanRecord(row pgx.Row) (Record, error) {
	var result Record
	var reasonCodes, reasonsTH, reasonsEN, evidence []byte
	var overrideID, overrideUser *uuid.UUID
	var overrideAction, overrideReason *string
	var overrideAt *time.Time
	err := row.Scan(&result.ID, &result.DomainID, &result.Domain, &result.Action, &result.OpportunityLevel, &result.ConfidenceScore, &result.ConfidenceLevel, &result.PolicyVersion, &result.InputSnapshot, &reasonCodes, &reasonsTH, &reasonsEN, &evidence, &result.SupersedesID, &result.GeneratedAt, &overrideID, &overrideAction, &overrideReason, &overrideUser, &overrideAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Record{}, ErrNotFound
	}
	if err != nil {
		return Record{}, err
	}
	_ = json.Unmarshal(reasonCodes, &result.ReasonCodes)
	_ = json.Unmarshal(reasonsTH, &result.ReasonsTH)
	_ = json.Unmarshal(reasonsEN, &result.ReasonsEN)
	_ = json.Unmarshal(evidence, &result.EvidenceRefs)
	result.EffectiveAction = result.Action
	if overrideID != nil && overrideAction != nil && overrideReason != nil && overrideUser != nil && overrideAt != nil {
		result.EffectiveAction = *overrideAction
		result.ManualOverride = &Override{ID: *overrideID, Action: *overrideAction, Reason: *overrideReason, CreatedBy: *overrideUser, CreatedAt: *overrideAt}
	}
	return result, nil
}

func (s *Service) loadInputs(ctx context.Context, domainID *uuid.UUID, limit int) ([]Input, error) {
	rows, err := s.pool.Query(ctx, `
		WITH incident_stats AS (
			SELECT domain_id,count(*) FILTER (WHERE status IN ('open','acknowledged'))::int open_count,count(*) FILTER (WHERE opened_at>=now()-interval '90 days')::int count_90
			FROM incidents GROUP BY domain_id
		), downtime AS (
			SELECT domain_id,count(*)::int changes FROM domain_status_history WHERE dimension='availability' AND current_value='UNAVAILABLE' AND effective_at>=now()-interval '90 days' GROUP BY domain_id
		)
		SELECT d.id::text,d.domain_ascii,d.lifecycle_status::text,d.source_status::text,d.business_priority::text,d.monitoring_enabled,
		       d.current_availability_status::text,d.current_dns_status::text,d.current_http_status::text,d.current_redirect_status::text,d.current_isp_status::text,d.current_tls_status::text,
		       d.current_confidence_score,d.last_checked_at,d.expiration_at,COALESCE(cost.amount::text,''),COALESCE(cost.currency_code::text,''),COALESCE(i.open_count,0),COALESCE(i.count_90,0),COALESCE(dt.changes,0)
		FROM domains d
		LEFT JOIN incident_stats i ON i.domain_id=d.id LEFT JOIN downtime dt ON dt.domain_id=d.id
		LEFT JOIN LATERAL (
			SELECT amount,currency_code FROM domain_costs WHERE domain_id=d.id AND cost_type='renewal' AND effective_from<=current_date AND (effective_to IS NULL OR effective_to>=current_date)
			ORDER BY CASE price_source WHEN 'registrar_api' THEN 1 WHEN 'google_sheet' THEN 2 WHEN 'manual' THEN 3 ELSE 4 END,effective_from DESC,created_at DESC LIMIT 1
		) cost ON true
		WHERE ($1::uuid IS NULL OR d.id=$1) AND d.lifecycle_status<>'archived'
		ORDER BY d.business_priority DESC,d.domain_ascii LIMIT $2
	`, domainID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Input{}
	for rows.Next() {
		var item Input
		if err := rows.Scan(&item.DomainID, &item.Domain, &item.Lifecycle, &item.SourceStatus, &item.Priority, &item.Monitoring, &item.Availability, &item.DNS, &item.HTTP, &item.Redirect, &item.ISP, &item.TLS, &item.StatusConfidence, &item.LastCheckedAt, &item.ExpirationAt, &item.RenewalAmount, &item.RenewalCurrency, &item.OpenIncidents, &item.Incidents90Days, &item.DowntimeChanges); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
