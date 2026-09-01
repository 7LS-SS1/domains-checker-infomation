import { z } from "zod";

/** Mirrors internal/monitor/model.go's Incident/IncidentPage structs exactly. */
export const incidentSchema = z.object({
  id: z.string(),
  domain_id: z.string(),
  domain_ascii: z.string(),
  status: z.string(),
  failure_stage: z.string().nullish(),
  error_code: z.string().nullish(),
  open_failure_count: z.number().int(),
  close_success_count: z.number().int(),
  opened_at: z.string(),
  closed_at: z.string().nullish(),
  opened_by_run_id: z.string(),
  closed_by_run_id: z.string().nullish(),
});

export const incidentPageSchema = z.object({
  items: z.array(incidentSchema),
  page: z.number().int(),
  page_size: z.number().int(),
  total_items: z.number(),
  total_pages: z.number().int(),
});

export type Incident = z.infer<typeof incidentSchema>;
export type IncidentPage = z.infer<typeof incidentPageSchema>;

/** incident_status enum from migrations/00002_monitoring_and_intelligence.sql */
export const INCIDENT_STATUSES = ["open", "acknowledged", "closed"] as const;
