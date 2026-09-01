import { z } from "zod";

/** Mirrors GET /health's writeData payload — internal/api/server.go health(). */
export const healthSchema = z.object({
  status: z.string(),
  service: z.string(),
  version: z.string(),
  commit: z.string(),
  started_at: z.string(),
});

/** Mirrors GET /ready's writeData payload — internal/api/server.go ready(). */
export const readySchema = z.object({
  status: z.string(),
  checks: z.record(z.string(), z.string()),
});

/** Mirrors GET /api/v1/meta/locales's writeData payload — internal/api/server.go locales(). */
export const localesMetaSchema = z.object({
  default: z.string(),
  supported: z.array(z.string()),
});

export type Health = z.infer<typeof healthSchema>;
export type Ready = z.infer<typeof readySchema>;
export type LocalesMeta = z.infer<typeof localesMetaSchema>;
