import { z } from "zod";

const overrideSchema = z.object({
  id: z.string(),
  action: z.string(),
  reason: z.string(),
  created_by: z.string(),
  created_at: z.string(),
});

/** Mirrors internal/recommendation/service.go's Record struct exactly. */
export const recommendationSchema = z.object({
  id: z.string(),
  domain_id: z.string(),
  domain: z.string(),
  action: z.string(),
  effective_action: z.string(),
  opportunity_level: z.string(),
  confidence_score: z.number().int(),
  confidence_level: z.string(),
  policy_version: z.string(),
  reason_codes: z.array(z.string()),
  reasons_th: z.array(z.string()),
  reasons_en: z.array(z.string()),
  evidence_refs: z.array(z.string()),
  supersedes_id: z.string().nullish(),
  generated_at: z.string(),
  manual_override: overrideSchema.nullish(),
});

export const recommendationListSchema = z.object({
  items: z.array(recommendationSchema),
});

export type Recommendation = z.infer<typeof recommendationSchema>;

/** recommendation_action enum (migrations 00002 + 00006 ALTER TYPE). */
export const RECOMMENDATION_ACTIONS = ["RENEW", "DROP", "REVIEW", "PROFIT_OPPORTUNITY"] as const;
export type RecommendationAction = (typeof RECOMMENDATION_ACTIONS)[number];
