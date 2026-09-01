"use client";

import { useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useTranslations } from "next-intl";
import { RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { DateValue } from "@/components/ui/date-value";
import { KpiCards } from "@/components/dashboard/kpi-cards";
import { RecommendationDistribution } from "@/components/dashboard/recommendation-distribution";
import { BudgetOverview } from "@/components/dashboard/budget-overview";
import { ActiveIncidents } from "@/components/dashboard/active-incidents";
import { OperationalStatus } from "@/components/dashboard/operational-status";
import { QuickActions } from "@/components/dashboard/quick-actions";
import { CurrencySelector } from "@/components/dashboard/currency-selector";
import {
  useFinanceSummary,
  useOpenIncidents,
  useReportsSummary,
  useReviewRecommendations,
} from "@/lib/dashboard/queries";
import { useHealth, useReady } from "@/lib/system/queries";

const ACTIVE_INCIDENTS_LIMIT = 5;
const REVIEW_RECOMMENDATIONS_LIMIT = 5;

export function DashboardContent({ roles }: { roles: readonly string[] }) {
  const t = useTranslations("dashboard.refresh");
  const [reportingCurrency, setReportingCurrency] = useState("THB");
  const queryClient = useQueryClient();

  const reportsSummaryQuery = useReportsSummary(reportingCurrency);
  const financeSummaryQuery = useFinanceSummary(reportingCurrency);
  const incidentsQuery = useOpenIncidents(ACTIVE_INCIDENTS_LIMIT);
  const reviewRecommendationsQuery = useReviewRecommendations(REVIEW_RECOMMENDATIONS_LIMIT);
  const healthQuery = useHealth();
  const readyQuery = useReady();

  const isRefreshing =
    reportsSummaryQuery.isFetching ||
    financeSummaryQuery.isFetching ||
    incidentsQuery.isFetching ||
    reviewRecommendationsQuery.isFetching ||
    healthQuery.isFetching ||
    readyQuery.isFetching;

  const lastUpdatedAt = reportsSummaryQuery.dataUpdatedAt;

  function handleRefresh() {
    void queryClient.invalidateQueries({ queryKey: ["dashboard"] });
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <CurrencySelector value={reportingCurrency} onChange={setReportingCurrency} />
        <div className="flex items-center gap-3">
          <p className="text-xs text-muted-foreground">
            {t("lastUpdated")}:{" "}
            {lastUpdatedAt ? (
              <DateValue value={new Date(lastUpdatedAt).toISOString()} />
            ) : (
              t("never")
            )}
          </p>
          <Button
            variant="secondary"
            size="sm"
            onClick={handleRefresh}
            disabled={isRefreshing}
            aria-label={t("refreshNow")}
          >
            <RefreshCw
              size={16}
              className={isRefreshing ? "animate-spin" : undefined}
              aria-hidden
            />
            {t("refreshNow")}
          </Button>
        </div>
      </div>

      <QuickActions roles={roles} />

      <KpiCards query={reportsSummaryQuery} />

      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <RecommendationDistribution
          summaryQuery={reportsSummaryQuery}
          reviewQuery={reviewRecommendationsQuery}
        />
        <BudgetOverview query={financeSummaryQuery} />
      </div>

      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <ActiveIncidents query={incidentsQuery} />
        <OperationalStatus healthQuery={healthQuery} readyQuery={readyQuery} />
      </div>
    </div>
  );
}
