"use client";

import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import { useLocale } from "next-intl";
import { bffFetch } from "@/lib/api/client";
import { reportSummarySchema, type ReportSummary } from "@/lib/reports/types";
import { financeSummarySchema, type FinanceSummary } from "@/lib/finance/types";
import { incidentPageSchema, type IncidentPage } from "@/lib/incidents/types";
import { recommendationListSchema, type Recommendation } from "@/lib/recommendations/types";

/**
 * 60s bounded polling, paused while the tab is hidden
 * (refetchIntervalInBackground: false), matching
 * web/FRONTEND_ARCHITECTURE.md section 4's polling-limits rule. All keys
 * share the "dashboard" prefix so the manual refresh button can invalidate
 * every dashboard query with one call.
 */
const REFRESH_INTERVAL_MS = 60_000;

export function useReportsSummary(reportingCurrency: string): UseQueryResult<ReportSummary> {
  const locale = useLocale();
  return useQuery({
    queryKey: ["dashboard", "reports-summary", reportingCurrency],
    queryFn: () =>
      bffFetch(
        `/api/bff/reports/summary?reporting_currency=${encodeURIComponent(reportingCurrency)}`,
        reportSummarySchema,
        { locale },
      ),
    refetchInterval: REFRESH_INTERVAL_MS,
    refetchIntervalInBackground: false,
  });
}

export function useFinanceSummary(reportingCurrency: string): UseQueryResult<FinanceSummary> {
  const locale = useLocale();
  return useQuery({
    queryKey: ["dashboard", "finance-summary", reportingCurrency],
    queryFn: () =>
      bffFetch(
        `/api/bff/finance/summary?reporting_currency=${encodeURIComponent(reportingCurrency)}`,
        financeSummarySchema,
        { locale },
      ),
    refetchInterval: REFRESH_INTERVAL_MS,
    refetchIntervalInBackground: false,
  });
}

export function useOpenIncidents(limit: number): UseQueryResult<IncidentPage> {
  const locale = useLocale();
  return useQuery({
    queryKey: ["dashboard", "incidents", "open", limit],
    queryFn: () =>
      bffFetch(
        `/api/bff/incidents?status=open&page_size=${encodeURIComponent(String(limit))}`,
        incidentPageSchema,
        { locale },
      ),
    refetchInterval: REFRESH_INTERVAL_MS,
    refetchIntervalInBackground: false,
  });
}

export function useReviewRecommendations(limit: number): UseQueryResult<Recommendation[]> {
  const locale = useLocale();
  return useQuery({
    queryKey: ["dashboard", "recommendations", "REVIEW", limit],
    queryFn: async () => {
      const result = await bffFetch(
        `/api/bff/recommendations?action=REVIEW&limit=${encodeURIComponent(String(limit))}`,
        recommendationListSchema,
        { locale },
      );
      return result.items;
    },
    refetchInterval: REFRESH_INTERVAL_MS,
    refetchIntervalInBackground: false,
  });
}
