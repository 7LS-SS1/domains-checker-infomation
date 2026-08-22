package finance

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
	ErrNotFound   = errors.New("financial record not found")
	ErrConflict   = errors.New("financial record conflict")
	ErrValidation = errors.New("financial validation failed")
)

type Actor struct {
	UserID    uuid.UUID
	RequestID string
}

type CostRecord struct {
	ID                 uuid.UUID  `json:"id"`
	DomainID           uuid.UUID  `json:"domain_id"`
	CostType           string     `json:"cost_type"`
	Amount             string     `json:"amount"`
	Currency           string     `json:"currency"`
	PriceSource        string     `json:"price_source"`
	TaxRate            *string    `json:"tax_rate"`
	TaxMode            string     `json:"tax_mode"`
	BillingCycleMonths int        `json:"billing_cycle_months"`
	EffectiveFrom      time.Time  `json:"effective_from"`
	EffectiveTo        *time.Time `json:"effective_to,omitempty"`
	SourceReference    string     `json:"source_reference"`
	Calculation        CostResult `json:"calculation"`
}

type AddCostInput struct {
	CostType           string
	Amount             string
	Currency           string
	TaxRate            *string
	TaxMode            string
	BillingCycleMonths int
	EffectiveFrom      time.Time
	SourceReference    string
	Reason             string
}

type RateRecord struct {
	ID            uuid.UUID `json:"id"`
	BaseCurrency  string    `json:"base_currency"`
	QuoteCurrency string    `json:"quote_currency"`
	Rate          string    `json:"rate"`
	Source        string    `json:"source"`
	ObservedAt    time.Time `json:"observed_at"`
}

type AddRateInput struct {
	BaseCurrency, QuoteCurrency, Rate, Source string
	ObservedAt                                time.Time
	Reason                                    string
}

type OverrideRecord struct {
	ID            uuid.UUID       `json:"id"`
	DomainID      uuid.UUID       `json:"domain_id"`
	FieldName     string          `json:"field_name"`
	OriginalValue json.RawMessage `json:"original_value"`
	OverrideValue json.RawMessage `json:"override_value"`
	Reason        string          `json:"reason"`
	CreatedBy     uuid.UUID       `json:"created_by"`
	EffectiveFrom time.Time       `json:"effective_from"`
	ExpiresAt     *time.Time      `json:"expires_at,omitempty"`
	RevokedAt     *time.Time      `json:"revoked_at,omitempty"`
}

type BudgetWindow struct {
	DomainCount   int    `json:"domain_count"`
	KnownRenewals int    `json:"known_renewals"`
	UnknownCosts  int    `json:"unknown_costs"`
	RenewalTotal  string `json:"renewal_total"`
}

type Summary struct {
	ReportingCurrency      string                  `json:"reporting_currency"`
	GeneratedAt            time.Time               `json:"generated_at"`
	TotalCurrentDomainCost string                  `json:"total_current_domain_cost"`
	TotalRenewalCost       string                  `json:"total_renewal_cost"`
	EstimatedTax           string                  `json:"estimated_tax"`
	TotalAnnualBudget      string                  `json:"total_annual_budget"`
	Complete               bool                    `json:"complete"`
	UnknownCostCount       int                     `json:"unknown_cost_count"`
	UnknownTaxCount        int                     `json:"unknown_tax_count"`
	FXIncompleteCount      int                     `json:"fx_incomplete_count"`
	Warnings               []string                `json:"warnings"`
	Windows                map[string]BudgetWindow `json:"windows"`
}

type Service struct {
	pool     *pgxpool.Pool
	audit    *audit.Store
	fxMaxAge time.Duration
	now      func() time.Time
}

func NewService(pool *pgxpool.Pool, auditStore *audit.Store, fxMaxAge time.Duration) *Service {
	return &Service{pool: pool, audit: auditStore, fxMaxAge: fxMaxAge, now: time.Now}
}

