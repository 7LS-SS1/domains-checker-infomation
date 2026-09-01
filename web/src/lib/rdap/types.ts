import { z } from "zod";

/** Mirrors internal/rdap/normalize.go's Normalized struct. */
const rdapNormalizedSchema = z.object({
  registrar_name: z.string().nullish(),
  registrar_iana_id: z.number().nullish(),
  registration_at: z.string().nullish(),
  expiration_at: z.string().nullish(),
  updated_at: z.string().nullish(),
  nameservers: z.array(z.string()),
  dnssec: z.boolean().nullish(),
  statuses: z.array(z.string()),
});

const rdapConflictSchema = z.object({
  field: z.string(),
  current_source: z.string(),
  current_value: z.unknown(),
  observed_source: z.string(),
  observed_value: z.unknown(),
});

/** Mirrors internal/rdap/service.go's Result struct. */
export const rdapResultSchema = z.object({
  id: z.string(),
  domain_id: z.string(),
  bootstrap_url: z.string(),
  rdap_url: z.string(),
  http_status: z.number().int().optional(),
  source_status: z.string(),
  confidence_score: z.number().int(),
  bootstrap_stale: z.boolean(),
  registration: rdapNormalizedSchema,
  response_headers: z.record(z.string(), z.string()),
  conflicts: z.array(rdapConflictSchema),
  error_code: z.string().optional(),
  error_message: z.string().optional(),
  checked_at: z.string(),
  cache_until: z.string().nullish(),
  cached: z.boolean(),
});

export type RdapResult = z.infer<typeof rdapResultSchema>;
export type RdapConflict = z.infer<typeof rdapConflictSchema>;
