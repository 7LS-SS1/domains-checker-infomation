"use client";

import { useMutation, useQuery, useQueryClient, type UseQueryResult } from "@tanstack/react-query";
import { useLocale } from "next-intl";
import { z } from "zod";
import { bffFetch } from "@/lib/api/client";
import {
  recommendationListSchema,
  recommendationSchema,
  type Recommendation,
} from "@/lib/recommendations/types";

export interface RecommendationListFilters {
  action: string;
  limit: number;
}

export function useRecommendationsList(
  filters: RecommendationListFilters,
): UseQueryResult<Recommendation[]> {
  const locale = useLocale();
  return useQuery({
    queryKey: ["recommendations", filters],
    queryFn: async () => {
      const params = new URLSearchParams();
      if (filters.action) params.set("action", filters.action);
      params.set("limit", String(filters.limit));
      const result = await bffFetch(
        `/api/bff/recommendations?${params.toString()}`,
        recommendationListSchema,
        { locale },
      );
      return result.items;
    },
  });
}

const runResponseSchema = z.object({
  items: z.array(recommendationSchema),
  count: z.number().int(),
  policy_version: z.string(),
});

export function useRunRecommendations() {
  const locale = useLocale();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (limit: number) =>
      bffFetch("/api/bff/recommendations/run", runResponseSchema, {
        method: "POST",
        body: JSON.stringify({ limit }),
        locale,
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["recommendations"] });
    },
  });
}