func (s *Service) AddCost(ctx context.Context, actor Actor, domainID uuid.UUID, input AddCostInput) (CostRecord, error) {
	input.CostType = strings.ToLower(strings.TrimSpace(input.CostType))
	if input.CostType != "purchase" && input.CostType != "renewal" {
		return CostRecord{}, fmt.Errorf("%w: cost_type", ErrValidation)
	}
	calculation, err := Calculate(CostInput{Amount: input.Amount, Currency: input.Currency, TaxRate: input.TaxRate, TaxMode: input.TaxMode, BillingCycleMonths: input.BillingCycleMonths})
	if err != nil {
		return CostRecord{}, fmt.Errorf("%w: %v", ErrValidation, err)
	}
	if input.EffectiveFrom.IsZero() {
		input.EffectiveFrom = s.now().UTC()
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return CostRecord{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM domains WHERE id=$1)`, domainID).Scan(&exists); err != nil || !exists {
		if err == nil {
			err = ErrNotFound
		}
		return CostRecord{}, err
	}
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM currencies WHERE code=$1 AND active=true)`, calculation.Currency).Scan(&exists); err != nil || !exists {
		if err == nil {
			err = fmt.Errorf("%w: currency", ErrValidation)
		}
		return CostRecord{}, err
	}
	record := CostRecord{ID: uuid.New(), DomainID: domainID, CostType: input.CostType, Amount: calculation.CycleAmount, Currency: calculation.Currency, PriceSource: "manual", TaxRate: input.TaxRate, TaxMode: strings.ToLower(strings.TrimSpace(input.TaxMode)), BillingCycleMonths: calculation.BillingCycleMonths, EffectiveFrom: input.EffectiveFrom, SourceReference: strings.TrimSpace(input.SourceReference), Calculation: calculation}
	if record.TaxMode == "" {
		record.TaxMode = TaxUnknown
	}
	_, err = tx.Exec(ctx, `
		UPDATE domain_costs SET effective_to=$3::date - 1
		WHERE domain_id=$1 AND cost_type=$2 AND price_source='manual' AND effective_to IS NULL AND effective_from < $3::date
	`, domainID, record.CostType, record.EffectiveFrom)
	if err != nil {
		return CostRecord{}, err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO domain_costs (id,domain_id,cost_type,amount,currency_code,price_source,tax_rate,tax_inclusive,tax_mode,billing_cycle_months,effective_from,source_reference)
		VALUES ($1,$2,$3,$4::numeric,$5,'manual',$6::numeric,$7,$8,$9,$10::date,$11)
	`, record.ID, domainID, record.CostType, record.Amount, record.Currency, record.TaxRate, record.TaxMode == TaxInclusive, record.TaxMode, record.BillingCycleMonths, record.EffectiveFrom, record.SourceReference)
	if err != nil {
		return CostRecord{}, fmt.Errorf("insert domain cost: %w", err)
	}
	if err := s.audit.AppendTx(ctx, tx, audit.Entry{ActorUserID: &actor.UserID, Action: "DOMAIN_COST_ADDED", ResourceType: "domain_cost", ResourceID: &record.ID, RequestID: actor.RequestID, Reason: strings.TrimSpace(input.Reason), After: record}); err != nil {
		return CostRecord{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return CostRecord{}, err
	}
	return record, nil
}

func (s *Service) ListCosts(ctx context.Context, domainID uuid.UUID) ([]CostRecord, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id,domain_id,cost_type,amount::text,currency_code::text,price_source,tax_rate::text,tax_mode,billing_cycle_months,effective_from,effective_to,COALESCE(source_reference,'')
		FROM domain_costs WHERE domain_id=$1 ORDER BY cost_type,effective_from DESC,created_at DESC LIMIT 200
	`, domainID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []CostRecord{}
	for rows.Next() {
		var item CostRecord
		if err := rows.Scan(&item.ID, &item.DomainID, &item.CostType, &item.Amount, &item.Currency, &item.PriceSource, &item.TaxRate, &item.TaxMode, &item.BillingCycleMonths, &item.EffectiveFrom, &item.EffectiveTo, &item.SourceReference); err != nil {
			return nil, err
		}
		item.Calculation, _ = Calculate(CostInput{Amount: item.Amount, Currency: item.Currency, TaxRate: item.TaxRate, TaxMode: item.TaxMode, BillingCycleMonths: item.BillingCycleMonths})
		items = append(items, item)
	}
	if len(items) == 0 {
		var exists bool
		if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM domains WHERE id=$1)`, domainID).Scan(&exists); err != nil {
			return nil, err
		}
		if !exists {
			return nil, ErrNotFound
		}
	}
	return items, rows.Err()
}

func (s *Service) AddRate(ctx context.Context, actor Actor, input AddRateInput) (RateRecord, error) {
	base := strings.ToUpper(strings.TrimSpace(input.BaseCurrency))
	quote := strings.ToUpper(strings.TrimSpace(input.QuoteCurrency))
	rate, err := ParseDecimal(input.Rate, 10)
	if err != nil || rate.Sign() <= 0 || base == quote || len(base) != 3 || len(quote) != 3 || strings.TrimSpace(input.Source) == "" {
		return RateRecord{}, fmt.Errorf("%w: invalid exchange rate", ErrValidation)
	}
	if input.ObservedAt.IsZero() {
		input.ObservedAt = s.now().UTC()
	}
	record := RateRecord{ID: uuid.New(), BaseCurrency: base, QuoteCurrency: quote, Rate: rate.String(10), Source: strings.TrimSpace(input.Source), ObservedAt: input.ObservedAt.UTC()}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return RateRecord{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, `INSERT INTO exchange_rates (id,base_currency,quote_currency,rate,source,observed_at) VALUES ($1,$2,$3,$4::numeric,$5,$6)`, record.ID, base, quote, record.Rate, record.Source, record.ObservedAt)
	if err != nil {
		return RateRecord{}, fmt.Errorf("insert exchange rate: %w", err)
	}
	if err := s.audit.AppendTx(ctx, tx, audit.Entry{ActorUserID: &actor.UserID, Action: "EXCHANGE_RATE_ADDED", ResourceType: "exchange_rate", ResourceID: &record.ID, RequestID: actor.RequestID, Reason: strings.TrimSpace(input.Reason), After: record}); err != nil {
		return RateRecord{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return RateRecord{}, err
	}
	return record, nil
}

var overrideFields = map[string]bool{"recommendation": true, "renewal_price": true, "tax_rate": true, "expiration_date": true, "business_priority": true}

func (s *Service) CreateOverride(ctx context.Context, actor Actor, domainID uuid.UUID, field string, value json.RawMessage, reason string, expiresAt *time.Time) (OverrideRecord, error) {
	field = strings.ToLower(strings.TrimSpace(field))
	reason = strings.TrimSpace(reason)
	if !overrideFields[field] || reason == "" || !json.Valid(value) || string(value) == "null" {
		return OverrideRecord{}, fmt.Errorf("%w: invalid manual override", ErrValidation)
	}
	if expiresAt != nil && !expiresAt.After(s.now()) {
		return OverrideRecord{}, fmt.Errorf("%w: expires_at must be in the future", ErrValidation)
	}
	if err := validateOverrideValue(field, value); err != nil {
		return OverrideRecord{}, fmt.Errorf("%w: %v", ErrValidation, err)
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return OverrideRecord{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	original, err := originalValue(ctx, tx, domainID, field)
	if err != nil {
		return OverrideRecord{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE manual_overrides SET revoked_at=now() WHERE domain_id=$1 AND field_name=$2 AND revoked_at IS NULL`, domainID, field); err != nil {
		return OverrideRecord{}, err
	}
	record := OverrideRecord{ID: uuid.New(), DomainID: domainID, FieldName: field, OriginalValue: original, OverrideValue: value, Reason: reason, CreatedBy: actor.UserID, EffectiveFrom: s.now().UTC(), ExpiresAt: expiresAt}
	_, err = tx.Exec(ctx, `INSERT INTO manual_overrides (id,domain_id,field_name,original_value,override_value,reason,created_by,effective_from,expires_at) VALUES ($1,$2,$3,$4::jsonb,$5::jsonb,$6,$7,$8,$9)`, record.ID, domainID, field, original, value, reason, actor.UserID, record.EffectiveFrom, expiresAt)
	if err != nil {
		return OverrideRecord{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE domain_field_provenance SET is_current=false WHERE domain_id=$1 AND field_name=$2 AND is_current=true`, domainID, field); err != nil {
		return OverrideRecord{}, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO domain_field_provenance (domain_id,field_name,value,source,source_reference,observed_at,is_current) VALUES ($1,$2,$3::jsonb,'manual',$4,$5,true)`, domainID, field, value, "manual-override:"+record.ID.String(), record.EffectiveFrom); err != nil {
		return OverrideRecord{}, err
	}
	if err := s.audit.AppendTx(ctx, tx, audit.Entry{ActorUserID: &actor.UserID, Action: "MANUAL_OVERRIDE_CREATED", ResourceType: "manual_override", ResourceID: &record.ID, RequestID: actor.RequestID, Reason: reason, Before: json.RawMessage(original), After: json.RawMessage(value), Metadata: map[string]any{"domain_id": domainID, "field": field}}); err != nil {
		return OverrideRecord{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return OverrideRecord{}, err
	}
	return record, nil
}

func (s *Service) RevokeOverride(ctx context.Context, actor Actor, domainID, overrideID uuid.UUID, reason string) error {
	if strings.TrimSpace(reason) == "" {
		return fmt.Errorf("%w: reason", ErrValidation)
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	command, err := tx.Exec(ctx, `UPDATE manual_overrides SET revoked_at=now() WHERE id=$1 AND domain_id=$2 AND revoked_at IS NULL`, overrideID, domainID)
	if err != nil {
		return err
	}
	if command.RowsAffected() != 1 {
		return ErrNotFound
	}
	if _, err := tx.Exec(ctx, `UPDATE domain_field_provenance SET is_current=false WHERE domain_id=$1 AND field_name=(SELECT field_name FROM manual_overrides WHERE id=$2) AND source_reference=$3 AND is_current=true`, domainID, overrideID, "manual-override:"+overrideID.String()); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE domain_field_provenance SET is_current=true
		WHERE id=(SELECT id FROM domain_field_provenance WHERE domain_id=$1 AND field_name=(SELECT field_name FROM manual_overrides WHERE id=$2) AND source_reference<>$3 ORDER BY observed_at DESC,created_at DESC LIMIT 1)
	`, domainID, overrideID, "manual-override:"+overrideID.String()); err != nil {
		return err
	}
	if err := s.audit.AppendTx(ctx, tx, audit.Entry{ActorUserID: &actor.UserID, Action: "MANUAL_OVERRIDE_REVOKED", ResourceType: "manual_override", ResourceID: &overrideID, RequestID: actor.RequestID, Reason: strings.TrimSpace(reason), Metadata: map[string]any{"domain_id": domainID}}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) ListOverrides(ctx context.Context, domainID uuid.UUID) ([]OverrideRecord, error) {
	rows, err := s.pool.Query(ctx, `SELECT id,domain_id,field_name,original_value,override_value,reason,created_by,effective_from,expires_at,revoked_at FROM manual_overrides WHERE domain_id=$1 ORDER BY effective_from DESC LIMIT 200`, domainID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []OverrideRecord{}
	for rows.Next() {
		var item OverrideRecord
		if err := rows.Scan(&item.ID, &item.DomainID, &item.FieldName, &item.OriginalValue, &item.OverrideValue, &item.Reason, &item.CreatedBy, &item.EffectiveFrom, &item.ExpiresAt, &item.RevokedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func validateOverrideValue(field string, value json.RawMessage) error {
	var text string
	if err := json.Unmarshal(value, &text); err != nil {
		return errors.New("override value must be a JSON string")
	}
	switch field {
	case "renewal_price":
		amount, err := ParseDecimal(text, 6)
		if err != nil || amount.Sign() < 0 {
			return errors.New("renewal_price must be a non-negative decimal string")
		}
	case "tax_rate":
		rate, err := ParseDecimal(text, 10)
		if err != nil || rate.Sign() < 0 || rate.value.Cmp(decimalFromInt(1).value) > 0 {
			return errors.New("tax_rate must be a decimal string between 0 and 1")
		}
	case "expiration_date":
		if _, err := time.Parse("2006-01-02", text); err != nil {
			return errors.New("expiration_date must use YYYY-MM-DD")
		}
	case "business_priority":
		if text != "low" && text != "medium" && text != "high" && text != "critical" {
			return errors.New("invalid business_priority")
		}
	case "recommendation":
		if text != "RENEW" && text != "DROP" && text != "REVIEW" && text != "PROFIT_OPPORTUNITY" {
			return errors.New("invalid recommendation")
		}
	}
	return nil
}

func originalValue(ctx context.Context, tx pgx.Tx, domainID uuid.UUID, field string) (json.RawMessage, error) {
	var raw []byte
	switch field {
	case "expiration_date":
		err := tx.QueryRow(ctx, `SELECT to_jsonb(expiration_at::date) FROM domains WHERE id=$1`, domainID).Scan(&raw)
		return raw, mapNoRows(err)
	case "business_priority":
		err := tx.QueryRow(ctx, `SELECT to_jsonb(business_priority::text) FROM domains WHERE id=$1`, domainID).Scan(&raw)
		return raw, mapNoRows(err)
	case "renewal_price":
		err := tx.QueryRow(ctx, `SELECT to_jsonb(amount::text) FROM domain_costs WHERE domain_id=$1 AND cost_type='renewal' AND effective_to IS NULL ORDER BY CASE price_source WHEN 'registrar_api' THEN 1 WHEN 'google_sheet' THEN 2 WHEN 'manual' THEN 3 ELSE 4 END,effective_from DESC LIMIT 1`, domainID).Scan(&raw)
		if errors.Is(err, pgx.ErrNoRows) {
			return json.RawMessage("null"), nil
		}
		return raw, err
	case "tax_rate":
		err := tx.QueryRow(ctx, `SELECT to_jsonb(tax_rate::text) FROM domain_costs WHERE domain_id=$1 AND cost_type='renewal' AND effective_to IS NULL ORDER BY CASE price_source WHEN 'registrar_api' THEN 1 WHEN 'google_sheet' THEN 2 WHEN 'manual' THEN 3 ELSE 4 END,effective_from DESC LIMIT 1`, domainID).Scan(&raw)
		if errors.Is(err, pgx.ErrNoRows) {
			return json.RawMessage("null"), nil
		}
		return raw, err
	default:
		return json.RawMessage("null"), nil
	}
}

func mapNoRows(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
