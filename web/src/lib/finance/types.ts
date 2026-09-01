import { z } from "zod";

/**
 * Mirrors internal/finance/service.go's Summary/BudgetWindow structs
 * exactly (see web/API_CONTRACT_MATRIX.md section 5). Every money field is
 * a decimal string — never coerce these with Number()/parseFloat.
 */
export const budgetWindowSchema = z.object({
  domain_count: z.number().int(),
  known_renewals: z.number().int(),
  unknown_costs: z.number().int(),
  renewal_total: z.string(),
});

export const financeSummarySchema = z.object({
  reporting_currency: z.string(),
  generated_at: z.string(),
  total_current_domain_cost: z.string(),
  total_renewal_cost: z.string(),
  estimated_tax: z.string(),
  total_annual_budget: z.string(),
  complete: z.boolean(),
  unknown_cost_count: z.number().int(),
  unknown_tax_count: z.number().int(),
  fx_incomplete_count: z.number().int(),
  warnings: z.array(z.string()),
  windows: z.record(z.string(), budgetWindowSchema),
});

export type BudgetWindow = z.infer<typeof budgetWindowSchema>;
export type FinanceSummary = z.infer<typeof financeSummarySchema>;

/** The four known finance-summary window keys, in display order. */
export const BUDGET_WINDOW_KEYS = [
  "next_30_days",
  "next_60_days",
  "next_90_days",
  "this_year",
] as const;

/** Mirrors internal/finance/service.go's CostRecord (calculation nested). */
const costCalculationSchema = z.object({
  cycle_amount: z.string().optional(),
  currency: z.string().optional(),
  billing_cycle_months: z.number().int().optional(),
  cycle_total: z.string().nullish(),
  tax_amount: z.string().nullish(),
  annual_estimate: z.string().nullish(),
  complete: z.boolean().optional(),
});

export const costRecordSchema = z.object({
  id: z.string(),
  domain_id: z.string(),
  cost_type: z.string(),
  amount: z.string(),
  currency: z.string(),
  price_source: z.string(),
  tax_rate: z.string().nullish(),
  tax_mode: z.string(),
  billing_cycle_months: z.number().int(),
  effective_from: z.string(),
  effective_to: z.string().nullish(),
  source_reference: z.string(),
  calculation: costCalculationSchema.optional(),
});

export const costRecordListSchema = z.object({ items: z.array(costRecordSchema) });

/** Mirrors internal/finance/service.go's RateRecord. */
export const rateRecordSchema = z.object({
  id: z.string(),
  base_currency: z.string(),
  quote_currency: z.string(),
  rate: z.string(),
  source: z.string(),
  observed_at: z.string(),
});

export type RateRecord = z.infer<typeof rateRecordSchema>;

/** Mirrors internal/finance/service.go's OverrideRecord. */
export const overrideRecordSchema = z.object({
  id: z.string(),
  domain_id: z.string(),
  field_name: z.string(),
  original_value: z.unknown(),
  override_value: z.unknown(),
  reason: z.string(),
  created_by: z.string(),
  effective_from: z.string(),
  expires_at: z.string().nullish(),
  revoked_at: z.string().nullish(),
});

export const overrideRecordListSchema = z.object({ items: z.array(overrideRecordSchema) });

export type CostRecord = z.infer<typeof costRecordSchema>;
export type OverrideRecord = z.infer<typeof overrideRecordSchema>;

export const COST_TYPES = ["purchase", "renewal"] as const;
export const TAX_MODES = ["exclusive", "inclusive", "exempt", "unknown"] as const;
export const OVERRIDE_FIELDS = [
  "recommendation",
  "renewal_price",
  "tax_rate",
  "expiration_date",
  "business_priority",
] as const;

/**
 * The backend never localizes these completeness-warning codes (they ride
 * inside a 200 `data` payload, not the `error` envelope, so
 * internal/i18n/i18n.go's catalog does not cover them). Source:
 * internal/finance/summary.go lines emitting
 * TAX_POLICY_UNKNOWN / FX_RATE_MISSING_OR_STALE / INVALID_COST_RECORD /
 * PURCHASE_TOTAL_INCOMPLETE.
 */
export const FINANCE_WARNING_CODES = [
  "TAX_POLICY_UNKNOWN",
  "FX_RATE_MISSING_OR_STALE",
  "INVALID_COST_RECORD",
  "PURCHASE_TOTAL_INCOMPLETE",
] as const;
