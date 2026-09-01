"use client";

import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import { useLocale } from "next-intl";
import { bffFetch } from "@/lib/api/client";
import { probeListSchema, type ProbeNode } from "@/lib/probes/types";

export function useProbes(): UseQueryResult<ProbeNode[]> {
  const locale = useLocale();
  return useQuery({
    queryKey: ["probes"],
    queryFn: async () => {
      const result = await bffFetch("/api/bff/probes", probeListSchema, { locale });
      return result.items;
    },
  });
}
