"use client";

import Link from "next/link";
import { useTranslations, useLocale } from "next-intl";
import type { UseQueryResult } from "@tanstack/react-query";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { ErrorState } from "@/components/ui/error-state";
import { EmptyState } from "@/components/ui/empty-state";
import { StatusBadge } from "@/components/ui/status-badge";
import { describeQueryError } from "@/lib/api/query-error";
import { recommendationActionIcon, recommendationActionTone } from "@/lib/recommendations/status";
import type { ReportSummary } from "@/lib/reports/types";
import type { Recommendation } from "@/lib/recommendations/types";

const DISTRIBUTION_ITEMS = [
  { action: "RENEW" as const, key: "recommended_renew" as const, labelKey: "renew" },
  { action: "DROP" as const, key: "recommended_drop" as const, labelKey: "drop" },
  { action: "REVIEW" as const, key: "review_required" as const, labelKey: "review" },
  {
    action: "PROFIT_OPPORTUNITY" as const,
    key: "profit_opportunities" as const,
    labelKey: "profitOpportunity",
  },
];

interface RecommendationDistributionProps {
  summaryQuery: UseQueryResult<ReportSummary>;
  reviewQuery: UseQueryResult<Recommendation[]>;
}

export function RecommendationDistribution({
  summaryQuery,
  reviewQuery,
}: RecommendationDistributionProps) {
  const t = useTranslations("dashboard.distribution");
  const tCommon = useTranslations("common");
  const locale = useLocale();

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("title")}</CardTitle>
        <Link href="/recommendations" className="text-xs font-medium text-primary hover:underline">
          {t("viewAll")}
        </Link>
      </CardHeader>
      <CardContent className="space-y-4">
        {summaryQuery.isError ? (
          <ErrorState
            title={tCommon("loadError")}
            description={describeQueryError(summaryQuery.error).message}
            requestId={describeQueryError(summaryQuery.error).requestId}
            onRetry={() => summaryQuery.refetch()}
          />
        ) : (
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
            {DISTRIBUTION_ITEMS.map((item) => (
              <div key={item.action} className="flex flex-col items-start gap-1.5">
                <StatusBadge
                  tone={recommendationActionTone(item.action)}
                  icon={recommendationActionIcon(item.action)}
                  label={t(item.labelKey)}
                />
                {summaryQuery.isPending ? (
                  <Skeleton className="h-6 w-10" />
                ) : (
                  <p className="text-xl font-semibold tabular-nums text-foreground">
                    {summaryQuery.data![item.key]}
                  </p>
                )}
              </div>
            ))}
          </div>
        )}

        <div>
          <h3 className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
            {t("reviewListTitle")}
          </h3>
          {reviewQuery.isError ? (
            <ErrorState
              title={tCommon("loadError")}
              description={describeQueryError(reviewQuery.error).message}
              requestId={describeQueryError(reviewQuery.error).requestId}
              onRetry={() => reviewQuery.refetch()}
            />
          ) : reviewQuery.isPending ? (
            <div className="space-y-2">
              <Skeleton className="h-8 w-full" />
              <Skeleton className="h-8 w-full" />
            </div>
          ) : reviewQuery.data!.length === 0 ? (
            <EmptyState title={t("empty")} className="p-4" />
          ) : (
            <ul className="divide-y divide-border">
              {reviewQuery.data!.map((recommendation) => {
                const reasons =
                  locale === "th" ? recommendation.reasons_th : recommendation.reasons_en;
                return (
                  <li key={recommendation.id} className="py-2">
                    <Link
                      href={`/domains/${recommendation.domain_id}`}
                      className="text-sm font-medium text-foreground hover:underline"
                    >
                      {recommendation.domain}
                    </Link>
                    {reasons[0] && <p className="text-xs text-muted-foreground">{reasons[0]}</p>}
                  </li>
                );
              })}
            </ul>
          )}
        </div>
      </CardContent>
    </Card>
  );
}
