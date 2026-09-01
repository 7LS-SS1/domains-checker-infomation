"use client";

import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useTranslations } from "next-intl";
import { Download, FileText, Loader2 } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { ErrorState } from "@/components/ui/error-state";
import { EmptyState } from "@/components/ui/empty-state";
import { DateValue } from "@/components/ui/date-value";
import { MoneyValue } from "@/components/ui/money-value";
import { CurrencySelector } from "@/components/dashboard/currency-selector";
import { ApiError } from "@/lib/api/envelope";
import { describeQueryError } from "@/lib/api/query-error";
import { useCreateReport, useReportSummary } from "@/lib/reports/queries";
import { REPORT_FORMATS, type ReportRecord } from "@/lib/reports/types";
import { hasCapability, toKnownRoles } from "@/lib/auth/capability";

export function ReportsPageContent({ roles }: { roles: readonly string[] }) {
  const t = useTranslations("reportsPage");
  const tCommon = useTranslations("common");
  const tKpi = useTranslations("dashboard.kpi");
  const tFinance = useTranslations("financePage");
  const [reportingCurrency, setReportingCurrency] = useState("THB");
  const [sessionReports, setSessionReports] = useState<ReportRecord[]>([]);

  const summaryQuery = useReportSummary(reportingCurrency);
  const createReport = useCreateReport();
  const canCreate = hasCapability(toKnownRoles(roles), "createReport");

  const schema = z.object({
    format: z.enum(REPORT_FORMATS),
  });
  type FormValues = z.infer<typeof schema>;
  const { register, handleSubmit } = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: { format: "json" },
  });

  const onSubmit = handleSubmit((values) => {
    createReport.mutate(
      { format: values.format, reporting_currency: reportingCurrency },
      { onSuccess: (record) => setSessionReports((current) => [record, ...current]) },
    );
  });

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-xl font-semibold text-foreground">{t("title")}</h1>
        <CurrencySelector value={reportingCurrency} onChange={setReportingCurrency} />
      </div>

      <Card>
        <CardHeader>
          <CardTitle>{t("summaryTitle")}</CardTitle>
        </CardHeader>
        <CardContent>
          {summaryQuery.isError ? (
            <ErrorState
              title={tCommon("loadError")}
              description={describeQueryError(summaryQuery.error).message}
              requestId={describeQueryError(summaryQuery.error).requestId}
              onRetry={() => summaryQuery.refetch()}
            />
          ) : summaryQuery.isPending ? (
            <Skeleton className="h-24 w-full" />
          ) : (
            <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
              <Stat label={tKpi("totalDomains")} value={String(summaryQuery.data.total_domains)} />
              <Stat
                label={tKpi("activeDomains")}
                value={String(summaryQuery.data.active_domains)}
              />
              <Stat
                label={tFinance("renewalCost")}
                value={
                  <MoneyValue
                    amount={summaryQuery.data.renewal_cost}
                    currency={summaryQuery.data.reporting_currency}
                  />
                }
              />
              <Stat
                label={tFinance("annualBudget")}
                value={
                  <MoneyValue
                    amount={summaryQuery.data.annual_budget}
                    currency={summaryQuery.data.reporting_currency}
                  />
                }
              />
            </div>
          )}
          {summaryQuery.isSuccess && summaryQuery.data.completeness_warnings.length > 0 && (
            <p className="mt-3 text-xs text-status-amber-fg">
              {t("warningsTitle")}: {summaryQuery.data.completeness_warnings.join(", ")}
            </p>
          )}
        </CardContent>
      </Card>

      {canCreate && (
        <Card>
          <CardHeader>
            <CardTitle>{t("form.title")}</CardTitle>
          </CardHeader>
          <CardContent>
            <form onSubmit={onSubmit} className="flex flex-wrap items-end gap-3">
              {createReport.isError && (
                <p role="alert" className="w-full text-sm text-status-rose-fg">
                  {createReport.error instanceof ApiError
                    ? createReport.error.message
                    : tCommon("loadError")}
                </p>
              )}
              <div className="space-y-1.5">
                <Label htmlFor="report_format">{t("form.format")}</Label>
                <select
                  id="report_format"
                  className="h-10 rounded-md border border-border bg-background px-2 text-sm text-foreground"
                  {...register("format")}
                >
                  {REPORT_FORMATS.map((format) => (
                    <option key={format} value={format}>
                      {format.toUpperCase()}
                    </option>
                  ))}
                </select>
              </div>
              <Button type="submit" disabled={createReport.isPending}>
                {createReport.isPending ? (
                  <Loader2 className="animate-spin" size={16} aria-hidden />
                ) : (
                  <FileText size={16} aria-hidden />
                )}
                {t("form.submit")}
              </Button>
            </form>
            <p className="mt-2 text-xs text-muted-foreground">{t("noPdfXlsx")}</p>
          </CardContent>
        </Card>
      )}

      <Card>
        <CardHeader>
          <CardTitle>{t("sessionReportsTitle")}</CardTitle>
        </CardHeader>
        <CardContent>
          {sessionReports.length === 0 ? (
            <EmptyState title={t("noSessionReports")} />
          ) : (
            <ul className="divide-y divide-border text-sm">
              {sessionReports.map((report) => (
                <li
                  key={report.id}
                  className="flex flex-wrap items-center justify-between gap-3 py-3"
                >
                  <div>
                    <p className="font-medium text-foreground">
                      {report.format.toUpperCase()} · {report.row_count}{" "}
                      {t("rowCount").toLowerCase()}
                    </p>
                    <p className="text-xs text-muted-foreground">
                      {t("requestedAt")}: <DateValue value={report.requested_at} /> · {t("sha256")}:{" "}
                      <code className="break-all">{report.sha256}</code>
                    </p>
                    {report.completeness_warnings.length > 0 && (
                      <p className="text-xs text-status-amber-fg">
                        {report.completeness_warnings.join(", ")}
                      </p>
                    )}
                  </div>
                  <a
                    href={`/api/bff/reports/${report.id}/download`}
                    className="inline-flex h-9 items-center gap-1.5 rounded-md border border-border px-3 text-sm font-medium text-foreground hover:bg-surface"
                  >
                    <Download size={14} aria-hidden />
                    {t("download")}
                  </a>
                </li>
              ))}
            </ul>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

function Stat({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div>
      <p className="text-xs font-medium text-muted-foreground">{label}</p>
      <p className="text-lg font-semibold tabular-nums text-foreground">{value}</p>
    </div>
  );
}
