package report

import (
	"encoding/csv"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestRenderSummaryJSONAndCSV(t *testing.T) {
	summary := Summary{GeneratedAt: time.Date(2026, 8, 22, 10, 0, 0, 0, time.UTC), ReportingCurrency: "THB", TotalDomains: 3, ActiveDomains: 2, RenewalCost: "300.000000", EstimatedTax: "21.000000", AnnualBudget: "321.000000", RecommendedRenew: 1, RecommendedDrop: 1, ReviewRequired: 1, RecommendationPolicy: "recommendation-2026-08-v1"}
	jsonContent, contentType, err := render(summary, "json")
	if err != nil || contentType != "application/json" || !json.Valid(jsonContent) || !strings.Contains(string(jsonContent), `"renewal_cost": "300.000000"`) {
		t.Fatalf("invalid JSON report: type=%s err=%v content=%s", contentType, err, jsonContent)
	}
	csvContent, contentType, err := render(summary, "csv")
	if err != nil || contentType != "text/csv" {
		t.Fatalf("CSV render: type=%s err=%v", contentType, err)
	}
	rows, err := csv.NewReader(strings.NewReader(string(csvContent))).ReadAll()
	if err != nil || len(rows) != 2 || len(rows[0]) != len(rows[1]) || rows[1][2] != "3" {
		t.Fatalf("invalid CSV rows=%#v err=%v", rows, err)
	}
}
