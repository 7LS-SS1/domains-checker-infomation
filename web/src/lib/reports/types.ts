import { z } from "zod";

/** Mirrors internal/report/service.go's Summary struct exactly. */
export const reportSummarySchema = z.object({
  generated_at: z.string(),
  reporting_currency: z.string(),
  total_domains: z.number().int(),
  active_domains: z.number().int(),
  unavailable_domains: z.number().int(),
  permanent_redirect_domains: z.number().int(),
  suspected_isp_block: z.number().int(),
  high_confidence_isp_block: z.number().int(),
  dns_errors: z.number().int(),
  tls_errors: z.number().int(),
  expiring_within_90_days: z.number().int(),
  renewal_cost: z.string(),
  estimated_tax: z.string(),
  annual_budget: z.string(),
  recommended_renew: z.number().int(),
  recommended_drop: z.number().int(),
  review_required: z.number().int(),
  profit_opportunities: z.number().int(),
  finance_complete: z.boolean(),
  completeness_warnings: z.array(z.string()),
  recommendation_policy_version: z.string(),
});

export type ReportSummary = z.infer<typeof reportSummarySchema>;

/** Mirrors internal/report/service.go's Record struct. */
export const reportRecordSchema = z.object({
  id: z.string(),
  report_type: z.string(),
  format: z.string(),
  status: z.string(),
  filters: z.unknown(),
  snapshot_at: z.string(),
  policy_versions: z.unknown(),
  completeness_warnings: z.array(z.string()),
  row_count: z.number(),
  storage_reference: z.string(),
  sha256: z.string(),
  requested_by: z.string(),
  requested_at: z.string(),
  completed_at: z.string().nullish(),
});

export type ReportRecord = z.infer<typeof reportRecordSchema>;

export const REPORT_FORMATS = ["json", "csv"] as const;
