"use client";

import { useMutation, useQuery, useQueryClient, type UseQueryResult } from "@tanstack/react-query";
import { useLocale } from "next-intl";
import { bffFetch } from "@/lib/api/client";
import {
  reportDashboardSchema,
  reportRecordSchema,
  reportSummarySchema,
  type ReportDashboard,
  type ReportRecord,
  type ReportSummary,
} from "@/lib/reports/types";

export function useReportSummary(reportingCurrency: string): UseQueryResult<ReportSummary> {
  const locale = useLocale();
  return useQuery({
    queryKey: ["report-summary", reportingCurrency],
    queryFn: () =>
      bffFetch(
        `/api/bff/reports/summary?reporting_currency=${encodeURIComponent(reportingCurrency)}`,
        reportSummarySchema,
        { locale },
      ),
  });
}

export function useReportDashboard(reportingCurrency: string): UseQueryResult<ReportDashboard> {
  const locale = useLocale();
  return useQuery({
    queryKey: ["report-dashboard", reportingCurrency],
    queryFn: () =>
      bffFetch(
        `/api/bff/reports/dashboard?reporting_currency=${encodeURIComponent(reportingCurrency)}`,
        reportDashboardSchema,
        { locale },
      ),
  });
}

export function useReport(reportId: string | undefined): UseQueryResult<ReportRecord> {
  const locale = useLocale();
  return useQuery({
    queryKey: ["report", reportId],
    queryFn: () => bffFetch(`/api/bff/reports/${reportId}`, reportRecordSchema, { locale }),
    enabled: Boolean(reportId),
  });
}

export interface CreateReportInput {
  format: "json" | "csv" | "pdf";
  reporting_currency: string;
}

/**
 * A fresh Idempotency-Key is generated per distinct submission — never
 * reused across two different form payloads, per the Master Prompt rule
 * and web/API_GAPS.md GAP-08 (the backend's idempotency lookup does not
 * hash the request body, so key reuse across a changed payload would
 * silently return the first stored result).
 */
export function useCreateReport() {
  const locale = useLocale();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: CreateReportInput) =>
      bffFetch("/api/bff/reports", reportRecordSchema, {
        method: "POST",
        body: JSON.stringify(input),
        locale,
        headers: { "Idempotency-Key": crypto.randomUUID() },
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["reports"] });
    },
  });
}
