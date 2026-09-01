"use client";

import { useCallback, useMemo } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { useTranslations } from "next-intl";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { DateValue } from "@/components/ui/date-value";
import { StatusBadge } from "@/components/ui/status-badge";
import { describeQueryError } from "@/lib/api/query-error";
import { useIncidents } from "@/lib/incidents/queries";
import { incidentStatusIcon, incidentStatusTone } from "@/lib/incidents/status";
import { INCIDENT_STATUSES } from "@/lib/incidents/types";

const PAGE_SIZE = 50;

export function IncidentsPageContent() {
  const t = useTranslations("incidentsPage");
  const tStatuses = useTranslations("incidents.statuses");
  const tCommon = useTranslations("common");
  const router = useRouter();
  const searchParams = useSearchParams();

  const status = searchParams.get("status") ?? "";
  const page = Number(searchParams.get("page") ?? "1") || 1;

  const query = useIncidents({ status, page, pageSize: PAGE_SIZE });

  const updateParams = useCallback(
    (updates: Record<string, string | undefined>) => {
      const params = new URLSearchParams(searchParams.toString());
      for (const [key, value] of Object.entries(updates)) {
        if (!value) {
          params.delete(key);
        } else {
          params.set(key, value);
        }
      }
      router.push(`/incidents?${params.toString()}`);
    },
    [router, searchParams],
  );

  const totalPages = query.data?.total_pages ?? 1;

  const statusOptions = useMemo(() => INCIDENT_STATUSES, []);

  return (
    <div className="space-y-4">
      <h1 className="text-xl font-semibold text-foreground">{t("title")}</h1>

      <div className="flex items-center gap-2">
        <label htmlFor="incident-status" className="text-xs font-medium text-muted-foreground">
          {t("filterStatus")}
        </label>
        <select
          id="incident-status"
          value={status}
          onChange={(event) =>
            updateParams({ status: event.target.value || undefined, page: undefined })
          }
          className="h-10 rounded-md border border-border bg-background px-2 text-sm text-foreground"
        >
          <option value="">{t("allOption")}</option>
          {statusOptions.map((value) => (
            <option key={value} value={value}>
              {tStatuses(value)}
            </option>
          ))}
        </select>
      </div>

      {query.isError ? (
        <ErrorState
          title={tCommon("loadError")}
          description={describeQueryError(query.error).message}
          requestId={describeQueryError(query.error).requestId}
          onRetry={() => query.refetch()}
        />
      ) : query.isPending ? (
        <div className="space-y-2">
          {Array.from({ length: 6 }).map((_, index) => (
            <Skeleton key={index} className="h-12 w-full" />
          ))}
        </div>
      ) : query.data.items.length === 0 ? (
        <EmptyState title={status ? t("empty") : t("emptyNoData")} />
      ) : (
        <div
          className="overflow-x-auto rounded-lg border border-border"
          tabIndex={0}
          role="region"
          aria-label={t("title")}
        >
          <table className="w-full min-w-[860px] text-sm">
            <thead>
              <tr className="border-b border-border bg-surface text-left text-xs font-medium text-muted-foreground">
                <th scope="col" className="px-3 py-2">
                  {t("columns.domain")}
                </th>
                <th scope="col" className="px-3 py-2">
                  {t("columns.status")}
                </th>
                <th scope="col" className="px-3 py-2">
                  {t("columns.failureStage")}
                </th>
                <th scope="col" className="px-3 py-2">
                  {t("columns.openedAt")}
                </th>
                <th scope="col" className="px-3 py-2">
                  {t("columns.closedAt")}
                </th>
                <th scope="col" className="px-3 py-2">
                  {t("viewRun")}
                </th>
              </tr>
            </thead>
            <tbody>
              {query.data.items.map((incident) => (
                <tr key={incident.id} className="border-b border-border last:border-0">
                  <td className="px-3 py-2">
                    <Link
                      href={`/domains/${incident.domain_id}`}
                      className="font-medium text-foreground hover:underline"
                    >
                      {incident.domain_ascii}
                    </Link>
                  </td>
                  <td className="px-3 py-2">
                    <StatusBadge
                      tone={incidentStatusTone(incident.status)}
                      icon={incidentStatusIcon(incident.status)}
                      label={tStatuses(incident.status)}
                    />
                  </td>
                  <td className="px-3 py-2 text-muted-foreground">
                    {incident.failure_stage ?? incident.error_code ?? "—"}
                  </td>
                  <td className="px-3 py-2 text-muted-foreground">
                    <DateValue value={incident.opened_at} />
                  </td>
                  <td className="px-3 py-2 text-muted-foreground">
                    {incident.closed_at ? <DateValue value={incident.closed_at} /> : t("notClosed")}
                  </td>
                  <td className="px-3 py-2">
                    <Link
                      href={`/domains/${incident.domain_id}`}
                      className="text-xs text-primary hover:underline"
                    >
                      {t("viewRun")}
                    </Link>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {query.data && query.data.total_items > 0 && (
        <div className="flex items-center justify-between gap-3 text-sm text-muted-foreground">
          <p>{tCommon("pageOf", { page, totalPages })}</p>
          <div className="flex gap-2">
            <Button
              variant="secondary"
              size="sm"
              disabled={page <= 1}
              onClick={() => updateParams({ page: String(page - 1) })}
            >
              {tCommon("previous")}
            </Button>
            <Button
              variant="secondary"
              size="sm"
              disabled={page >= totalPages}
              onClick={() => updateParams({ page: String(page + 1) })}
            >
              {tCommon("next")}
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}
