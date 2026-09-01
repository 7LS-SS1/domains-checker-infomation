"use client";

import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import { useLocale } from "next-intl";
import { bffFetch } from "@/lib/api/client";
import { provenanceListSchema, type Provenance } from "@/lib/domains/types";

export function useDomainProvenance(domainId: string | undefined): UseQueryResult<Provenance[]> {
  const locale = useLocale();
  return useQuery({
    queryKey: ["domain", domainId, "provenance"],
    queryFn: async () => {
      const result = await bffFetch(
        `/api/bff/domains/${domainId}/provenance`,
        provenanceListSchema,
        { locale },
      );
      return result.items;
    },
    enabled: Boolean(domainId),
  });
}
