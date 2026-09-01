"use client";

import { useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { CheckCircle2, Clock, HelpCircle, Loader2, XCircle } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { DateValue } from "@/components/ui/date-value";
import { StatusBadge } from "@/components/ui/status-badge";
import { describeQueryError } from "@/lib/api/query-error";
import { useMonitoringHistory, useMonitoringRuns } from "@/lib/monitoring/queries";
import { HISTORY_WINDOWS, type HistoryWindow, type MonitoringRun } from "@/lib/monitoring/types";
import { RunDetailPanel } from "@/components/domains/tabs/run-detail-panel";

const NON_TERMINAL_STATUSES = new Set(["queued", "running"]);
const POLL_INTERVAL_MS = 3_000;
// ~66s at 3s/attempt, just past the backend's own MONITOR_RUN_TIMEOUT
// default of 60s (internal/config/config.go) — an attempt-count bound
// rather than a wall-clock one, so it never needs Date.now() during render.
const MAX_POLL_ATTEMPTS = 22;

function runStatusTone(status: string) {
  switch (status) {
    case "completed":
      return "emerald" as const;
    case "partial":
      return "amber" as const;
    case "failed":
    case "cancelled":
      return "rose" as const;
    default:
      return "slate" as const;
  }
}

export function MonitoringTab({ domainId }: { domainId: string }) {
  const t = useTranslations("domainDetail.monitoring");
  const tCommon = useTranslations("common");
  const [page, setPage] = useState(1);
  const [window, setWindow] = useState<HistoryWindow>("24h");
  const [selectedRunId, setSelectedRunId] = useState<string | null>(null);

  const [pollAttempts, setPollAttempts] = useState(0);

  const runsQuery = useMonitoringRuns(domainId, page, 20);
  const historyQuery = useMonitoringHistory(domainId, window);

  const hasNonTerminalRun = (runsQuery.data?.items ?? []).some((run: MonitoringRun) =>
    NON_TERMINAL_STATUSES.has(run.status),
  );
  const pollExhausted = hasNonTerminalRun && pollAttempts >= MAX_POLL_ATTEMPTS;

  // Reset the attempt counter once nothing is in flight anymore — state
  // adjustment during render (React's recommended pattern), not an effect.
  const [trackedNonTerminal, setTrackedNonTerminal] = useState(hasNonTerminalRun);
  if (hasNonTerminalRun !== trackedNonTerminal) {
    setTrackedNonTerminal(hasNonTerminalRun);
    if (!hasNonTerminalRun) {
      setPollAttempts(0);
    }
  }

  useEffect(() => {
    if (!hasNonTerminalRun || pollAttempts >= MAX_POLL_ATTEMPTS) {
      return;
    }
    const timer = setTimeout(() => {
      setPollAttempts((count) => count + 1);
      void runsQuery.refetch();
    }, POLL_INTERVAL_MS);
    return () => clearTimeout(timer);
    // eslint-disable-next-line react-hooks/exhaustive-deps -- runsQuery identity changes every render; only re-schedule on attempt/state changes
  }, [hasNonTerminalRun, pollAttempts]);

  if (selectedRunId) {
    return (
      <RunDetailPanel runId={selectedRunId} pollWhileActive onBack={() => setSelectedRunId(null)} />
    );
  }

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle>{t("historyTitle")}</CardTitle>
          <div className="flex gap-1">
            {HISTORY_WINDOWS.map((value) => (
              <button
                key={value}
                type="button"
                onClick={() => setWindow(value)}
                aria-pressed={window === value}
                className={`rounded-md px-2 py-1 text-xs font-medium ${
                  window === value
                    ? "bg-primary text-primary-foreground"
                    : "text-muted-foreground hover:bg-surface"
                }`}
              >
                {t(`window${value}`)}
              </button>
            ))}
          </div>
        </CardHeader>
        <CardContent>
          {historyQuery.isError ? (
            <ErrorState
              title={tCommon("loadError")}
              description={describeQueryError(historyQuery.error).message}
              requestId={describeQueryError(historyQuery.error).requestId}
              onRetry={() => historyQuery.refetch()}
            />
          ) : historyQuery.isPending ? (
            <Skeleton className="h-24 w-full" />
          ) : (
            <>
              <div className="grid grid-cols-2 gap-3 sm:grid-cols-3">
                <Stat
                  label={t("uptimePercentage")}
                  value={
                    historyQuery.data.aggregate.uptime_percentage == null
                      ? t("insufficientData")
                      : `${historyQuery.data.aggregate.uptime_percentage.toFixed(2)}%`
                  }
                />
                <Stat
                  label={t("monitoringCoverage")}
                  value={`${(historyQuery.data.aggregate.monitoring_coverage * 100).toFixed(1)}%`}
                />
                <Stat
                  label={t("incidentCount")}
                  value={String(historyQuery.data.aggregate.incident_count)}
                />
                <Stat
                  label={t("statusChangeCount")}
                  value={String(historyQuery.data.aggregate.status_change_count)}
                />
                <Stat
                  label={t("averageResponseMs")}
                  value={
                    historyQuery.data.aggregate.average_response_ms == null
                      ? t("insufficientData")
                      : `${historyQuery.data.aggregate.average_response_ms.toFixed(0)} ms`
                  }
                />
              </div>

              <h3 className="mb-2 mt-4 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                {t("timelineTitle")}
              </h3>
              {historyQuery.data.timeline.length === 0 ? (
                <EmptyState title={t("noTimelineEntries")} className="p-4" />
              ) : (
                <ul className="divide-y divide-border text-sm">
                  {historyQuery.data.timeline.map((entry) => (
                    <li key={entry.id} className="flex items-center justify-between gap-3 py-2">
                      <span className="text-foreground">
                        {entry.dimension}: {entry.previous_value ?? "—"} → {entry.current_value}
                      </span>
                      <DateValue
                        value={entry.effective_at}
                        className="text-xs text-muted-foreground"
                      />
                    </li>
                  ))}
                </ul>
              )}
            </>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t("runsTitle")}</CardTitle>
          {pollExhausted && (
            <Button variant="secondary" size="sm" onClick={() => runsQuery.refetch()}>
              {tCommon("retry")}
            </Button>
          )}
        </CardHeader>
        <CardContent>
          {runsQuery.isError ? (
            <ErrorState
              title={tCommon("loadError")}
              description={describeQueryError(runsQuery.error).message}
              requestId={describeQueryError(runsQuery.error).requestId}
              onRetry={() => runsQuery.refetch()}
            />
          ) : runsQuery.isPending ? (
            <Skeleton className="h-32 w-full" />
          ) : runsQuery.data.items.length === 0 ? (
            <EmptyState title={t("noRuns")} />
          ) : (
            <ul className="divide-y divide-border text-sm">
              {runsQuery.data.items.map((run) => (
                <li key={run.id} className="flex items-center justify-between gap-3 py-2">
                  <button
                    type="button"
                    onClick={() => setSelectedRunId(run.id)}
                    className="text-left text-foreground hover:underline"
                  >
                    {run.trigger_type} · <DateValue value={run.scheduled_for} />
                  </button>
                  <StatusBadge
                    tone={runStatusTone(run.status)}
                    icon={StatusIconFor(run.status)}
                    label={run.status}
                  />
                </li>
              ))}
            </ul>
          )}
          {runsQuery.data && runsQuery.data.total_pages > 1 && (
            <div className="mt-3 flex items-center justify-between gap-3 text-sm text-muted-foreground">
              <p>{tCommon("pageOf", { page, totalPages: runsQuery.data.total_pages })}</p>
              <div className="flex gap-2">
                <Button
                  variant="secondary"
                  size="sm"
                  disabled={page <= 1}
                  onClick={() => setPage((value) => value - 1)}
                >
                  {tCommon("previous")}
                </Button>
                <Button
                  variant="secondary"
                  size="sm"
                  disabled={page >= runsQuery.data.total_pages}
                  onClick={() => setPage((value) => value + 1)}
                >
                  {tCommon("next")}
                </Button>
              </div>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <p className="text-xs font-medium text-muted-foreground">{label}</p>
      <p className="text-lg font-semibold tabular-nums text-foreground">{value}</p>
    </div>
  );
}

function StatusIconFor(status: string) {
  switch (status) {
    case "completed":
      return CheckCircle2;
    case "partial":
      return HelpCircle;
    case "failed":
    case "cancelled":
      return XCircle;
    case "running":
      return Loader2;
    default:
      return Clock;
  }
}
