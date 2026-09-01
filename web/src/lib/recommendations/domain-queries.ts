"use client";

import { useMutation, useQuery, useQueryClient, type UseQueryResult } from "@tanstack/react-query";
import { useLocale } from "next-intl";
import { bffFetch } from "@/lib/api/client";
import { recommendationSchema, type Recommendation } from "@/lib/recommendations/types";

export function useDomainRecommendation(
  domainId: string | undefined,
): UseQueryResult<Recommendation> {
  const locale = useLocale();
  return useQuery({
    queryKey: ["domain", domainId, "recommendation"],
    queryFn: () =>
      bffFetch(`/api/bff/domains/${domainId}/recommendation`, recommendationSchema, { locale }),
    enabled: Boolean(domainId),
    retry: false,
  });
}

export function useGenerateRecommendation() {
  const locale = useLocale();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (domainId: string) =>
      bffFetch(`/api/bff/domains/${domainId}/recommendation`, recommendationSchema, {
        method: "POST",
        locale,
      }),
    onSuccess: (_data, domainId) => {
      void queryClient.invalidateQueries({ queryKey: ["domain", domainId, "recommendation"] });
    },
  });
}
