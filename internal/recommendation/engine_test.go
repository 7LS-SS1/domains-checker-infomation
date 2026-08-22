package recommendation

import (
	"strings"
	"testing"
	"time"
)

func healthyInput() Input {
	now := time.Now().UTC()
	expires := now.Add(30 * 24 * time.Hour)
	return Input{Domain: "brand.com", Lifecycle: "active", SourceStatus: "present", Priority: "high", Monitoring: true, Availability: "ACTIVE", DNS: "OK", HTTP: "OK", Redirect: "NONE", ISP: "NOT_DETECTED", TLS: "VALID", StatusConfidence: 90, LastCheckedAt: &now, ExpirationAt: &expires, RenewalAmount: "500.000000", RenewalCurrency: "THB"}
}

func TestEvaluateRuleOutcomes(t *testing.T) {
	now := time.Now().UTC()
	tests := []struct {
		name string
		edit func(*Input)
		want string
	}{
		{"renew", func(*Input) {}, "RENEW"},
		{"review incomplete", func(input *Input) { input.DNS = "UNKNOWN" }, "REVIEW"},
		{"review ISP", func(input *Input) { input.ISP = "HIGH_CONFIDENCE_BLOCK" }, "REVIEW"},
		{"drop conservative", func(input *Input) { input.Lifecycle, input.Priority, input.Redirect = "inactive", "low", "PERMANENT" }, "DROP"},
		{"profit indicators", func(input *Input) { input.Domain, input.Priority, input.ExpirationAt = "gold.com", "medium", nil }, "PROFIT_OPPORTUNITY"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := healthyInput()
			test.edit(&input)
			result := Evaluate(input, now)
			if result.Action != test.want || len(result.ReasonsTH) == 0 || len(result.ReasonsEN) == 0 || len(result.ReasonCodes) == 0 {
				t.Fatalf("result=%#v want action %s", result, test.want)
			}
			joined := strings.ToLower(strings.Join(append(result.ReasonsEN, result.ReasonsTH...), " "))
			if strings.Contains(joined, "valuation") || strings.Contains(joined, "มูลค่า") || strings.Contains(joined, "บาท") {
				t.Fatalf("recommendation invented a monetary valuation: %s", joined)
			}
		})
	}
}

func TestIncompleteEvidenceNeverDrops(t *testing.T) {
	input := healthyInput()
	input.Lifecycle, input.Priority, input.Redirect = "inactive", "low", "PERMANENT"
	input.LastCheckedAt = nil
	if result := Evaluate(input, time.Now().UTC()); result.Action != "REVIEW" {
		t.Fatalf("incomplete evidence action=%s, want REVIEW", result.Action)
	}
}
