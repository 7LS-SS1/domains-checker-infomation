"use client";

import { useState } from "react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { Loader2, Sparkles } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { DateValue } from "@/components/ui/date-value";
import { StatusBadge } from "@/components/ui/status-badge";
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "@/components/ui/dialog";
import { describeQueryError } from "@/lib/api/query-error";
import { useRecommendationsList, useRunRecommendations } from "@/lib/recommendations/list-queries";
import { recommendationActionIcon, recommendationActionTone } from "@/lib/recommendations/status";
import { RECOMMENDATION_ACTIONS } from "@/lib/recommendations/types";
import { ApiError } from "@/lib/api/envelope";
import { hasCapability, toKnownRoles } from "@/lib/auth/capability";

const DEFAULT_LIMIT = 100;
const RUN_LIMIT = 500;

export function RecommendationsPageContent({ roles }: { roles: readonly string[] }) {
  const t = useTranslations("recommendationsPage");
  const tActions = useTranslations("recommendations.actions");
  const tCommon = useTranslations("common");
  const [action, setAction] = useState("");
  const [runDialogOpen, setRunDialogOpen] = useState(false);

  const query = useRecommendationsList({ action, limit: DEFAULT_LIMIT });
  const runMutation = useRunRecommendations();
  const canRun = hasCapability(toKnownRoles(roles), "generateRecommendation");

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-xl font-semibold text-foreground">{t("title")}</h1>
        {canRun && (
          <Button size="sm" onClick={() => setRunDialogOpen(true)} disabled={runMutation.isPending}>
            <Sparkles size={14} aria-hidden />
            {t("runBulk")}
          </Button>
        )}
      </div>

      <div className="flex items-center gap-2">
        <label htmlFor="rec-action" className="text-xs font-medium text-muted-foreground">
          {t("filterAction")}
        </label>
        <select
          id="rec-action"
          value={action}
          onChange={(event) => setAction(event.target.value)}
          className="h-10 rounded-md border border-border bg-background px-2 text-sm text-foreground"
        >
          <option value="">{t("allOption")}</option>
          {RECOMMENDATION_ACTIONS.map((value) => (
            <option key={value} value={value}>
              {tActions(value)}
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
      ) : query.data.length === 0 ? (
        <EmptyState title={t("empty")} />
      ) : (
        <div
          className="overflow-x-auto rounded-lg border border-border"
          tabIndex={0}
          role="region"
          aria-label={t("title")}
        >
          <table className="w-full min-w-[820px] text-sm">
            <thead>
              <tr className="border-b border-border bg-surface text-left text-xs font-medium text-muted-foreground">
                <th scope="col" className="px-3 py-2">
                  {t("columns.domain")}
                </th>
                <th scope="col" className="px-3 py-2">
                  {t("columns.generatedAction")}
                </th>
                <th scope="col" className="px-3 py-2">
                  {t("columns.effectiveAction")}
                </th>
                <th scope="col" className="px-3 py-2">
                  {t("columns.opportunityLevel")}
                </th>
                <th scope="col" className="px-3 py-2">
                  {t("columns.confidence")}
                </th>
                <th scope="col" className="px-3 py-2">
                  {t("columns.generatedAt")}
                </th>
              </tr>
            </thead>
            <tbody>
              {query.data.map((recommendation) => (
                <tr key={recommendation.id} className="border-b border-border last:border-0">
                  <td className="px-3 py-2">
                    <Link
                      href={`/domains/${recommendation.domain_id}`}
                      className="font-medium text-foreground hover:underline"
                    >
                      {recommendation.domain}
                    </Link>
                  </td>
                  <td className="px-3 py-2">
                    <StatusBadge
                      tone={recommendationActionTone(recommendation.action)}
                      icon={recommendationActionIcon(recommendation.action)}
                      label={tActions(recommendation.action)}
                    />
                  </td>
                  <td className="px-3 py-2">
                    <div className="flex items-center gap-1.5">
                      <StatusBadge
                        tone={recommendationActionTone(recommendation.effective_action)}
                        icon={recommendationActionIcon(recommendation.effective_action)}
                        label={tActions(recommendation.effective_action)}
                      />
                      {recommendation.manual_override && (
                        <span className="text-xs text-status-violet-fg">
                          ({t("overrideIndicator")})
                        </span>
                      )}
                    </div>
                  </td>
                  <td className="px-3 py-2 text-muted-foreground">
                    {recommendation.opportunity_level}
                  </td>
                  <td className="px-3 py-2 tabular-nums text-muted-foreground">
                    {recommendation.confidence_score} ({recommendation.confidence_level})
                  </td>
                  <td className="px-3 py-2 text-muted-foreground">
                    <DateValue value={recommendation.generated_at} />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {action === "PROFIT_OPPORTUNITY" && (
        <p className="text-xs text-muted-foreground">{t("profitOpportunityDisclaimer")}</p>
      )}
      {action === "REVIEW" && (
        <p className="text-xs text-muted-foreground">{t("reviewDisclaimer")}</p>
      )}

      {canRun && (
        <Dialog open={runDialogOpen} onOpenChange={setRunDialogOpen}>
          <DialogContent closeLabel={tCommon("cancel")}>
            <DialogTitle>{t("runConfirmTitle")}</DialogTitle>
            <DialogDescription>
              {t("runConfirmDescription", { limit: RUN_LIMIT })}
            </DialogDescription>
            {runMutation.isError && (
              <p role="alert" className="mt-2 text-sm text-status-rose-fg">
                {runMutation.error instanceof ApiError
                  ? runMutation.error.message
                  : tCommon("loadError")}
              </p>
            )}
            <div className="mt-4 flex justify-end gap-2">
              <Button variant="secondary" onClick={() => setRunDialogOpen(false)}>
                {tCommon("cancel")}
              </Button>
              <Button
                disabled={runMutation.isPending}
                onClick={() =>
                  runMutation.mutate(RUN_LIMIT, { onSuccess: () => setRunDialogOpen(false) })
                }
              >
                {runMutation.isPending && (
                  <Loader2 className="animate-spin" size={16} aria-hidden />
                )}
                {tCommon("confirm")}
              </Button>
            </div>
          </DialogContent>
        </Dialog>
      )}
    </div>
  );
}
