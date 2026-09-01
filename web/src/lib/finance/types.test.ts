import { describe, expect, it } from "vitest";
import {
  financeSummarySchema,
  BUDGET_WINDOW_KEYS,
  FINANCE_WARNING_CODES,
} from "@/lib/finance/types";

describe("financeSummarySchema (summary mapping, incomplete/warning handling)", () => {
  it("parses a complete summary with no warnings", () => {
    const payload = {
      reporting_currency: "THB",
      generated_at: "2026-08-29T10:00:00Z",
      total_current_domain_cost: "120000.000000",
      total_renewal_cost: "45230.500000",
      estimated_tax: "3166.135000",
      total_annual_budget: "180922.000000",
      complete: true,
      unknown_cost_count: 0,
      unknown_tax_count: 0,
      fx_incomplete_count: 0,
      warnings: [],
      windows: {
        next_30_days: {
          domain_count: 4,
          known_renewals: 4,
          unknown_costs: 0,
          renewal_total: "5000.00",
        },
        next_60_days: {
          domain_count: 6,
          known_renewals: 6,
          unknown_costs: 0,
          renewal_total: "7000.00",
        },
        next_90_days: {
          domain_count: 9,
          known_renewals: 9,
          unknown_costs: 0,
          renewal_total: "9000.00",
        },
        this_year: {
          domain_count: 40,
          known_renewals: 38,
          unknown_costs: 2,
          renewal_total: "40000.00",
        },
      },
    };

    const result = financeSummarySchema.parse(payload);
    expect(result.complete).toBe(true);
    expect(result.windows.next_30_days?.renewal_total).toBe("5000.00");
    for (const key of BUDGET_WINDOW_KEYS) {
      expect(result.windows[key]).toBeDefined();
    }
  });

  it("parses an incomplete summary carrying every known warning code", () => {
    const payload = {
      reporting_currency: "USD",
      generated_at: "2026-08-29T10:00:00Z",
      total_current_domain_cost: "0.000000",
      total_renewal_cost: "0.000000",
      estimated_tax: "0.000000",
      total_annual_budget: "0.000000",
      complete: false,
      unknown_cost_count: 3,
      unknown_tax_count: 1,
      fx_incomplete_count: 2,
      warnings: [...FINANCE_WARNING_CODES],
      windows: {},
    };

    const result = financeSummarySchema.parse(payload);
    expect(result.complete).toBe(false);
    expect(result.warnings).toEqual([
      "TAX_POLICY_UNKNOWN",
      "FX_RATE_MISSING_OR_STALE",
      "INVALID_COST_RECORD",
      "PURCHASE_TOTAL_INCOMPLETE",
    ]);
  });

  it("keeps money fields as strings, never numbers, after parsing", () => {
    const payload = {
      reporting_currency: "THB",
      generated_at: "2026-08-29T10:00:00Z",
      total_current_domain_cost: "90071992547409999999.123456",
      total_renewal_cost: "0.000000",
      estimated_tax: "0.000000",
      total_annual_budget: "0.000000",
      complete: true,
      unknown_cost_count: 0,
      unknown_tax_count: 0,
      fx_incomplete_count: 0,
      warnings: [],
      windows: {},
    };
    const result = financeSummarySchema.parse(payload);
    expect(typeof result.total_current_domain_cost).toBe("string");
    expect(result.total_current_domain_cost).toBe("90071992547409999999.123456");
  });
});
