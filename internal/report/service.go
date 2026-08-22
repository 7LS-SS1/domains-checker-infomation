package report

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"domainmonitor/internal/audit"
	"domainmonitor/internal/finance"
	"domainmonitor/internal/recommendation"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound   = errors.New("report not found")
	ErrValidation = errors.New("report validation failed")
)

type Actor struct {
	UserID    uuid.UUID
	RequestID string
}

type Summary struct {
	GeneratedAt            time.Time `json:"generated_at"`
	ReportingCurrency      string    `json:"reporting_currency"`
	TotalDomains           int       `json:"total_domains"`
	ActiveDomains          int       `json:"active_domains"`
	UnavailableDomains     int       `json:"unavailable_domains"`
	PermanentRedirects     int       `json:"permanent_redirect_domains"`
	SuspectedISPBlock      int       `json:"suspected_isp_block"`
	HighConfidenceISPBlock int       `json:"high_confidence_isp_block"`
	DNSErrors              int       `json:"dns_errors"`
	TLSErrors              int       `json:"tls_errors"`
	ExpiringSoon           int       `json:"expiring_within_90_days"`
	RenewalCost            string    `json:"renewal_cost"`
	EstimatedTax           string    `json:"estimated_tax"`
	AnnualBudget           string    `json:"annual_budget"`
	RecommendedRenew       int       `json:"recommended_renew"`
	RecommendedDrop        int       `json:"recommended_drop"`
	ReviewRequired         int       `json:"review_required"`
	ProfitOpportunities    int       `json:"profit_opportunities"`
	FinanceComplete        bool      `json:"finance_complete"`
	CompletenessWarnings   []string  `json:"completeness_warnings"`
	RecommendationPolicy   string    `json:"recommendation_policy_version"`
}

type Record struct {
	ID                   uuid.UUID       `json:"id"`
	ReportType           string          `json:"report_type"`
	Format               string          `json:"format"`
	Status               string          `json:"status"`
	Filters              json.RawMessage `json:"filters"`
	SnapshotAt           time.Time       `json:"snapshot_at"`
	PolicyVersions       json.RawMessage `json:"policy_versions"`
	CompletenessWarnings []string        `json:"completeness_warnings"`
	RowCount             int64           `json:"row_count"`
	StorageReference     string          `json:"storage_reference"`
	SHA256               string          `json:"sha256"`
	RequestedBy          uuid.UUID       `json:"requested_by"`
	RequestedAt          time.Time       `json:"requested_at"`
	CompletedAt          *time.Time      `json:"completed_at,omitempty"`
}

type Payload struct {
	Record      Record
	ContentType string
	Content     []byte
}

type Service struct {
	pool    *pgxpool.Pool
	audit   *audit.Store
	finance *finance.Service
	now     func() time.Time
}

func NewService(pool *pgxpool.Pool, auditStore *audit.Store, financeService *finance.Service) *Service {
	return &Service{pool: pool, audit: auditStore, finance: financeService, now: time.Now}
}

