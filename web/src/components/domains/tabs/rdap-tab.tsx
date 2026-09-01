"use client";

import { useTranslations } from "next-intl";
import { Loader2, RefreshCw } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { DateValue } from "@/components/ui/date-value";
import { JsonViewer } from "@/components/ui/json-viewer";
import { ApiError } from "@/lib/api/envelope";
import { describeQueryError } from "@/lib/api/query-error";
import { useRdap, useRdapCheck } from "@/lib/rdap/queries";
import { hasCapability, toKnownRoles } from "@/lib/auth/capability";

export function RdapTab({ domainId, roles }: { domainId: string; roles: readonly string[] }) {
  const t = useTranslations("domainDetail.rdap");
  const tCommon = useTranslations("common");
  const query = useRdap(domainId);
  const check = useRdapCheck();
  const canCheck = hasCapability(toKnownRoles(roles), "editDomains");

  const isNotFound =
    query.isError &&
    query.error instanceof ApiError &&
    query.error.code === "RDAP_RESULT_NOT_FOUND";

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("title")}</CardTitle>
        {canCheck && (
          <Button
            variant="secondary"
            size="sm"
            disabled={check.isPending}
            onClick={() => check.mutate({ domainId, force: true })}
          >
            {check.isPending ? (
              <Loader2 size={14} className="animate-spin" aria-hidden />
            ) : (
              <RefreshCw size={14} aria-hidden />
            )}
            {t("forceCheck")}
          </Button>
        )}
      </CardHeader>
      <CardContent className="space-y-3">
        {query.isPending ? (
          <Skeleton className="h-40 w-full" />
        ) : isNotFound ? (
          <EmptyState title={t("notFound")} />
        ) : query.isError ? (
          <ErrorState
            title={tCommon("loadError")}
            description={describeQueryError(query.error).message}
            requestId={describeQueryError(query.error).requestId}
            onRetry={() => query.refetch()}
          />
        ) : (
          <>
            <dl className="grid grid-cols-1 gap-3 text-sm sm:grid-cols-2">
              <div>
                <dt className="text-xs text-muted-foreground">{t("registrar")}</dt>
                <dd className="text-foreground">{query.data.registration.registrar_name ?? "—"}</dd>
              </div>
              <div>
                <dt className="text-xs text-muted-foreground">{t("registrarIana")}</dt>
                <dd className="text-foreground">
                  {query.data.registration.registrar_iana_id ?? "—"}
                </dd>
              </div>
              <div>
                <dt className="text-xs text-muted-foreground">{t("registrationAt")}</dt>
                <dd className="text-foreground">
                  {query.data.registration.registration_at ? (
                    <DateValue value={query.data.registration.registration_at} dateOnly />
                  ) : (
                    "—"
                  )}
                </dd>
              </div>
              <div>
                <dt className="text-xs text-muted-foreground">{t("expirationAt")}</dt>
                <dd className="text-foreground">
                  {query.data.registration.expiration_at ? (
                    <DateValue value={query.data.registration.expiration_at} dateOnly />
                  ) : (
                    "—"
                  )}
                </dd>
              </div>
              <div>
                <dt className="text-xs text-muted-foreground">{t("dnssec")}</dt>
                <dd className="text-foreground">
                  {query.data.registration.dnssec == null
                    ? "—"
                    : query.data.registration.dnssec
                      ? tCommon("yes")
                      : tCommon("no")}
                </dd>
              </div>
              <div>
                <dt className="text-xs text-muted-foreground">{t("nameservers")}</dt>
                <dd className="text-foreground">
                  {query.data.registration.nameservers.length > 0
                    ? query.data.registration.nameservers.join(", ")
                    : "—"}
                </dd>
              </div>
            </dl>

            {(query.data.cached || query.data.bootstrap_stale) && (
              <p className="text-xs text-status-amber-fg">
                {query.data.cached && t("cached")}{" "}
                {query.data.bootstrap_stale && t("bootstrapStale")}
              </p>
            )}

            <div>
              <h3 className="mb-1 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                {t("conflictsTitle")}
              </h3>
              {query.data.conflicts.length === 0 ? (
                <p className="text-sm text-muted-foreground">{t("noConflicts")}</p>
              ) : (
                <ul className="space-y-1 text-sm">
                  {query.data.conflicts.map((conflict, index) => (
                    <li
                      key={index}
                      className="rounded-md border border-status-amber-border bg-status-amber-bg p-2"
                    >
                      <span className="font-medium">{conflict.field}</span>:{" "}
                      {conflict.current_source}={JSON.stringify(conflict.current_value)} vs{" "}
                      {conflict.observed_source}={JSON.stringify(conflict.observed_value)}
                    </li>
                  ))}
                </ul>
              )}
            </div>

            <JsonViewer data={query.data} label={t("title")} />
          </>
        )}
      </CardContent>
    </Card>
  );
}
