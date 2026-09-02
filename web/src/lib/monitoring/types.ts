import { z } from "zod";

/** Mirrors internal/monitor/model.go's Run struct. */
export const monitoringRunSchema = z.object({
  id: z.string(),
  domain_id: z.string(),
  domain_ascii: z.string().optional(),
  trigger_type: z.string(),
  status: z.string(),
  priority: z.string(),
  deduplication_key: z.string().optional(),
  policy_version: z.string(),
  policy_snapshot: z.unknown().optional(),
  requested_by: z.string().nullish(),
  scheduled_for: z.string(),
  deadline_at: z.string(),
  started_at: z.string().nullish(),
  completed_at: z.string().nullish(),
  last_error_code: z.string().nullish(),
  last_error_message: z.string().nullish(),
  execution_attempts: z.number().int(),
  created_at: z.string(),
  updated_at: z.string(),
});

export const monitoringRunPageSchema = z.object({
  items: z.array(monitoringRunSchema),
  page: z.number().int(),
  page_size: z.number().int(),
  total_items: z.number(),
  total_pages: z.number().int(),
});

const dnsAnswerSchema = z.object({
  name: z.string(),
  type: z.string(),
  value: z.string(),
  ttl_seconds: z.number(),
});

const dnsCheckSchema = z.object({
  id: z.string(),
  monitoring_result_id: z.string(),
  resolver_type: z.string(),
  resolver_endpoint: z.string(),
  query_name: z.string(),
  query_type: z.string(),
  attempt: z.number().int(),
  rcode: z.string().nullish(),
  truncated: z.boolean(),
  authoritative: z.boolean(),
  duration_us: z.number(),
  error_code: z.string().nullish(),
  error_message: z.string().nullish(),
  checked_at: z.string(),
  answers: z.array(dnsAnswerSchema),
});

const redirectHopSchema = z.object({
  hop: z.number().int(),
  source_url: z.string(),
  status_code: z.number().int(),
  location: z.string(),
  resolved_target: z.string().nullish(),
  cross_domain: z.boolean(),
  https_downgrade: z.boolean(),
  duration_us: z.number(),
  error_code: z.string().nullish(),
});

const tlsCheckSchema = z.object({
  server_name: z.string(),
  remote_address: z.string().nullish(),
  tls_version: z.string().nullish(),
  cipher_suite: z.string().nullish(),
  certificate_subject: z.string().nullish(),
  certificate_issuer: z.string().nullish(),
  valid_from: z.string().nullish(),
  valid_until: z.string().nullish(),
  certificate_expiration_days: z.number().int().nullish(),
  hostname_valid: z.boolean().nullish(),
  chain_valid: z.boolean().nullish(),
  tls_status: z.string(),
  diagnostic_only: z.boolean(),
  error_code: z.string().nullish(),
  error_message: z.string().nullish(),
  checked_at: z.string(),
});

const httpCheckSchema = z.object({
  id: z.string(),
  monitoring_result_id: z.string(),
  scheme: z.string(),
  resolver_mode: z.string(),
  request_url: z.string(),
  effective_url: z.string().nullish(),
  protocol: z.string().nullish(),
  attempt: z.number().int(),
  initial_status_code: z.number().int().nullish(),
  final_status_code: z.number().int().nullish(),
  total_duration_us: z.number(),
  content_type: z.string().nullish(),
  declared_content_length: z.number().nullish(),
  body_size: z.number(),
  hash_complete: z.boolean(),
  title: z.string().nullish(),
  content_status: z.string(),
  server_header: z.string().nullish(),
  error_code: z.string().nullish(),
  error_message: z.string().nullish(),
  checked_at: z.string(),
  redirects: z.array(redirectHopSchema),
  tls_check: tlsCheckSchema.nullish(),
});

const monitoringResultSchema = z.object({
  id: z.string(),
  monitoring_run_id: z.string(),
  domain_id: z.string(),
  vantage_type: z.string(),
  vantage_key: z.string(),
  vantage_country: z.string().nullish(),
  vantage_network: z.string().nullish(),
  availability_status: z.string(),
  dns_status: z.string(),
  http_status: z.string(),
  redirect_status: z.string(),
  isp_status: z.string(),
  tls_status: z.string(),
  content_status: z.string(),
  initial_http_status_code: z.number().int().nullish(),
  final_http_status_code: z.number().int().nullish(),
  final_target_status: z.string().nullish(),
  failure_stage: z.string().nullish(),
  error_code: z.string().nullish(),
  error_message: z.string().nullish(),
  confidence_score: z.number().int(),
  confidence_level: z.string(),
  reason_codes: z.array(z.string()).default([]),
  policy_version: z.string(),
  checked_at: z.string(),
  completed_at: z.string().nullish(),
});

export const monitoringRunDetailSchema = z.object({
  run: monitoringRunSchema,
  results: z.array(monitoringResultSchema),
  dns_checks: z.array(dnsCheckSchema),
  http_checks: z.array(httpCheckSchema),
});

const historyEntrySchema = z.object({
  id: z.string(),
  dimension: z.string(),
  previous_value: z.string().nullish(),
  current_value: z.string(),
  confidence_score: z.number().int(),
  policy_version: z.string(),
  reason_codes: z.unknown(),
  supporting_run_ids: z.array(z.string()),
  effective_at: z.string(),
});

const historyAggregateSchema = z.object({
  window_start: z.string(),
  window_end: z.string(),
  uptime_percentage: z.number().nullish(),
  monitoring_coverage: z.number(),
  known_seconds: z.number(),
  active_seconds: z.number(),
  degraded_seconds: z.number(),
  unavailable_seconds: z.number(),
  status_change_count: z.number().int(),
  incident_count: z.number().int(),
  average_response_ms: z.number().nullish(),
});

export const monitoringHistorySchema = z.object({
  domain_id: z.string(),
  timeline: z.array(historyEntrySchema),
  aggregate: historyAggregateSchema,
});

export type MonitoringRun = z.infer<typeof monitoringRunSchema>;
export type MonitoringRunPage = z.infer<typeof monitoringRunPageSchema>;
export type MonitoringRunDetail = z.infer<typeof monitoringRunDetailSchema>;
export type MonitoringHistory = z.infer<typeof monitoringHistorySchema>;
export type HistoryAggregate = z.infer<typeof historyAggregateSchema>;

export const MONITORING_RUN_STATUSES = [
  "queued",
  "running",
  "partial",
  "completed",
  "failed",
  "cancelled",
] as const;

export const HISTORY_WINDOWS = ["24h", "7d", "30d", "90d"] as const;
export type HistoryWindow = (typeof HISTORY_WINDOWS)[number];
