package finance

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type budgetCost struct {
	domainID  uuid.UUID
	expiresAt *time.Time
	amount    *string
	currency  *string
	taxRate   *string
	taxMode   *string
	months    *int
}

type fxRate struct {
	rate       string
	observedAt time.Time
}

func (s *Service) BudgetSummary(ctx context.Context, reportingCurrency string) (Summary, error) {
	reportingCurrency = normalizeCurrency(reportingCurrency)
	if len(reportingCurrency) != 3 {
		return Summary{}, fmt.Errorf("%w: reporting_currency", ErrValidation)
	}
	now := s.now().UTC()
	summary := Summary{
		ReportingCurrency: reportingCurrency,
		GeneratedAt:       now,
		Complete:          true,
		Warnings:          []string{},
		Windows: map[string]BudgetWindow{
			"next_30_days": {}, "next_60_days": {}, "next_90_days": {}, "this_year": {},
		},
	}
	rows, err := s.pool.Query(ctx, `
		SELECT d.id,d.expiration_at,c.amount::text,c.currency_code::text,c.tax_rate::text,c.tax_mode,c.billing_cycle_months
		FROM domains d
		LEFT JOIN LATERAL (
			SELECT amount,currency_code,tax_rate,tax_mode,billing_cycle_months
			FROM domain_costs
			WHERE domain_id=d.id AND cost_type='renewal' AND effective_from <= current_date
			  AND (effective_to IS NULL OR effective_to >= current_date)
			ORDER BY CASE price_source WHEN 'registrar_api' THEN 1 WHEN 'google_sheet' THEN 2 WHEN 'manual' THEN 3 ELSE 4 END,
			         effective_from DESC,created_at DESC LIMIT 1
		) c ON true
		WHERE d.lifecycle_status='active'
		ORDER BY d.id
	`)
	if err != nil {
		return Summary{}, err
	}
	costs := []budgetCost{}
	for rows.Next() {
		var item budgetCost
		if err := rows.Scan(&item.domainID, &item.expiresAt, &item.amount, &item.currency, &item.taxRate, &item.taxMode, &item.months); err != nil {
			rows.Close()
			return Summary{}, err
		}
		costs = append(costs, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return Summary{}, err
	}
	rows.Close()

	overrides, err := s.activeOverrides(ctx)
	if err != nil {
		return Summary{}, err
	}
	for index := range costs {
		if fields := overrides[costs[index].domainID]; fields != nil {
			if value, ok := fields["renewal_price"]; ok {
				costs[index].amount = &value
			}
			if value, ok := fields["tax_rate"]; ok {
				costs[index].taxRate = &value
				mode := TaxExclusive
				costs[index].taxMode = &mode
			}
			if value, ok := fields["expiration_date"]; ok {
				if parsed, parseErr := time.Parse("2006-01-02", value); parseErr == nil {
					costs[index].expiresAt = &parsed
				}
			}
		}
	}
	rates, err := s.latestRates(ctx, reportingCurrency)
	if err != nil {
		return Summary{}, err
	}
	renewalTotal := decimalFromInt(0)
	taxTotal := decimalFromInt(0)
	annualTotal := decimalFromInt(0)
	windowTotals := map[string]Decimal{"next_30_days": decimalFromInt(0), "next_60_days": decimalFromInt(0), "next_90_days": decimalFromInt(0), "this_year": decimalFromInt(0)}
	warnings := map[string]bool{}
	for _, item := range costs {
		windows := applicableWindows(now, item.expiresAt)
		for _, name := range windows {
			window := summary.Windows[name]
			window.DomainCount++
			summary.Windows[name] = window
		}
		if item.amount == nil || item.currency == nil || item.taxMode == nil || item.months == nil {
			summary.UnknownCostCount++
			for _, name := range windows {
				window := summary.Windows[name]
				window.UnknownCosts++
				summary.Windows[name] = window
			}
			continue
		}
		calculation, calcErr := Calculate(CostInput{Amount: *item.amount, Currency: *item.currency, TaxRate: item.taxRate, TaxMode: *item.taxMode, BillingCycleMonths: *item.months})
		if calcErr != nil {
			summary.UnknownCostCount++
			warnings["INVALID_COST_RECORD"] = true
			continue
		}
		if !calculation.Complete {
			summary.UnknownTaxCount++
			warnings["TAX_POLICY_UNKNOWN"] = true
			continue
		}
		cycleConverted, ok := convertForSummary(*calculation.CycleTotal, calculation.Currency, reportingCurrency, rates, now, s.fxMaxAge)
		if !ok {
			summary.FXIncompleteCount++
			warnings["FX_RATE_MISSING_OR_STALE"] = true
			continue
		}
		taxConverted, _ := convertForSummary(*calculation.TaxAmount, calculation.Currency, reportingCurrency, rates, now, s.fxMaxAge)
		annualConverted, _ := convertForSummary(*calculation.AnnualEstimate, calculation.Currency, reportingCurrency, rates, now, s.fxMaxAge)
		renewalTotal = renewalTotal.Add(cycleConverted)
		taxTotal = taxTotal.Add(taxConverted)
		annualTotal = annualTotal.Add(annualConverted)
		for _, name := range windows {
			windowTotals[name] = windowTotals[name].Add(cycleConverted)
			window := summary.Windows[name]
			window.KnownRenewals++
			summary.Windows[name] = window
		}
	}

	currentTotal, currentIncomplete, err := s.currentPurchaseTotal(ctx, reportingCurrency, rates, now)
	if err != nil {
		return Summary{}, err
	}
	if currentIncomplete > 0 {
		summary.FXIncompleteCount += currentIncomplete
		warnings["PURCHASE_TOTAL_INCOMPLETE"] = true
	}
	summary.TotalCurrentDomainCost = currentTotal.String(6)
	summary.TotalRenewalCost = renewalTotal.String(6)
	summary.EstimatedTax = taxTotal.String(6)
	summary.TotalAnnualBudget = annualTotal.String(6)
	for name, total := range windowTotals {
		window := summary.Windows[name]
		window.RenewalTotal = total.String(6)
		summary.Windows[name] = window
	}
	for _, code := range []string{"TAX_POLICY_UNKNOWN", "FX_RATE_MISSING_OR_STALE", "INVALID_COST_RECORD", "PURCHASE_TOTAL_INCOMPLETE"} {
		if warnings[code] {
			summary.Warnings = append(summary.Warnings, code)
		}
	}
	summary.Complete = summary.UnknownCostCount == 0 && summary.UnknownTaxCount == 0 && summary.FXIncompleteCount == 0
	return summary, nil
}

func (s *Service) activeOverrides(ctx context.Context) (map[uuid.UUID]map[string]string, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT ON (domain_id,field_name) domain_id,field_name,override_value
		FROM manual_overrides
		WHERE revoked_at IS NULL AND effective_from <= now() AND (expires_at IS NULL OR expires_at > now())
		ORDER BY domain_id,field_name,effective_from DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[uuid.UUID]map[string]string{}
	for rows.Next() {
		var domainID uuid.UUID
		var field string
		var raw json.RawMessage
		if err := rows.Scan(&domainID, &field, &raw); err != nil {
			return nil, err
		}
		var value string
		if json.Unmarshal(raw, &value) != nil {
			continue
		}
		if result[domainID] == nil {
			result[domainID] = map[string]string{}
		}
		result[domainID][field] = value
	}
	return result, rows.Err()
}

func (s *Service) latestRates(ctx context.Context, quote string) (map[string]fxRate, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT ON (base_currency) base_currency::text,rate::text,observed_at
		FROM exchange_rates WHERE quote_currency=$1
		ORDER BY base_currency,observed_at DESC,created_at DESC
	`, quote)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[string]fxRate{}
	for rows.Next() {
		var currency string
		var rate fxRate
		if err := rows.Scan(&currency, &rate.rate, &rate.observedAt); err != nil {
			return nil, err
		}
		result[normalizeCurrency(currency)] = rate
	}
	return result, rows.Err()
}

func convertForSummary(amount, from, to string, rates map[string]fxRate, now time.Time, maxAge time.Duration) (Decimal, bool) {
	parsed, err := ParseDecimal(amount, 6)
	if err != nil {
		return Decimal{}, false
	}
	from = normalizeCurrency(from)
	if from == to {
		return parsed, true
	}
	rate, ok := rates[from]
	if !ok || rate.observedAt.IsZero() || now.Sub(rate.observedAt) > maxAge || rate.observedAt.After(now.Add(5*time.Minute)) {
		return Decimal{}, false
	}
	parsedRate, err := ParseDecimal(rate.rate, 10)
	if err != nil || parsedRate.Sign() <= 0 {
		return Decimal{}, false
	}
	return parsed.Mul(parsedRate), true
}

func (s *Service) currentPurchaseTotal(ctx context.Context, reportingCurrency string, rates map[string]fxRate, now time.Time) (Decimal, int, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT ON (domain_id) amount::text,currency_code::text
		FROM domain_costs
		WHERE cost_type='purchase' AND effective_from <= current_date AND (effective_to IS NULL OR effective_to >= current_date)
		ORDER BY domain_id,CASE price_source WHEN 'registrar_api' THEN 1 WHEN 'google_sheet' THEN 2 WHEN 'manual' THEN 3 ELSE 4 END,effective_from DESC,created_at DESC
	`)
	if err != nil {
		return Decimal{}, 0, err
	}
	defer rows.Close()
	total := decimalFromInt(0)
	incomplete := 0
	for rows.Next() {
		var amount, currency string
		if err := rows.Scan(&amount, &currency); err != nil {
			return Decimal{}, 0, err
		}
		converted, ok := convertForSummary(amount, currency, reportingCurrency, rates, now, s.fxMaxAge)
		if !ok {
			incomplete++
			continue
		}
		total = total.Add(converted)
	}
	return total, incomplete, rows.Err()
}

func applicableWindows(now time.Time, expiration *time.Time) []string {
	if expiration == nil {
		return nil
	}
	date := expiration.UTC()
	if date.Before(now) {
		return nil
	}
	result := []string{}
	if !date.After(now.Add(30 * 24 * time.Hour)) {
		result = append(result, "next_30_days")
	}
	if !date.After(now.Add(60 * 24 * time.Hour)) {
		result = append(result, "next_60_days")
	}
	if !date.After(now.Add(90 * 24 * time.Hour)) {
		result = append(result, "next_90_days")
	}
	if date.Year() == now.Year() {
		result = append(result, "this_year")
	}
	return result
}

func normalizeCurrency(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}