func (s *Service) Summary(ctx context.Context, currency string) (Summary, error) {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if currency == "" {
		currency = "THB"
	}
	budget, err := s.finance.BudgetSummary(ctx, currency)
	if err != nil {
		return Summary{}, err
	}
	result := Summary{GeneratedAt: s.now().UTC(), ReportingCurrency: budget.ReportingCurrency, RenewalCost: budget.TotalRenewalCost, EstimatedTax: budget.EstimatedTax, AnnualBudget: budget.TotalAnnualBudget, FinanceComplete: budget.Complete, CompletenessWarnings: budget.Warnings, RecommendationPolicy: recommendation.PolicyVersion}
	err = s.pool.QueryRow(ctx, `
		WITH latest AS (
			SELECT DISTINCT ON (domain_id) domain_id,action::text,opportunity_level::text FROM recommendations ORDER BY domain_id,generated_at DESC,created_at DESC
		), effective AS (
			SELECT l.domain_id,COALESCE(o.override_value #>> '{}',l.action) action,l.opportunity_level
			FROM latest l LEFT JOIN LATERAL (
				SELECT override_value FROM manual_overrides WHERE domain_id=l.domain_id AND field_name='recommendation' AND revoked_at IS NULL AND effective_from<=now() AND (expires_at IS NULL OR expires_at>now()) ORDER BY effective_from DESC LIMIT 1
			) o ON true
		)
		SELECT count(*)::int,
		count(*) FILTER (WHERE d.lifecycle_status='active')::int,
		count(*) FILTER (WHERE d.current_availability_status='UNAVAILABLE')::int,
		count(*) FILTER (WHERE d.current_redirect_status='PERMANENT')::int,
		count(*) FILTER (WHERE d.current_isp_status='SUSPECTED')::int,
		count(*) FILTER (WHERE d.current_isp_status='HIGH_CONFIDENCE_BLOCK')::int,
		count(*) FILTER (WHERE d.current_dns_status NOT IN ('OK','UNKNOWN'))::int,
		count(*) FILTER (WHERE d.current_tls_status NOT IN ('VALID','EXPIRING','NOT_APPLICABLE','UNKNOWN'))::int,
		count(*) FILTER (WHERE d.expiration_at>=now() AND d.expiration_at<now()+interval '90 days')::int,
		count(*) FILTER (WHERE e.action='RENEW')::int,
		count(*) FILTER (WHERE e.action='DROP')::int,
		count(*) FILTER (WHERE e.action='REVIEW')::int,
		count(*) FILTER (WHERE e.action='PROFIT_OPPORTUNITY' OR e.opportunity_level='HIGH')::int
		FROM domains d LEFT JOIN effective e ON e.domain_id=d.id WHERE d.lifecycle_status<>'archived'
	`).Scan(&result.TotalDomains, &result.ActiveDomains, &result.UnavailableDomains, &result.PermanentRedirects, &result.SuspectedISPBlock, &result.HighConfidenceISPBlock, &result.DNSErrors, &result.TLSErrors, &result.ExpiringSoon, &result.RecommendedRenew, &result.RecommendedDrop, &result.ReviewRequired, &result.ProfitOpportunities)
	return result, err
}

