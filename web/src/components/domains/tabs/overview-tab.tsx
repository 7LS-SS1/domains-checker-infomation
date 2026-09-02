"use client";

import { useTranslations } from "next-intl";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { DateValue } from "@/components/ui/date-value";
import { DomainStatusBadge } from "@/components/domains/domain-status-badge";
import type { Domain } from "@/lib/domains/types";

export function OverviewTab({ domain }: { domain: Domain; roles: readonly string[] }) {
  const t = useTranslations("domainDetail.overview");
  const tDomains = useTranslations("domains");
  const failureDetails = [domain.current_failure_stage, domain.current_error_code]
    .filter(Boolean)
    .join(" · ");

  return (
    <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
      <Card className="lg:col-span-2">
        <CardHeader>
          <CardTitle>{t("sheetSummaryTitle")}</CardTitle>
        </CardHeader>
        <CardContent>
          <dl className="grid grid-cols-1 gap-x-6 gap-y-5 text-sm sm:grid-cols-2 xl:grid-cols-3">
            <div>
              <dt className="text-xs text-muted-foreground">{t("renewalDecision")}</dt>
              <dd className="mt-1 text-foreground">
                {tDomains(`renewalDecision.${domain.renewal_decision}`)}
              </dd>
              {domain.renewal_decision_reason ? (
                <p className="mt-1 whitespace-pre-wrap text-xs text-muted-foreground">
                  {t("renewalReason")}: {domain.renewal_decision_reason}
                </p>
              ) : null}
            </div>
            <div>
              <dt className="text-xs text-muted-foreground">{t("websiteStatus")}</dt>
              <dd className="mt-1">
                <DomainStatusBadge dimension="http" value={domain.http_status} />
              </dd>
            </div>
            <div>
              <dt className="text-xs text-muted-foreground">{t("detailsReason")}</dt>
              <dd className="mt-1 whitespace-pre-wrap text-foreground">
                {failureDetails ||
                  (domain.last_checked_at ? t("noErrorDetected") : t("noCheckData"))}
              </dd>
            </div>
            <div>
              <dt className="text-xs text-muted-foreground">{t("mainStatus")}</dt>
              <dd className="mt-1">
                <DomainStatusBadge dimension="availability" value={domain.availability_status} />
              </dd>
            </div>
            <div className="min-w-0">
              <dt className="text-xs text-muted-foreground">{t("redirectTarget")}</dt>
              <dd
                className="mt-1 break-all text-foreground"
                title={domain.redirect_target_url ?? undefined}
              >
                {domain.redirect_target_url || t("noData")}
              </dd>
            </div>
            <div>
              <dt className="text-xs text-muted-foreground">{t("httpStatus")}</dt>
              <dd className="mt-1 tabular-nums text-foreground">
                {domain.latest_http_status_code ?? t("noData")}
              </dd>
            </div>
          </dl>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t("effectiveStateTitle")}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          <p className="text-xs text-muted-foreground">{t("effectiveStateDescription")}</p>
          <dl className="grid grid-cols-2 gap-3 text-sm">
            <div>
              <dt className="text-xs text-muted-foreground">{t("consecutiveFailures")}</dt>
              <dd className="tabular-nums text-foreground">{domain.consecutive_failures}</dd>
            </div>
            <div>
              <dt className="text-xs text-muted-foreground">{t("consecutiveSuccesses")}</dt>
              <dd className="tabular-nums text-foreground">{domain.consecutive_successes}</dd>
            </div>
            {domain.current_failure_stage && (
              <div className="col-span-2">
                <dt className="text-xs text-muted-foreground">{t("failureStage")}</dt>
                <dd className="text-foreground">{domain.current_failure_stage}</dd>
              </div>
            )}
            {domain.current_error_code && (
              <div className="col-span-2">
                <dt className="text-xs text-muted-foreground">{t("errorCode")}</dt>
                <dd className="text-foreground">{domain.current_error_code}</dd>
              </div>
            )}
          </dl>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t("detailsTitle")}</CardTitle>
        </CardHeader>
        <CardContent>
          <dl className="space-y-3 text-sm">
            <div>
              <dt className="text-xs text-muted-foreground">{t("originalInput")}</dt>
              <dd className="text-foreground">{domain.original_input}</dd>
            </div>
            <div>
              <dt className="text-xs text-muted-foreground">{t("registrableDomain")}</dt>
              <dd className="text-foreground">{domain.registrable_domain}</dd>
            </div>
            <div>
              <dt className="text-xs text-muted-foreground">{t("confidence")}</dt>
              <dd className="tabular-nums text-foreground">{domain.confidence_score}</dd>
            </div>
            <div>
              <dt className="text-xs text-muted-foreground">{t("contentMode")}</dt>
              <dd className="text-foreground">{domain.expected_content_mode}</dd>
            </div>
            <div>
              <dt className="text-xs text-muted-foreground">{t("monitoringLabel")}</dt>
              <dd>
                <DomainStatusBadge
                  dimension="source"
                  value={domain.monitoring_enabled ? "present" : "missing_from_source"}
                />
              </dd>
            </div>
            <div>
              <dt className="text-xs text-muted-foreground">{t("notes")}</dt>
              <dd className="whitespace-pre-wrap text-foreground">
                {domain.notes || t("noNotes")}
              </dd>
            </div>
            <div>
              <dt className="text-xs text-muted-foreground">{t("created")}</dt>
              <dd className="text-foreground">
                <DateValue value={domain.created_at} />
              </dd>
            </div>
          </dl>
        </CardContent>
      </Card>
    </div>
  );
}
