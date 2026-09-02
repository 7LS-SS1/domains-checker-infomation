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

export const REPORT_FORMATS = ["json", "csv", "pdf"] as const;

/** Mirrors internal/report/service.go's StatusCount struct. */
export const statusCountSchema = z.object({
  status: z.string(),
  count: z.number().int(),
});
export type StatusCount = z.infer<typeof statusCountSchema>;

/** Mirrors internal/report/service.go's DailyCount struct. */
export const dailyCountSchema = z.object({
  date: z.string(),
  count: z.number().int(),
});
export type DailyCount = z.infer<typeof dailyCountSchema>;

/** Mirrors internal/report/service.go's TopDomain struct. */
export const topDomainSchema = z.object({
  domain: z.string(),
  availability_status: z.string(),
  isp_status: z.string(),
  recommendation: z.string(),
  renewal_cost: z.string().nullish(),
  renewal_cost_currency: z.string().nullish(),
});
export type TopDomain = z.infer<typeof topDomainSchema>;

/** Mirrors internal/report/service.go's Dashboard struct (Summary + charts). */
export const reportDashboardSchema = reportSummarySchema.extend({
  availability_distribution: z.array(statusCountSchema),
  isp_distribution: z.array(statusCountSchema),
  incident_trend_30d: z.array(dailyCountSchema),
  top_domains: z.array(topDomainSchema),
});
export type ReportDashboard = z.infer<typeof reportDashboardSchema>;
