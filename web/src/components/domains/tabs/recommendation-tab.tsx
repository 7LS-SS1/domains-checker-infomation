"use client";

import { useLocale, useTranslations } from "next-intl";
import { Loader2, Sparkles } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { DateValue } from "@/components/ui/date-value";
import { StatusBadge } from "@/components/ui/status-badge";
import { ApiError } from "@/lib/api/envelope";
import { describeQueryError } from "@/lib/api/query-error";
import {
  useDomainRecommendation,
  useGenerateRecommendation,
} from "@/lib/recommendations/domain-queries";
import { recommendationActionIcon, recommendationActionTone } from "@/lib/recommendations/status";
import { hasCapability, toKnownRoles } from "@/lib/auth/capability";

export function RecommendationTab({
  domainId,
  roles,
}: {
  domainId: string;
  roles: readonly string[];
}) {
  const t = useTranslations("domainDetail.recommendation");
  const tActions = useTranslations("recommendations.actions");
  const tCommon = useTranslations("common");
  const locale = useLocale();
  const query = useDomainRecommendation(domainId);
  const generate = useGenerateRecommendation();
  const canGenerate = hasCapability(toKnownRoles(roles), "generateRecommendation");

  const isNotFound =
    query.isError &&
    query.error instanceof ApiError &&
    query.error.code === "RECOMMENDATION_NOT_FOUND";

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("title")}</CardTitle>
        {canGenerate && (
          <Button size="sm" disabled={generate.isPending} onClick={() => generate.mutate(domainId)}>
            {generate.isPending ? (
              <Loader2 size={14} className="animate-spin" aria-hidden />
            ) : (
              <Sparkles size={14} aria-hidden />
            )}
            {t("generate")}
          </Button>
        )}
      </CardHeader>
      <CardContent className="space-y-4">
        {query.isPending ? (
          <Skeleton className="h-40 w-full" />
        ) : isNotFound ? (
          <EmptyState title={t("notGenerated")} />
        ) : query.isError ? (
          <ErrorState
            title={tCommon("loadError")}
            description={describeQueryError(query.error).message}
            requestId={describeQueryError(query.error).requestId}
            onRetry={() => query.refetch()}
          />
        ) : (
          <>
            {query.data.manual_override && (
              <div className="rounded-md border border-status-violet-border bg-status-violet-bg p-3 text-sm text-status-violet-fg">
                <p className="font-medium">{t("overrideBanner")}</p>
                <p className="text-xs">
                  {t("overrideReason")}: {query.data.manual_override.reason}
                </p>
              </div>
            )}

            <div className="flex flex-wrap gap-4">
              <div>
                <p className="text-xs text-muted-foreground">{t("generatedAction")}</p>
                <StatusBadge
                  tone={recommendationActionTone(query.data.action)}
                  icon={recommendationActionIcon(query.data.action)}
                  label={tActions(query.data.action)}
                />
              </div>
              <div>
                <p className="text-xs text-muted-foreground">{t("effectiveAction")}</p>
                <StatusBadge
                  tone={recommendationActionTone(query.data.effective_action)}
                  icon={recommendationActionIcon(query.data.effective_action)}
                  label={tActions(query.data.effective_action)}
                />
              </div>
              <div>
                <p className="text-xs text-muted-foreground">{t("opportunityLevel")}</p>
                <p className="text-sm text-foreground">{query.data.opportunity_level}</p>
              </div>
              <div>
                <p className="text-xs text-muted-foreground">{t("confidence")}</p>
                <p className="text-sm text-foreground">
                  {query.data.confidence_score} ({query.data.confidence_level})
                </p>
              </div>
            </div>

            <div>
              <h3 className="mb-1 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                {t("reasonsTitle")}
              </h3>
              <ul className="list-inside list-disc text-sm text-foreground">
                {(locale === "th" ? query.data.reasons_th : query.data.reasons_en).map(
                  (reason, index) => (
                    <li key={index}>{reason}</li>
                  ),
                )}
              </ul>
            </div>

            {query.data.evidence_refs.length > 0 && (
              <div>
                <h3 className="mb-1 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                  {t("evidenceTitle")}
                </h3>
                <p className="text-xs text-muted-foreground">
                  {query.data.evidence_refs.join(", ")}
                </p>
              </div>
            )}

            <p className="text-xs text-muted-foreground">
              {t("policyVersion")}: {query.data.policy_version} ·{" "}
              <DateValue value={query.data.generated_at} />
            </p>
          </>
        )}
      </CardContent>
    </Card>
  );
}
