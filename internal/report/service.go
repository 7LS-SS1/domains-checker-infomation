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

// StatusCount is one segment of a distribution chart (e.g. how many domains
// are ACTIVE vs UNAVAILABLE).
type StatusCount struct {
	Status string `json:"status"`
	Count  int    `json:"count"`
}

// DailyCount is one point on the incident trend line — always a full,
// gap-free run of consecutive days (zero-filled) so the chart never has to
// guess at missing days.
type DailyCount struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

// TopDomain is one row of the "domains needing attention" table: the
// highest-risk active domains, ranked by availability/ISP/recommendation
// signals already computed elsewhere in the system.
type TopDomain struct {
	Domain              string  `json:"domain"`
	AvailabilityStatus  string  `json:"availability_status"`
	ISPStatus           string  `json:"isp_status"`
	Recommendation      string  `json:"recommendation"`
	RenewalCost         *string `json:"renewal_cost,omitempty"`
	RenewalCostCurrency *string `json:"renewal_cost_currency,omitempty"`
}

// Dashboard extends Summary with the chart- and table-ready data behind the
// visual report page and its PDF export: status distributions, a 30-day
// incident trend, and a top-10 "needs attention" table.
type Dashboard struct {
	Summary
	AvailabilityDistribution []StatusCount `json:"availability_distribution"`
	ISPDistribution          []StatusCount `json:"isp_distribution"`
	IncidentTrend30d         []DailyCount  `json:"incident_trend_30d"`
	TopDomains                []TopDomain   `json:"top_domains"`
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

// Dashboard computes the same KPI snapshot as Summary plus the chart- and
// table-ready data behind the visual report page: status distributions, a
// zero-filled 30-day incident trend, and a top-10 "needs attention" table.
// Read-only and uncached — safe to call from a live view or a PDF export.
func (s *Service) Dashboard(ctx context.Context, currency string) (Dashboard, error) {
	summary, err := s.Summary(ctx, currency)
	if err != nil {
		return Dashboard{}, err
	}
	dashboard := Dashboard{Summary: summary, AvailabilityDistribution: []StatusCount{}, ISPDistribution: []StatusCount{}, IncidentTrend30d: []DailyCount{}, TopDomains: []TopDomain{}}

	availabilityRows, err := s.pool.Query(ctx, `
		SELECT current_availability_status::text, count(*)::int
		FROM domains WHERE lifecycle_status <> 'archived'
		GROUP BY current_availability_status ORDER BY count(*) DESC
	`)
	if err != nil {
		return Dashboard{}, fmt.Errorf("query availability distribution: %w", err)
	}
	for availabilityRows.Next() {
		var item StatusCount
		if err := availabilityRows.Scan(&item.Status, &item.Count); err != nil {
			availabilityRows.Close()
			return Dashboard{}, fmt.Errorf("scan availability distribution: %w", err)
		}
		dashboard.AvailabilityDistribution = append(dashboard.AvailabilityDistribution, item)
	}
	if err := availabilityRows.Err(); err != nil {
		return Dashboard{}, err
	}

	ispRows, err := s.pool.Query(ctx, `
		SELECT current_isp_status::text, count(*)::int
		FROM domains WHERE lifecycle_status <> 'archived'
		GROUP BY current_isp_status ORDER BY count(*) DESC
	`)
	if err != nil {
		return Dashboard{}, fmt.Errorf("query isp distribution: %w", err)
	}
	for ispRows.Next() {
		var item StatusCount
		if err := ispRows.Scan(&item.Status, &item.Count); err != nil {
			ispRows.Close()
			return Dashboard{}, fmt.Errorf("scan isp distribution: %w", err)
		}
		dashboard.ISPDistribution = append(dashboard.ISPDistribution, item)
	}
	if err := ispRows.Err(); err != nil {
		return Dashboard{}, err
	}

	trendRows, err := s.pool.Query(ctx, `
		SELECT day.value::date, COALESCE(opened.count, 0)::int
		FROM generate_series(current_date - interval '29 days', current_date, interval '1 day') AS day(value)
		LEFT JOIN (
			SELECT date_trunc('day', opened_at)::date AS day, count(*)::int AS count
			FROM incidents WHERE opened_at >= now() - interval '30 days'
			GROUP BY day
		) opened ON opened.day = day.value::date
		ORDER BY day.value
	`)
	if err != nil {
		return Dashboard{}, fmt.Errorf("query incident trend: %w", err)
	}
	for trendRows.Next() {
		var item DailyCount
		var day time.Time
		if err := trendRows.Scan(&day, &item.Count); err != nil {
			trendRows.Close()
			return Dashboard{}, fmt.Errorf("scan incident trend: %w", err)
		}
		item.Date = day.Format("2006-01-02")
		dashboard.IncidentTrend30d = append(dashboard.IncidentTrend30d, item)
	}
	if err := trendRows.Err(); err != nil {
		return Dashboard{}, err
	}

	topRows, err := s.pool.Query(ctx, `
		WITH latest AS (
			SELECT DISTINCT ON (domain_id) domain_id, action::text, opportunity_level::text
			FROM recommendations ORDER BY domain_id, generated_at DESC, created_at DESC
		), effective AS (
			SELECT l.domain_id, COALESCE(o.override_value #>> '{}', l.action) action, l.opportunity_level
			FROM latest l LEFT JOIN LATERAL (
				SELECT override_value FROM manual_overrides WHERE domain_id=l.domain_id AND field_name='recommendation' AND revoked_at IS NULL AND effective_from<=now() AND (expires_at IS NULL OR expires_at>now()) ORDER BY effective_from DESC LIMIT 1
			) o ON true
		), renewal_cost AS (
			SELECT DISTINCT ON (domain_id) domain_id, amount, currency_code
			FROM domain_costs
			WHERE cost_type = 'renewal' AND effective_from <= current_date AND (effective_to IS NULL OR effective_to >= current_date)
			ORDER BY domain_id, CASE price_source WHEN 'registrar_api' THEN 1 WHEN 'google_sheet' THEN 2 WHEN 'manual' THEN 3 ELSE 4 END, effective_from DESC, created_at DESC
		)
		SELECT d.domain_ascii, d.current_availability_status::text, d.current_isp_status::text,
		       COALESCE(e.action, 'UNKNOWN'), rc.amount::text, rc.currency_code::text,
		       (CASE WHEN d.current_availability_status <> 'ACTIVE' THEN 2 ELSE 0 END +
		        CASE WHEN d.current_isp_status IN ('SUSPECTED','HIGH_CONFIDENCE_BLOCK') THEN 2 ELSE 0 END +
		        CASE WHEN e.action IN ('DROP','REVIEW') THEN 1 ELSE 0 END) AS risk_score
		FROM domains d
		LEFT JOIN effective e ON e.domain_id = d.id
		LEFT JOIN renewal_cost rc ON rc.domain_id = d.id
		WHERE d.lifecycle_status = 'active'
		ORDER BY risk_score DESC, rc.amount DESC NULLS LAST, d.domain_ascii
		LIMIT 10
	`)
	if err != nil {
		return Dashboard{}, fmt.Errorf("query top domains: %w", err)
	}
	for topRows.Next() {
		var item TopDomain
		var riskScore int
		if err := topRows.Scan(&item.Domain, &item.AvailabilityStatus, &item.ISPStatus, &item.Recommendation, &item.RenewalCost, &item.RenewalCostCurrency, &riskScore); err != nil {
			topRows.Close()
			return Dashboard{}, fmt.Errorf("scan top domains: %w", err)
		}
		dashboard.TopDomains = append(dashboard.TopDomains, item)
	}
	if err := topRows.Err(); err != nil {
		return Dashboard{}, err
	}

	return dashboard, nil
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
	if format != "json" && format != "csv" && format != "pdf" {
		return Record{}, fmt.Errorf("%w: format", ErrValidation)
	}
	var (
		content              []byte
		contentType          string
		generatedAt          time.Time
		reportingCurrency    string
		completenessWarnings []string
		policyVersion        string
	)
	if format == "pdf" {
		dashboard, err := s.Dashboard(ctx, currency)
		if err != nil {
			return Record{}, err
		}
		content, err = renderDashboardPDF(dashboard)
		if err != nil {
			return Record{}, err
		}
		contentType = "application/pdf"
		generatedAt, reportingCurrency = dashboard.GeneratedAt, dashboard.ReportingCurrency
		completenessWarnings, policyVersion = dashboard.CompletenessWarnings, dashboard.RecommendationPolicy
	} else {
		summary, err := s.Summary(ctx, currency)
		if err != nil {
			return Record{}, err
		}
		content, contentType, err = render(summary, format)
		if err != nil {
			return Record{}, err
		}
		generatedAt, reportingCurrency = summary.GeneratedAt, summary.ReportingCurrency
		completenessWarnings, policyVersion = summary.CompletenessWarnings, summary.RecommendationPolicy
	}
	reportType := "summary"
	if format == "pdf" {
		reportType = "dashboard"
	}
	digest := sha256.Sum256(content)
	warnings, _ := json.Marshal(completenessWarnings)
	policy, _ := json.Marshal(map[string]string{"recommendation": policyVersion})
	filters, _ := json.Marshal(map[string]string{"reporting_currency": reportingCurrency})
	now := s.now().UTC()
	record := Record{ID: uuid.New(), ReportType: reportType, Format: format, Status: "completed", Filters: filters, SnapshotAt: generatedAt, PolicyVersions: policy, CompletenessWarnings: completenessWarnings, RowCount: 1, StorageReference: "database:report_payloads", SHA256: hex.EncodeToString(digest[:]), RequestedBy: actor.UserID, RequestedAt: now, CompletedAt: &now}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Record{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, `INSERT INTO reports (id,report_type,format,status,filters,snapshot_at,policy_versions,exchange_rate_snapshot,completeness_warnings,row_count,storage_reference,file_sha256,requested_by,requested_at,completed_at,idempotency_key) VALUES ($1,$2,$3,'completed',$4::jsonb,$5,$6::jsonb,'{}'::jsonb,$7::jsonb,1,$8,$9,$10,$11,$11,$12)`, record.ID, reportType, format, filters, record.SnapshotAt, policy, warnings, record.StorageReference, digest[:], actor.UserID, now, idempotencyKey)
	if err != nil {
		return Record{}, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO report_payloads (report_id,content_type,content) VALUES ($1,$2,$3)`, record.ID, contentType, content); err != nil {
		return Record{}, err
	}
	if err := s.audit.AppendTx(ctx, tx, audit.Entry{ActorUserID: &actor.UserID, Action: "REPORT_GENERATED", ResourceType: "report", ResourceID: &record.ID, RequestID: actor.RequestID, After: map[string]any{"type": reportType, "format": format, "sha256": record.SHA256, "row_count": 1}}); err != nil {
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
