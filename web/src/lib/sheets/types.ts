import { z } from "zod";

const normalizedRowSchema = z.object({
  domain: z.string(),
  domain_unicode: z.string(),
  registrable_domain: z.string(),
  registrar: z.string().optional(),
  purchase_price: z.string().nullish(),
  renewal_price: z.string().nullish(),
  currency: z.string().optional(),
  tax_rate: z.string().nullish(),
  purchase_date: z.string().nullish(),
  expiration_date: z.string().nullish(),
  business_priority: z.string(),
  notes: z.string(),
  active: z.boolean(),
});

/** Mirrors internal/sheets/model.go's ImportRow. */
export const importRowSchema = z.object({
  id: z.string(),
  row_number: z.number().int(),
  matched_domain_id: z.string().nullish(),
  action: z.string(),
  valid: z.boolean(),
  raw_values: z.record(z.string(), z.string()),
  normalized_values: normalizedRowSchema.nullish(),
  validation_errors: z.array(z.string()),
  diff: z.unknown(),
});

/** Mirrors internal/sheets/model.go's Import. */
export const sheetImportSchema = z.object({
  id: z.string(),
  config_id: z.string().nullish(),
  spreadsheet_id: z.string(),
  sheet_name: z.string(),
  status: z.string(),
  trigger_type: z.string(),
  source_kind: z.string(),
  source_metadata: z.unknown(),
  source_revision: z.string(),
  source_hash: z.string(),
  column_mapping: z.record(z.string(), z.string()),
  total_rows: z.number().int(),
  added_count: z.number().int(),
  modified_count: z.number().int(),
  unchanged_count: z.number().int(),
  missing_count: z.number().int(),
  invalid_count: z.number().int(),
  valid_rows_applied: z.number().int(),
  requested_by: z.string().nullish(),
  previewed_at: z.string(),
  applied_at: z.string().nullish(),
  rejected_at: z.string().nullish(),
  rows: z.array(importRowSchema).optional(),
});

export const sheetImportListSchema = z.object({ items: z.array(sheetImportSchema) });

/** Mirrors internal/sheets/model.go's Config. */
export const sheetConfigSchema = z.object({
  id: z.string(),
  connection_id: z.string().nullish(),
  spreadsheet_id: z.string(),
  sheet_name: z.string(),
  range: z.string(),
  column_mapping: z.record(z.string(), z.string()),
  enabled: z.boolean(),
  sync_interval_minutes: z.number().int(),
  next_sync_at: z.string().nullish(),
  last_sync_at: z.string().nullish(),
  updated_by: z.string(),
  created_at: z.string(),
  updated_at: z.string(),
  version: z.number(),
});

export type ImportRow = z.infer<typeof importRowSchema>;
export type SheetImport = z.infer<typeof sheetImportSchema>;
export type SheetConfig = z.infer<typeof sheetConfigSchema>;

export const IMPORT_ROW_ACTIONS = ["ADD", "MODIFY", "UNCHANGED", "MISSING", "INVALID"] as const;
export const SHEET_IMPORT_STATUSES = [
  "preview",
  "applying",
  "applied",
  "rejected",
  "failed",
] as const;