func (s *Service) Create(ctx context.Context, actor Actor, idempotencyKey, format, currency string) (Record, error) {
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if len(idempotencyKey) < 8 || len(idempotencyKey) > 200 {
		return Record{}, fmt.Errorf("%w: idempotency key", ErrValidation)
	}
	if existing, getErr := scanRecord(s.pool.QueryRow(ctx, reportSelect+` WHERE requested_by=$1 AND idempotency_key=$2`, actor.UserID, idempotencyKey)); getErr == nil {
		return existing, nil
	}
	format = strings.ToLower(strings.TrimSpace(format))
	if format != "json" && format != "csv" {
		return Record{}, fmt.Errorf("%w: format", ErrValidation)
	}
	summary, err := s.Summary(ctx, currency)
	if err != nil {
		return Record{}, err
	}
	content, contentType, err := render(summary, format)
	if err != nil {
		return Record{}, err
	}
	digest := sha256.Sum256(content)
	warnings, _ := json.Marshal(summary.CompletenessWarnings)
	policy, _ := json.Marshal(map[string]string{"recommendation": summary.RecommendationPolicy})
	filters, _ := json.Marshal(map[string]string{"reporting_currency": summary.ReportingCurrency})
	now := s.now().UTC()
	record := Record{ID: uuid.New(), ReportType: "summary", Format: format, Status: "completed", Filters: filters, SnapshotAt: summary.GeneratedAt, PolicyVersions: policy, CompletenessWarnings: summary.CompletenessWarnings, RowCount: 1, StorageReference: "database:report_payloads", SHA256: hex.EncodeToString(digest[:]), RequestedBy: actor.UserID, RequestedAt: now, CompletedAt: &now}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Record{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, `INSERT INTO reports (id,report_type,format,status,filters,snapshot_at,policy_versions,exchange_rate_snapshot,completeness_warnings,row_count,storage_reference,file_sha256,requested_by,requested_at,completed_at,idempotency_key) VALUES ($1,'summary',$2,'completed',$3::jsonb,$4,$5::jsonb,'{}'::jsonb,$6::jsonb,1,$7,$8,$9,$10,$10,$11)`, record.ID, format, filters, record.SnapshotAt, policy, warnings, record.StorageReference, digest[:], actor.UserID, now, idempotencyKey)
	if err != nil {
		return Record{}, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO report_payloads (report_id,content_type,content) VALUES ($1,$2,$3)`, record.ID, contentType, content); err != nil {
		return Record{}, err
	}
	if err := s.audit.AppendTx(ctx, tx, audit.Entry{ActorUserID: &actor.UserID, Action: "REPORT_GENERATED", ResourceType: "report", ResourceID: &record.ID, RequestID: actor.RequestID, After: map[string]any{"type": "summary", "format": format, "sha256": record.SHA256, "row_count": 1}}); err != nil {
		return Record{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Record{}, err
	}
	return record, nil
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (Record, error) {
	return scanRecord(s.pool.QueryRow(ctx, reportSelect+` WHERE id=$1`, id))
}

func (s *Service) Download(ctx context.Context, id uuid.UUID) (Payload, error) {
	record, err := s.Get(ctx, id)
	if err != nil {
		return Payload{}, err
	}
	var payload Payload
	payload.Record = record
	err = s.pool.QueryRow(ctx, `SELECT content_type,content FROM report_payloads WHERE report_id=$1`, id).Scan(&payload.ContentType, &payload.Content)
	if errors.Is(err, pgx.ErrNoRows) {
		return Payload{}, ErrNotFound
	}
	return payload, err
}

const reportSelect = `SELECT id,report_type,format,status::text,filters,snapshot_at,policy_versions,completeness_warnings,row_count,COALESCE(storage_reference,''),COALESCE(encode(file_sha256,'hex'),''),requested_by,requested_at,completed_at FROM reports`

func scanRecord(row pgx.Row) (Record, error) {
	var result Record
	var warnings []byte
	err := row.Scan(&result.ID, &result.ReportType, &result.Format, &result.Status, &result.Filters, &result.SnapshotAt, &result.PolicyVersions, &warnings, &result.RowCount, &result.StorageReference, &result.SHA256, &result.RequestedBy, &result.RequestedAt, &result.CompletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Record{}, ErrNotFound
	}
	if err != nil {
		return Record{}, err
	}
	_ = json.Unmarshal(warnings, &result.CompletenessWarnings)
	return result, nil
}

func render(summary Summary, format string) ([]byte, string, error) {
	if format == "json" {
		content, err := json.MarshalIndent(summary, "", "  ")
		return append(content, '\n'), "application/json", err
	}
	buffer := &bytes.Buffer{}
	writer := csv.NewWriter(buffer)
	header := []string{"generated_at", "reporting_currency", "total_domains", "active_domains", "unavailable_domains", "permanent_redirect_domains", "suspected_isp_block", "high_confidence_isp_block", "dns_errors", "tls_errors", "expiring_within_90_days", "renewal_cost", "estimated_tax", "annual_budget", "recommended_renew", "recommended_drop", "review_required", "profit_opportunities", "finance_complete", "recommendation_policy_version"}
	values := []string{summary.GeneratedAt.Format(time.RFC3339), summary.ReportingCurrency, fmt.Sprint(summary.TotalDomains), fmt.Sprint(summary.ActiveDomains), fmt.Sprint(summary.UnavailableDomains), fmt.Sprint(summary.PermanentRedirects), fmt.Sprint(summary.SuspectedISPBlock), fmt.Sprint(summary.HighConfidenceISPBlock), fmt.Sprint(summary.DNSErrors), fmt.Sprint(summary.TLSErrors), fmt.Sprint(summary.ExpiringSoon), summary.RenewalCost, summary.EstimatedTax, summary.AnnualBudget, fmt.Sprint(summary.RecommendedRenew), fmt.Sprint(summary.RecommendedDrop), fmt.Sprint(summary.ReviewRequired), fmt.Sprint(summary.ProfitOpportunities), fmt.Sprint(summary.FinanceComplete), summary.RecommendationPolicy}
	if err := writer.Write(header); err != nil {
		return nil, "", err
	}
	if err := writer.Write(values); err != nil {
		return nil, "", err
	}
	writer.Flush()
	return buffer.Bytes(), "text/csv", writer.Error()
}
