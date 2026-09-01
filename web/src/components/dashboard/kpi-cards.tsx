"use client";

import { useTranslations } from "next-intl";
import type { UseQueryResult } from "@tanstack/react-query";
import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { ErrorState } from "@/components/ui/error-state";
import { describeQueryError } from "@/lib/api/query-error";
import type { ReportSummary } from "@/lib/reports/types";

interface KpiDefinition {
  key: keyof Pick<
    ReportSummary,
    | "total_domains"
    | "active_domains"
    | "unavailable_domains"
    | "permanent_redirect_domains"
    | "suspected_isp_block"
    | "high_confidence_isp_block"
    | "dns_errors"
    | "tls_errors"
    | "expiring_within_90_days"
  >;
  labelKey: string;
  tone: "default" | "warn" | "danger";
}

const KPI_DEFINITIONS: readonly KpiDefinition[] = [
  { key: "total_domains", labelKey: "totalDomains", tone: "default" },
  { key: "active_domains", labelKey: "activeDomains", tone: "default" },
  { key: "unavailable_domains", labelKey: "unavailableDomains", tone: "danger" },
  { key: "permanent_redirect_domains", labelKey: "permanentRedirects", tone: "default" },
  { key: "suspected_isp_block", labelKey: "suspectedIspBlock", tone: "warn" },
  { key: "high_confidence_isp_block", labelKey: "highConfidenceIspBlock", tone: "danger" },
  { key: "dns_errors", labelKey: "dnsErrors", tone: "danger" },
  { key: "tls_errors", labelKey: "tlsErrors", tone: "warn" },
  { key: "expiring_within_90_days", labelKey: "expiringSoon", tone: "warn" },
];

const TONE_CLASSES: Record<KpiDefinition["tone"], string> = {
  default: "text-foreground",
  warn: "text-status-amber-fg",
  danger: "text-status-rose-fg",
};

export function KpiCards({ query }: { query: UseQueryResult<ReportSummary> }) {
  const t = useTranslations("dashboard.kpi");
  const tCommon = useTranslations("common");

  if (query.isError) {
    const info = describeQueryError(query.error);
    return (
      <ErrorState
        title={tCommon("loadError")}
        description={info.message}
        requestId={info.requestId}
        onRetry={() => query.refetch()}
      />
    );
  }

  return (
    <section aria-labelledby="kpi-heading">
      <h2 id="kpi-heading" className="mb-2 text-sm font-semibold text-foreground">
        {t("title")}
      </h2>
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 xl:grid-cols-5">
        {KPI_DEFINITIONS.map((definition) => (
          <Card key={definition.key}>
            <CardContent className="p-4">
              <p className="text-xs font-medium text-muted-foreground">{t(definition.labelKey)}</p>
              {query.isPending ? (
                <Skeleton className="mt-1 h-7 w-16" />
              ) : (
                <p
                  className={`mt-1 text-2xl font-semibold tabular-nums ${TONE_CLASSES[definition.tone]}`}
                >
                  {query.data![definition.key]}
                </p>
              )}
            </CardContent>
          </Card>
        ))}
      </div>
    </section>
  );
}
