import { z } from "zod";

/** Mirrors internal/domain/model.go's Domain struct exactly. */
export const domainSchema = z.object({
  id: z.string(),
  original_input: z.string(),
  domain_ascii: z.string(),
  domain_unicode: z.string(),
  registrable_domain: z.string(),
  registrar_id: z.string().nullish(),
  lifecycle_status: z.string(),
  source_status: z.string(),
  source_type: z.string(),
  business_priority: z.string(),
  monitoring_enabled: z.boolean(),
  expected_content_mode: z.string(),
  expiration_at: z.string().nullish(),
  notes: z.string(),
  renewal_decision: z.string().default("UNDECIDED"),
  renewal_decision_reason: z.string().default(""),
  renewal_decided_at: z.string().nullish(),
  renewal_price: z.string().nullish(),
  renewal_currency: z.string().nullish(),
  redirect_target_url: z.string().nullish(),
  latest_http_status_code: z.number().int().nullish(),
  availability_status: z.string(),
  dns_status: z.string(),
  http_status: z.string(),
  redirect_status: z.string(),
  isp_status: z.string(),
  tls_status: z.string(),
  content_status: z.string(),
  confidence_score: z.number().int(),
  consecutive_failures: z.number().int(),
  consecutive_successes: z.number().int(),
  current_failure_stage: z.string().nullish(),
  current_error_code: z.string().nullish(),
  last_checked_at: z.string().nullish(),
  archived_at: z.string().nullish(),
  created_at: z.string(),
  updated_at: z.string(),
  version: z.number(),
});

export const domainPageSchema = z.object({
  items: z.array(domainSchema),
  page: z.number().int(),
  page_size: z.number().int(),
  total_items: z.number(),
  total_pages: z.number().int(),
});

export type Domain = z.infer<typeof domainSchema>;
export type DomainPage = z.infer<typeof domainPageSchema>;

/** Mirrors internal/domain/model.go's Provenance struct. */
export const provenanceSchema = z.object({
  field_name: z.string(),
  value: z.unknown(),
  source: z.string(),
  source_reference: z.string(),
  observed_at: z.string(),
  is_current: z.boolean(),
});

export const provenanceListSchema = z.object({
  items: z.array(provenanceSchema),
});

export type Provenance = z.infer<typeof provenanceSchema>;

// --- Canonical enums, from web/API_CONTRACT_MATRIX.md section 9 / migrations ---
export const LIFECYCLE_STATUSES = ["active", "inactive", "archived"] as const;
export const SOURCE_STATUSES = ["present", "missing_from_source", "unknown"] as const;
export const BUSINESS_PRIORITIES = ["low", "medium", "high", "critical"] as const;
export const EXPECTED_CONTENT_MODES = ["HTML", "ANY", "STATUS_ONLY"] as const;
export const AVAILABILITY_STATUSES = ["ACTIVE", "UNAVAILABLE", "DEGRADED", "UNKNOWN"] as const;
export const DNS_STATUSES = [
  "OK",
  "NXDOMAIN",
  "SERVFAIL",
  "REFUSED",
  "TIMEOUT",
  "NETWORK_ERROR",
  "DISCREPANCY",
  "UNKNOWN",
] as const;
export const HTTP_STATUSES = [
  "OK",
  "REDIRECT",
  "CLIENT_ERROR",
  "SERVER_ERROR",
  "TIMEOUT",
  "CONNECTION_ERROR",
  "UNKNOWN",
] as const;
export const REDIRECT_STATUSES = [
  "NONE",
  "TEMPORARY",
  "PERMANENT",
  "LOOP",
  "INVALID",
  "HTTPS_DOWNGRADE",
  "UNKNOWN",
] as const;
export const ISP_STATUSES = [
  "NOT_DETECTED",
  "SUSPECTED",
  "HIGH_CONFIDENCE_BLOCK",
  "UNKNOWN",
] as const;
export const TLS_STATUSES = [
  "VALID",
  "EXPIRING",
  "EXPIRED",
  "HOSTNAME_MISMATCH",
  "INVALID",
  "ERROR",
  "NOT_APPLICABLE",
  "UNKNOWN",
] as const;
export const RENEWAL_DECISIONS = ["RENEW", "DO_NOT_RENEW", "HOLD", "UNDECIDED"] as const;
