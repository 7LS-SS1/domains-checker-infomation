"use client";

import { useTranslations } from "next-intl";
import type { UseQueryResult } from "@tanstack/react-query";
import { AlertTriangle, CheckCircle2 } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { ErrorState } from "@/components/ui/error-state";
import { MoneyValue } from "@/components/ui/money-value";
import { describeQueryError } from "@/lib/api/query-error";
import { BUDGET_WINDOW_KEYS, type FinanceSummary } from "@/lib/finance/types";

const TOTAL_FIELDS = [
  { key: "total_current_domain_cost" as const, labelKey: "currentCost" },
  { key: "total_renewal_cost" as const, labelKey: "renewalCost" },
  { key: "estimated_tax" as const, labelKey: "estimatedTax" },
  { key: "total_annual_budget" as const, labelKey: "annualBudget" },
];

const WINDOW_LABEL_KEYS: Record<(typeof BUDGET_WINDOW_KEYS)[number], string> = {
  next_30_days: "windowNext30Days",
  next_60_days: "windowNext60Days",
  next_90_days: "windowNext90Days",
  this_year: "windowThisYear",
};

export function BudgetOverview({ query }: { query: UseQueryResult<FinanceSummary> }) {
  const t = useTranslations("dashboard.budget");
  const tCommon = useTranslations("common");
  const tWarnings = useTranslations("financeWarnings");

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("title")}</CardTitle>
        {query.isSuccess &&
          (query.data.complete ? (
            <span className="inline-flex items-center gap-1 text-xs font-medium text-status-emerald-fg">
              <CheckCircle2 size={14} aria-hidden />
              {t("complete")}
            </span>
          ) : (
            <span className="inline-flex items-center gap-1 text-xs font-medium text-status-amber-fg">
              <AlertTriangle size={14} aria-hidden />
              {t("incomplete")}
            </span>
          ))}
      </CardHeader>
      <CardContent className="space-y-4">
        {query.isError ? (
          <ErrorState
            title={tCommon("loadError")}
            description={describeQueryError(query.error).message}
            requestId={describeQueryError(query.error).requestId}
            onRetry={() => query.refetch()}
          />
        ) : query.isPending ? (
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
            {TOTAL_FIELDS.map((field) => (
              <Skeleton key={field.key} className="h-12 w-full" />
            ))}
          </div>
        ) : (
          <>
            <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
              {TOTAL_FIELDS.map((field) => (
                <div key={field.key}>
                  <p className="text-xs font-medium text-muted-foreground">{t(field.labelKey)}</p>
                  <p className="text-base font-semibold text-foreground">
                    <MoneyValue
                      amount={query.data[field.key]}
                      currency={query.data.reporting_currency}
                    />
                  </p>
                </div>
              ))}
            </div>

            <div className="overflow-x-auto" tabIndex={0} role="region" aria-label={t("title")}>
              <table className="w-full min-w-[480px] text-sm">
                <thead>
                  <tr className="border-b border-border text-left text-xs font-medium text-muted-foreground">
                    <th scope="col" className="py-1.5 pr-3"></th>
                    <th scope="col" className="py-1.5 pr-3">
                      {t("domainCount")}
                    </th>
                    <th scope="col" className="py-1.5 pr-3">
                      {t("knownRenewals")}
                    </th>
                    <th scope="col" className="py-1.5 pr-3">
                      {t("unknownCosts")}
                    </th>
                    <th scope="col" className="py-1.5">
                      {t("renewalTotal")}
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {BUDGET_WINDOW_KEYS.map((windowKey) => {
                    const window = query.data.windows[windowKey];
                    return (
                      <tr key={windowKey} className="border-b border-border last:border-0">
                        <th
                          scope="row"
                          className="py-1.5 pr-3 text-left font-medium text-foreground"
                        >
                          {t(WINDOW_LABEL_KEYS[windowKey])}
                        </th>
                        <td className="py-1.5 pr-3 tabular-nums">{window?.domain_count ?? 0}</td>
                        <td className="py-1.5 pr-3 tabular-nums">{window?.known_renewals ?? 0}</td>
                        <td className="py-1.5 pr-3 tabular-nums">{window?.unknown_costs ?? 0}</td>
                        <td className="py-1.5 tabular-nums">
                          <MoneyValue
                            amount={window?.renewal_total ?? "0"}
                            currency={query.data.reporting_currency}
                          />
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>

            {query.data.warnings.length > 0 && (
              <div className="rounded-md border border-status-amber-border bg-status-amber-bg p-3">
                <p className="mb-1 text-xs font-semibold text-status-amber-fg">
                  {t("warningsTitle")}
                </p>
                <ul className="list-inside list-disc text-xs text-status-amber-fg">
                  {query.data.warnings.map((code) => (
                    <li key={code}>{tWarnings.has(code) ? tWarnings(code) : code}</li>
                  ))}
                </ul>
              </div>
            )}
          </>
        )}
      </CardContent>
    </Card>
  );
}
