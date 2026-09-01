"use client";

import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import { useLocale } from "next-intl";
import { bffFetch } from "@/lib/api/client";
import {
  costRecordListSchema,
  overrideRecordListSchema,
  financeSummarySchema,
} from "@/lib/finance/types";
import type { CostRecord, FinanceSummary, OverrideRecord } from "@/lib/finance/types";

export function useFinanceSummary(reportingCurrency: string): UseQueryResult<FinanceSummary> {
  const locale = useLocale();
  return useQuery({
    queryKey: ["finance-summary", reportingCurrency],
    queryFn: () =>
      bffFetch(
        `/api/bff/finance/summary?reporting_currency=${encodeURIComponent(reportingCurrency)}`,
        financeSummarySchema,
        { locale },
      ),
  });
}

export function useDomainCosts(domainId: string | undefined): UseQueryResult<CostRecord[]> {
  const locale = useLocale();
  return useQuery({
    queryKey: ["domain", domainId, "costs"],
    queryFn: async () => {
      const result = await bffFetch(`/api/bff/domains/${domainId}/costs`, costRecordListSchema, {
        locale,
      });
      return result.items;
    },
    enabled: Boolean(domainId),
  });
}

export function useDomainOverrides(domainId: string | undefined): UseQueryResult<OverrideRecord[]> {
  const locale = useLocale();
  return useQuery({
    queryKey: ["domain", domainId, "overrides"],
    queryFn: async () => {
      const result = await bffFetch(
        `/api/bff/domains/${domainId}/overrides`,
        overrideRecordListSchema,
        { locale },
      );
      return result.items;
    },
    enabled: Boolean(domainId),
  });
}
