import { describe, expect, it } from "vitest";
import { reportSummarySchema } from "@/lib/reports/types";

describe("reportSummarySchema (summary mapping)", () => {
  it("parses a realistic GET /api/v1/reports/summary payload", () => {
    const payload = {
      generated_at: "2026-08-29T10:00:00Z",
      reporting_currency: "THB",
      total_domains: 120,
      active_domains: 110,
      unavailable_domains: 3,
      permanent_redirect_domains: 5,
      suspected_isp_block: 2,
      high_confidence_isp_block: 1,
      dns_errors: 0,
      tls_errors: 1,
      expiring_within_90_days: 8,
      renewal_cost: "45230.500000",
      estimated_tax: "3166.135000",
      annual_budget: "180922.000000",
      recommended_renew: 60,
      recommended_drop: 4,
      review_required: 12,
      profit_opportunities: 3,
      finance_complete: false,
      completeness_warnings: ["FX_RATE_MISSING_OR_STALE"],
      recommendation_policy_version: "recommendation-2026-08-v1",
    };

    const result = reportSummarySchema.parse(payload);
    expect(result.total_domains).toBe(120);
    expect(result.renewal_cost).toBe("45230.500000");
    expect(result.finance_complete).toBe(false);
    expect(result.completeness_warnings).toEqual(["FX_RATE_MISSING_OR_STALE"]);
  });

  it("rejects a payload missing a required field", () => {
    const { total_domains: _omitted, ...incomplete } = {
      generated_at: "2026-08-29T10:00:00Z",
      reporting_currency: "THB",
      total_domains: 1,
      active_domains: 1,
      unavailable_domains: 0,
      permanent_redirect_domains: 0,
      suspected_isp_block: 0,
      high_confidence_isp_block: 0,
      dns_errors: 0,
      tls_errors: 0,
      expiring_within_90_days: 0,
      renewal_cost: "0",
      estimated_tax: "0",
      annual_budget: "0",
      recommended_renew: 0,
      recommended_drop: 0,
      review_required: 0,
      profit_opportunities: 0,
      finance_complete: true,
      completeness_warnings: [],
      recommendation_policy_version: "v1",
    };
    expect(reportSummarySchema.safeParse(incomplete).success).toBe(false);
  });
});
