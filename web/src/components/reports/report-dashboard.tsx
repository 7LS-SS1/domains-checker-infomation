"use client";

import type { ReactNode } from "react";
import { useTranslations } from "next-intl";
import type { UseQueryResult } from "@tanstack/react-query";
import {
  Cell,
  CartesianGrid,
  Line,
  LineChart,
  Pie,
  PieChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { ErrorState } from "@/components/ui/error-state";
import { MoneyValue } from "@/components/ui/money-value";
import { describeQueryError } from "@/lib/api/query-error";
import type { ReportDashboard as ReportDashboardData, StatusCount } from "@/lib/reports/types";

/**
 * Distribution donuts deliberately deviate from the app-wide single-tone ISP
 * badge convention (SUSPECTED and HIGH_CONFIDENCE_BLOCK both render violet
 * elsewhere): a chart needs to visually distinguish the two severities, so
 * HIGH_CONFIDENCE_BLOCK escalates to rose here. The PDF export
 * (internal/report/pdf.go) uses the same mapping so the two stay identical.
 */
const AVAILABILITY_COLOR: Record<string, string> = {
  ACTIVE: "var(--status-emerald-fg)",
  DEGRADED: "var(--status-amber-fg)",
  UNAVAILABLE: "var(--status-rose-fg)",
};
const ISP_COLOR: Record<string, string> = {
  NOT_DETECTED: "var(--status-emerald-fg)",
  SUSPECTED: "var(--status-violet-fg)",
  HIGH_CONFIDENCE_BLOCK: "var(--status-rose-fg)",
};
const FALLBACK_COLOR = "var(--status-slate-fg)";

export function ReportDashboard({ query }: { query: UseQueryResult<ReportDashboardData> }) {
  const t = useTranslations("reportsPage.dashboard");
  const tKpi = useTranslations("dashboard.kpi");
  const tFinance = useTranslations("financePage");
  const tCommon = useTranslations("common");
  const tAvailability = useTranslations("domainStatus.availability");
  const tIsp = useTranslations("domainStatus.isp");
  const tActions = useTranslations("recommendations.actions");

  if (query.isError) {
    return (
      <Card>
        <CardContent className="pt-6">
          <ErrorState
            title={tCommon("loadError")}
            description={describeQueryError(query.error).message}
            requestId={describeQueryError(query.error).requestId}
            onRetry={() => query.refetch()}
          />
        </CardContent>
      </Card>
    );
  }

  if (query.isPending) {
    return (
      <Card>
        <CardContent className="space-y-3 pt-6">
          <Skeleton className="h-24 w-full" />
          <Skeleton className="h-48 w-full" />
          <Skeleton className="h-48 w-full" />
        </CardContent>
      </Card>
    );
  }

  const data = query.data;
  const availabilityLabel = (status: string) =>
    tAvailability.has(status) ? tAvailability(status) : status;
  const ispLabel = (status: string) => (tIsp.has(status) ? tIsp(status) : status);
  const actionLabel = (action: string) => (tActions.has(action) ? tActions(action) : action);

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle>{t("kpiTitle")}</CardTitle>
        </CardHeader>
        <CardContent className="grid grid-cols-2 gap-3 sm:grid-cols-4">
          <Stat label={tKpi("totalDomains")} value={String(data.total_domains)} />
          <Stat label={tKpi("activeDomains")} value={String(data.active_domains)} />
          <Stat label={tKpi("expiringSoon")} value={String(data.expiring_within_90_days)} />
          <Stat
            label={tFinance("renewalCost")}
            value={<MoneyValue amount={data.renewal_cost} currency={data.reporting_currency} />}
          />
          <Stat label={tKpi("unavailableDomains")} value={String(data.unavailable_domains)} />
          <Stat label={tKpi("suspectedIspBlock")} value={String(data.suspected_isp_block)} />
          <Stat label={tKpi("dnsErrors")} value={String(data.dns_errors)} />
          <Stat label={tKpi("tlsErrors")} value={String(data.tls_errors)} />
        </CardContent>
      </Card>

      <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
        <DistributionCard
          title={t("availabilityTitle")}
          counts={data.availability_distribution}
          colorFor={(status) => AVAILABILITY_COLOR[status] ?? FALLBACK_COLOR}
          labelFor={availabilityLabel}
        />
        <DistributionCard
          title={t("ispTitle")}
          counts={data.isp_distribution}
          colorFor={(status) => ISP_COLOR[status] ?? FALLBACK_COLOR}
          labelFor={ispLabel}
        />
      </div>

      <Card>
        <CardHeader>
          <CardTitle>{t("trendTitle")}</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="h-56 w-full">
            <ResponsiveContainer width="100%" height="100%">
              <LineChart
                data={data.incident_trend_30d}
                margin={{ top: 8, right: 8, left: -16, bottom: 0 }}
              >
                <CartesianGrid strokeDasharray="3 3" stroke="var(--status-slate-border)" />
                <XAxis
                  dataKey="date"
                  tick={{ fontSize: 11, fill: "var(--muted-foreground)" }}
                  interval="preserveStartEnd"
                  minTickGap={40}
                />
                <YAxis
                  allowDecimals={false}
                  tick={{ fontSize: 11, fill: "var(--muted-foreground)" }}
                  width={28}
                />
                <Tooltip
                  contentStyle={{
                    background: "var(--card)",
                    border: "1px solid var(--border)",
                    borderRadius: 8,
                    fontSize: 12,
                  }}
                  labelStyle={{ color: "var(--foreground)" }}
                />
                <Line
                  type="monotone"
                  dataKey="count"
                  name={t("trendSeriesName")}
                  stroke="var(--status-rose-fg)"
                  strokeWidth={2}
                  dot={false}
                />
              </LineChart>
            </ResponsiveContainer>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t("topDomainsTitle")}</CardTitle>
        </CardHeader>
        <CardContent>
          {data.top_domains.length === 0 ? (
            <p className="text-sm text-muted-foreground">{t("topDomainsEmpty")}</p>
          ) : (
            <div
              className="overflow-x-auto"
              tabIndex={0}
              role="region"
              aria-label={t("topDomainsTitle")}
            >
              <table className="w-full min-w-[640px] text-sm">
                <thead>
                  <tr className="border-b border-border text-left text-xs font-medium text-muted-foreground">
                    <th scope="col" className="py-1.5 pr-3">
                      {t("columns.domain")}
                    </th>
                    <th scope="col" className="py-1.5 pr-3">
                      {t("columns.availability")}
                    </th>
                    <th scope="col" className="py-1.5 pr-3">
                      {t("columns.isp")}
                    </th>
                    <th scope="col" className="py-1.5 pr-3">
                      {t("columns.recommendation")}
                    </th>
                    <th scope="col" className="py-1.5">
                      {t("columns.renewalCost")}
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {data.top_domains.map((item) => (
                    <tr key={item.domain} className="border-b border-border last:border-0">
                      <td
                        className="max-w-[220px] truncate py-1.5 pr-3 font-medium text-foreground"
                        title={item.domain}
                      >
                        {item.domain}
                      </td>
                      <td className="py-1.5 pr-3 text-muted-foreground">
                        {availabilityLabel(item.availability_status)}
                      </td>
                      <td className="py-1.5 pr-3 text-muted-foreground">
                        {ispLabel(item.isp_status)}
                      </td>
                      <td className="py-1.5 pr-3 text-muted-foreground">
                        {actionLabel(item.recommendation)}
                      </td>
                      <td className="py-1.5 tabular-nums">
                        {item.renewal_cost && item.renewal_cost_currency ? (
                          <MoneyValue
                            amount={item.renewal_cost}
                            currency={item.renewal_cost_currency}
                          />
                        ) : (
                          t("columns.noRenewalCost")
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

function DistributionCard({
  title,
  counts,
  colorFor,
  labelFor,
}: {
  title: string;
  counts: StatusCount[];
  colorFor: (status: string) => string;
  labelFor: (status: string) => string;
}) {
  const total = counts.reduce((sum, item) => sum + item.count, 0);
  return (
    <Card>
      <CardHeader>
        <CardTitle>{title}</CardTitle>
      </CardHeader>
      <CardContent>
        {total === 0 ? (
          <p className="text-sm text-muted-foreground">—</p>
        ) : (
          <div className="flex items-center gap-4">
            <div className="h-40 w-40 shrink-0">
              <ResponsiveContainer width="100%" height="100%">
                <PieChart>
                  <Pie
                    data={counts}
                    dataKey="count"
                    nameKey="status"
                    innerRadius="55%"
                    outerRadius="100%"
                    paddingAngle={1}
                    stroke="none"
                  >
                    {counts.map((item) => (
                      <Cell key={item.status} fill={colorFor(item.status)} />
                    ))}
                  </Pie>
                </PieChart>
              </ResponsiveContainer>
            </div>
            <ul className="space-y-1.5 text-sm">
              {counts.map((item) => (
                <li key={item.status} className="flex items-center gap-2">
                  <span
                    className="h-2.5 w-2.5 shrink-0 rounded-sm"
                    style={{ backgroundColor: colorFor(item.status) }}
                  />
                  <span className="text-foreground">{labelFor(item.status)}</span>
                  <span className="text-muted-foreground">
                    {item.count} ({total > 0 ? Math.round((100 * item.count) / total) : 0}%)
                  </span>
                </li>
              ))}
            </ul>
          </div>
        )}
      </CardContent>
    </Card>
  );
}

function Stat({ label, value }: { label: string; value: ReactNode }) {
  return (
    <div>
      <p className="text-xs font-medium text-muted-foreground">{label}</p>
      <p className="text-lg font-semibold tabular-nums text-foreground">{value}</p>
    </div>
  );
}
