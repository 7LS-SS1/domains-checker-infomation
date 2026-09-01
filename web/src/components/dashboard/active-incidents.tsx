"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";
import type { UseQueryResult } from "@tanstack/react-query";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { ErrorState } from "@/components/ui/error-state";
import { EmptyState } from "@/components/ui/empty-state";
import { StatusBadge } from "@/components/ui/status-badge";
import { DateValue } from "@/components/ui/date-value";
import { describeQueryError } from "@/lib/api/query-error";
import { incidentStatusIcon, incidentStatusTone } from "@/lib/incidents/status";
import type { IncidentPage } from "@/lib/incidents/types";

export function ActiveIncidents({ query }: { query: UseQueryResult<IncidentPage> }) {
  const t = useTranslations("dashboard.incidentsSection");
  const tStatuses = useTranslations("incidents.statuses");
  const tCommon = useTranslations("common");

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("title")}</CardTitle>
        <Link href="/incidents" className="text-xs font-medium text-primary hover:underline">
          {t("viewAll")}
        </Link>
      </CardHeader>
      <CardContent>
        {query.isError ? (
          <ErrorState
            title={tCommon("loadError")}
            description={describeQueryError(query.error).message}
            requestId={describeQueryError(query.error).requestId}
            onRetry={() => query.refetch()}
          />
        ) : query.isPending ? (
          <div className="space-y-2">
            <Skeleton className="h-10 w-full" />
            <Skeleton className="h-10 w-full" />
          </div>
        ) : query.data.items.length === 0 ? (
          <EmptyState title={t("empty")} />
        ) : (
          <ul className="divide-y divide-border">
            {query.data.items.map((incident) => (
              <li key={incident.id} className="flex items-center justify-between gap-3 py-2">
                <div className="min-w-0">
                  <Link
                    href={`/domains/${incident.domain_id}`}
                    className="truncate text-sm font-medium text-foreground hover:underline"
                  >
                    {incident.domain_ascii}
                  </Link>
                  <p className="text-xs text-muted-foreground">
                    {t("openedAt")}: <DateValue value={incident.opened_at} />
                  </p>
                </div>
                <StatusBadge
                  tone={incidentStatusTone(incident.status)}
                  icon={incidentStatusIcon(incident.status)}
                  label={tStatuses(incident.status)}
                />
              </li>
            ))}
          </ul>
        )}
      </CardContent>
    </Card>
  );
}
