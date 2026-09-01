"use client";

import { useTranslations } from "next-intl";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { DateValue } from "@/components/ui/date-value";
import { describeQueryError } from "@/lib/api/query-error";
import { useDomainProvenance } from "@/lib/domains/provenance-queries";

export function ProvenanceTab({ domainId }: { domainId: string }) {
  const t = useTranslations("domainDetail.provenance");
  const tCommon = useTranslations("common");
  const query = useDomainProvenance(domainId);

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("title")}</CardTitle>
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
          <Skeleton className="h-24 w-full" />
        ) : query.data.length === 0 ? (
          <EmptyState title={t("noEntries")} />
        ) : (
          <div className="overflow-x-auto" tabIndex={0} role="region" aria-label={t("title")}>
            <table className="w-full min-w-[560px] text-sm">
              <thead>
                <tr className="border-b border-border text-left text-xs font-medium text-muted-foreground">
                  <th scope="col" className="py-1.5 pr-3">
                    {t("field")}
                  </th>
                  <th scope="col" className="py-1.5 pr-3">
                    {t("source")}
                  </th>
                  <th scope="col" className="py-1.5 pr-3">
                    {t("value")}
                  </th>
                  <th scope="col" className="py-1.5 pr-3">
                    {t("observedAt")}
                  </th>
                  <th scope="col" className="py-1.5">
                    {t("current")}
                  </th>
                </tr>
              </thead>
              <tbody>
                {query.data.map((entry, index) => (
                  <tr key={index} className="border-b border-border last:border-0">
                    <td className="py-1.5 pr-3 text-foreground">{entry.field_name}</td>
                    <td className="py-1.5 pr-3 text-muted-foreground">{entry.source}</td>
                    <td className="py-1.5 pr-3 text-muted-foreground">
                      {JSON.stringify(entry.value)}
                    </td>
                    <td className="py-1.5 pr-3 text-muted-foreground">
                      <DateValue value={entry.observed_at} />
                    </td>
                    <td className="py-1.5">{entry.is_current ? tCommon("yes") : tCommon("no")}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
