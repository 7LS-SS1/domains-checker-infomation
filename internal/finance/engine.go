package finance

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	TaxExclusive = "exclusive"
	TaxInclusive = "inclusive"
	TaxExempt    = "exempt"
	TaxUnknown   = "unknown"
)

type CostInput struct {
	Amount             string
	Currency           string
	TaxRate            *string
	TaxMode            string
	BillingCycleMonths int
}

type CostResult struct {
	Currency           string   `json:"currency"`
	CycleAmount        string   `json:"cycle_amount"`
	TaxAmount          *string  `json:"tax_amount"`
	CycleTotal         *string  `json:"cycle_total"`
	AnnualEstimate     *string  `json:"annual_estimate"`
	BillingCycleMonths int      `json:"billing_cycle_months"`
	Formula            string   `json:"formula"`
	Complete           bool     `json:"complete"`
	Warnings           []string `json:"warnings"`
}

func Calculate(input CostInput) (CostResult, error) {
	amount, err := ParseDecimal(input.Amount, 6)
	if err != nil || amount.Sign() < 0 {
		return CostResult{}, errors.New("amount must be a non-negative decimal string with at most 6 decimal places")
	}
	currency := strings.ToUpper(strings.TrimSpace(input.Currency))
	if len(currency) != 3 {
		return CostResult{}, errors.New("currency must be a three-letter code")
	}
	months := input.BillingCycleMonths
	if months == 0 {
		months = 12
	}
	if months < 1 || months > 120 {
		return CostResult{}, errors.New("billing cycle must be between 1 and 120 months")
	}
	mode := strings.ToLower(strings.TrimSpace(input.TaxMode))
	if mode == "" {
		mode = TaxUnknown
	}
	result := CostResult{
		Currency: currency, CycleAmount: amount.String(6), BillingCycleMonths: months,
		Formula: fmt.Sprintf("cycle_total * (12 / %d)", months), Warnings: []string{},
	}
	if mode == TaxUnknown {
		if input.TaxRate != nil {
			return CostResult{}, errors.New("tax_rate must be null when tax_mode is unknown")
		}
		result.Warnings = append(result.Warnings, "TAX_POLICY_UNKNOWN")
		return result, nil
	}
	if mode == TaxExempt {
		if input.TaxRate != nil {
			rate, rateErr := ParseDecimal(*input.TaxRate, 10)
			if rateErr != nil || rate.Sign() != 0 {
				return CostResult{}, errors.New("tax_rate must be null or zero when tax_mode is exempt")
			}
		}
		zero := decimalFromInt(0).String(6)
		total := amount.String(6)
		annual := amount.Mul(decimalFromInt(12)).Quo(decimalFromInt(int64(months))).String(6)
		result.TaxAmount, result.CycleTotal, result.AnnualEstimate = &zero, &total, &annual
		result.Complete = true
		return result, nil
	}
	if mode != TaxExclusive && mode != TaxInclusive {
		return CostResult{}, errors.New("tax_mode must be exclusive, inclusive, exempt, or unknown")
	}
	if input.TaxRate == nil {
		return CostResult{}, errors.New("tax_rate is required for inclusive or exclusive tax")
	}
	rate, err := ParseDecimal(*input.TaxRate, 10)
	if err != nil || rate.Sign() < 0 || rate.value.Cmp(decimalFromInt(1).value) > 0 {
		return CostResult{}, errors.New("tax_rate must be between 0 and 1 with at most 10 decimal places")
	}
	var tax, total Decimal
	if mode == TaxExclusive {
		tax = amount.Mul(rate)
		total = amount.Add(tax)
	} else {
		base := amount.Quo(decimalFromInt(1).Add(rate))
		tax = amount.Sub(base)
		total = amount
	}
	taxString := tax.String(6)
	totalString := total.String(6)
	annualString := total.Mul(decimalFromInt(12)).Quo(decimalFromInt(int64(months))).String(6)
	result.TaxAmount, result.CycleTotal, result.AnnualEstimate = &taxString, &totalString, &annualString
	result.Complete = true
	return result, nil
}

type FXInput struct {
	Amount       string
	Rate         string
	FromCurrency string
	ToCurrency   string
	ObservedAt   time.Time
	Now          time.Time
	MaxAge       time.Duration
}

type FXResult struct {
	OriginalAmount    string     `json:"original_amount"`
	OriginalCurrency  string     `json:"original_currency"`
	ConvertedAmount   *string    `json:"converted_amount"`
	ReportingCurrency string     `json:"reporting_currency"`
	Rate              *string    `json:"exchange_rate"`
	RateObservedAt    *time.Time `json:"exchange_rate_timestamp"`
	Complete          bool       `json:"complete"`
	Warnings          []string   `json:"warnings"`
}

func Convert(input FXInput) (FXResult, error) {
	amount, err := ParseDecimal(input.Amount, 6)
	if err != nil || amount.Sign() < 0 {
		return FXResult{}, errors.New("amount must be a non-negative decimal string")
	}
	from := strings.ToUpper(strings.TrimSpace(input.FromCurrency))
	to := strings.ToUpper(strings.TrimSpace(input.ToCurrency))
	result := FXResult{OriginalAmount: amount.String(6), OriginalCurrency: from, ReportingCurrency: to, Warnings: []string{}}
	if from == to {
		converted := amount.String(6)
		result.ConvertedAmount = &converted
		result.Complete = true
		return result, nil
	}
	rate, err := ParseDecimal(input.Rate, 10)
	if err != nil || rate.Sign() <= 0 {
		result.Warnings = append(result.Warnings, "FX_RATE_MISSING")
		return result, nil
	}
	now := input.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if input.ObservedAt.IsZero() || input.MaxAge <= 0 || now.Sub(input.ObservedAt) > input.MaxAge || input.ObservedAt.After(now.Add(5*time.Minute)) {
		result.Warnings = append(result.Warnings, "FX_RATE_STALE")
		return result, nil
	}
	converted := amount.Mul(rate).String(6)
	rateString := rate.String(10)
	observed := input.ObservedAt.UTC()
	result.ConvertedAmount, result.Rate, result.RateObservedAt = &converted, &rateString, &observed
	result.Complete = true
	return result, nil
}
