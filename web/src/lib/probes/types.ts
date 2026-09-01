import { z } from "zod";

/** Mirrors internal/probe/model.go's Node struct. */
export const probeNodeSchema = z.object({
  id: z.string(),
  name: z.string(),
  region_code: z.string(),
  country_code: z.string(),
  network_name: z.string().nullish(),
  status: z.string(),
  version: z.string(),
  capabilities: z.unknown(),
  last_seen_at: z.string().nullish(),
  clock_offset_ms: z.number().nullish(),
  registered_at: z.string(),
  revoked_at: z.string().nullish(),
});

export const probeListSchema = z.object({ items: z.array(probeNodeSchema) });

/**
 * Mirrors internal/probe/model.go's RegistrationToken. `token` is the
 * plaintext registration secret — the backend returns it exactly once, on
 * creation, and never again. The UI must treat it as a secret-reveal-once
 * value: shown once, copyable, never logged or persisted client-side.
 */
export const probeRegistrationTokenSchema = z.object({
  id: z.string(),
  token: z.string(),
  name: z.string(),
  region_code: z.string(),
  country_code: z.string(),
  network_name: z.string().optional(),
  expires_at: z.string(),
});

export type ProbeNode = z.infer<typeof probeNodeSchema>;
export type ProbeRegistrationToken = z.infer<typeof probeRegistrationTokenSchema>;

/** probe_status enum — migrations/00002_monitoring_and_intelligence.sql line 4. */
export const PROBE_STATUSES = [
  "ONLINE",
  "DEGRADED",
  "OFFLINE",
  "REVOKED",
  "UPGRADE_REQUIRED",
] as const;
