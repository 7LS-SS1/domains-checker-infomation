"use client";

import { useTranslations } from "next-intl";
import { ArrowLeft } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { ErrorState } from "@/components/ui/error-state";
import { EmptyState } from "@/components/ui/empty-state";
import { DateValue } from "@/components/ui/date-value";
import { JsonViewer } from "@/components/ui/json-viewer";
import { DomainStatusBadge } from "@/components/domains/domain-status-badge";
import { describeQueryError } from "@/lib/api/query-error";
import { useMonitoringRun } from "@/lib/monitoring/queries";

interface RunDetailPanelProps {
  runId: string;
  pollWhileActive?: boolean;
  onBack: () => void;
}

export function RunDetailPanel({ runId, pollWhileActive, onBack }: RunDetailPanelProps) {
  const t = useTranslations("domainDetail.monitoring");
  const tCommon = useTranslations("common");
  const query = useMonitoringRun(runId, { pollWhileActive: pollWhileActive ?? false });

  return (
    <div className="space-y-4">
      <Button variant="ghost" size="sm" onClick={onBack}>
        <ArrowLeft size={14} aria-hidden />
        {t("runsTitle")}
      </Button>

      {query.isError ? (
        <ErrorState
          title={tCommon("loadError")}
          description={describeQueryError(query.error).message}
          requestId={describeQueryError(query.error).requestId}
          onRetry={() => query.refetch()}
        />
      ) : query.isPending ? (
        <Skeleton className="h-64 w-full" />
      ) : (
        <>
          <Card>
            <CardHeader>
              <CardTitle>{t("runDetailTitle")}</CardTitle>
            </CardHeader>
            <CardContent className="space-y-3">
              <dl className="grid grid-cols-2 gap-3 text-sm sm:grid-cols-4">
                <div>
                  <dt className="text-xs text-muted-foreground">{t("statusLabel")}</dt>
                  <dd className="text-foreground">{query.data.run.status}</dd>
                </div>
                <div>
                  <dt className="text-xs text-muted-foreground">{t("triggerLabel")}</dt>
                  <dd className="text-foreground">{query.data.run.trigger_type}</dd>
                </div>
                <div>
                  <dt className="text-xs text-muted-foreground">{t("scheduledLabel")}</dt>
                  <dd className="text-foreground">
                    <DateValue value={query.data.run.scheduled_for} />
                  </dd>
                </div>
                <div>
                  <dt className="text-xs text-muted-foreground">{t("policyLabel")}</dt>
                  <dd className="text-foreground">{query.data.run.policy_version}</dd>
                </div>
              </dl>
              {query.data.run.last_error_message && (
                <p className="text-sm text-status-rose-fg">{query.data.run.last_error_message}</p>
              )}
              <JsonViewer data={query.data.run} label={t("rawRun")} />
            </CardContent>
          </Card>

          {query.data.results.map((result) => (
            <Card key={result.id}>
              <CardHeader>
                <CardTitle>
                  {t("vantage")}: {result.vantage_type} ({result.vantage_key})
                </CardTitle>
              </CardHeader>
              <CardContent className="space-y-3">
                {(result.vantage_country || result.vantage_network) && (
                  <p className="text-sm text-muted-foreground">
                    {t("networkScope")}:{" "}
                    {[result.vantage_country, result.vantage_network].filter(Boolean).join(" · ")}
                  </p>
                )}
                <div className="flex flex-wrap gap-1.5">
                  <DomainStatusBadge dimension="availability" value={result.availability_status} />
                  <DomainStatusBadge dimension="dns" value={result.dns_status} />
                  <DomainStatusBadge dimension="http" value={result.http_status} />
                  <DomainStatusBadge dimension="redirect" value={result.redirect_status} />
                  <DomainStatusBadge dimension="isp" value={result.isp_status} />
                  <DomainStatusBadge dimension="tls" value={result.tls_status} />
                </div>
                {result.error_message && (
                  <p className="text-sm text-status-rose-fg">{result.error_message}</p>
                )}
                {(result.failure_stage || result.error_code) && (
                  <p className="text-xs text-muted-foreground">
                    {t("failureEvidence")}:{" "}
                    {[result.failure_stage, result.error_code].filter(Boolean).join(" · ")}
                  </p>
                )}
                {result.reason_codes.length > 0 && (
                  <div className="rounded-md border border-border bg-muted/30 p-2 text-sm">
                    <p className="font-medium text-foreground">{t("restrictionEvidence")}</p>
                    <ul className="mt-1 list-disc pl-5 text-xs text-muted-foreground">
                      {result.reason_codes.map((reason) => (
                        <li key={reason}>{reason}</li>
                      ))}
                    </ul>
                  </div>
                )}
                <JsonViewer data={result} label={t("rawResult")} />
              </CardContent>
            </Card>
          ))}

          {query.data.dns_checks.length > 0 && (
            <Card>
              <CardHeader>
                <CardTitle>{t("dnsChecksTitle")}</CardTitle>
              </CardHeader>
              <CardContent className="space-y-2">
                {query.data.dns_checks.map((check) => (
                  <div key={check.id} className="rounded-md border border-border p-2 text-sm">
                    <p className="text-foreground">
                      {check.resolver_type} · {check.query_name} ({check.query_type}) ·{" "}
                      {check.rcode ?? check.error_code ?? "—"}
                    </p>
                    {check.answers.length > 0 && (
                      <ul className="mt-1 text-xs text-muted-foreground">
                        {check.answers.map((answer, index) => (
                          <li key={index}>
                            {answer.type} {answer.value} (TTL {answer.ttl_seconds}s)
                          </li>
                        ))}
                      </ul>
                    )}
                    <JsonViewer data={check} label={t("rawDnsCheck")} />
                  </div>
                ))}
              </CardContent>
            </Card>
          )}

          {query.data.http_checks.length > 0 && (
            <Card>
              <CardHeader>
                <CardTitle>{t("httpChecksTitle")}</CardTitle>
              </CardHeader>
              <CardContent className="space-y-3">
                {query.data.http_checks.map((check) => (
                  <div key={check.id} className="rounded-md border border-border p-2 text-sm">
                    <p className="text-foreground">
                      {check.scheme} {check.request_url} → {check.final_status_code ?? "—"}
                    </p>
                    {check.title && <p className="text-xs text-muted-foreground">{check.title}</p>}

                    <h4 className="mt-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                      {t("redirectChainTitle")}
                    </h4>
                    {check.redirects.length === 0 ? (
                      <p className="text-xs text-muted-foreground">{t("noRedirects")}</p>
                    ) : (
                      <ol className="text-xs text-muted-foreground">
                        {check.redirects.map((hop) => (
                          <li key={hop.hop}>
                            {hop.hop}. {hop.source_url} ({hop.status_code}) → {hop.location}
                            {hop.https_downgrade ? " ⚠ HTTPS downgrade" : ""}
                          </li>
                        ))}
                      </ol>
                    )}

                    {check.tls_check && (
                      <>
                        <h4 className="mt-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                          {t("tlsTitle")}
                        </h4>
                        <p className="text-xs text-muted-foreground">
                          {check.tls_check.tls_version ?? "—"} ·{" "}
                          {check.tls_check.certificate_expiration_days != null
                            ? `${check.tls_check.certificate_expiration_days}d`
                            : "—"}
                        </p>
                      </>
                    )}

                    <JsonViewer data={check} label={t("rawHttpCheck")} />
                  </div>
                ))}
              </CardContent>
            </Card>
          )}

          {query.data.results.length === 0 &&
            query.data.dns_checks.length === 0 &&
            query.data.http_checks.length === 0 && <EmptyState title={t("noRuns")} />}
        </>
      )}
    </div>
  );
}
