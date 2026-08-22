package finance

import (
	"testing"
	"time"
)

func stringPointer(value string) *string { return &value }

func TestDecimalArithmeticIsExact(t *testing.T) {
	one, _ := ParseDecimal("0.1", 6)
	two, _ := ParseDecimal("0.2", 6)
	if got := one.Add(two).String(6); got != "0.300000" {
		t.Fatalf("0.1 + 0.2 = %s", got)
	}
}

func TestCalculateTaxModesAndBillingCycles(t *testing.T) {
	tests := []struct {
		name, mode, total, tax, annual string
	}{
		{name: "exclusive annual", mode: TaxExclusive, total: "107.000000", tax: "7.000000", annual: "107.000000"},
		{name: "inclusive six months", mode: TaxInclusive, total: "100.000000", tax: "6.542056", annual: "200.000000"},
		{name: "exempt", mode: TaxExempt, total: "100.000000", tax: "0.000000", annual: "100.000000"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			months := 12
			var rate *string
			if test.mode != TaxExempt {
				rate = stringPointer("0.07")
			}
			if test.name == "inclusive six months" {
				months = 6
			}
			result, err := Calculate(CostInput{Amount: "100", Currency: "THB", TaxRate: rate, TaxMode: test.mode, BillingCycleMonths: months})
			if err != nil {
				t.Fatal(err)
			}
			if !result.Complete || *result.CycleTotal != test.total || *result.TaxAmount != test.tax || *result.AnnualEstimate != test.annual {
				t.Fatalf("unexpected result: %#v", result)
			}
		})
	}
}

func TestMissingTaxAndStaleFXStayIncomplete(t *testing.T) {
	result, err := Calculate(CostInput{Amount: "100", Currency: "USD", TaxMode: TaxUnknown, BillingCycleMonths: 12})
	if err != nil || result.Complete || len(result.Warnings) != 1 || result.Warnings[0] != "TAX_POLICY_UNKNOWN" {
		t.Fatalf("unexpected tax result: %#v, err=%v", result, err)
	}
	now := time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC)
	fx, err := Convert(FXInput{Amount: "10", Rate: "35.5", FromCurrency: "USD", ToCurrency: "THB", ObservedAt: now.Add(-49 * time.Hour), Now: now, MaxAge: 48 * time.Hour})
	if err != nil || fx.Complete || fx.ConvertedAmount != nil || fx.Warnings[0] != "FX_RATE_STALE" {
		t.Fatalf("unexpected FX result: %#v, err=%v", fx, err)
	}
}
