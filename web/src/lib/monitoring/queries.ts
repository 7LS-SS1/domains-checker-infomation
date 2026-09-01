"use client";

import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import { useLocale } from "next-intl";
import { bffFetch } from "@/lib/api/client";
import {
  monitoringRunDetailSchema,
  monitoringRunPageSchema,
  monitoringHistorySchema,
  type MonitoringRunDetail,
  type MonitoringRunPage,
  type MonitoringHistory,
  type HistoryWindow,
} from "@/lib/monitoring/types";

export function useMonitoringRuns(
  domainId: string | undefined,
  page: number,
  pageSize: number,
): UseQueryResult<MonitoringRunPage> {
  const locale = useLocale();
  return useQuery({
    queryKey: ["domain", domainId, "monitoring-runs", page, pageSize],
    queryFn: () =>
      bffFetch(
        `/api/bff/domains/${domainId}/monitoring-runs?page=${page}&page_size=${pageSize}`,
        monitoringRunPageSchema,
        { locale },
      ),
    enabled: Boolean(domainId),
  });
}

export function useMonitoringHistory(
  domainId: string | undefined,
  window: HistoryWindow,
): UseQueryResult<MonitoringHistory> {
  const locale = useLocale();
  return useQuery({
    queryKey: ["domain", domainId, "monitoring-history", window],
    queryFn: () =>
      bffFetch(
        `/api/bff/domains/${domainId}/monitoring-history?window=${window}`,
        monitoringHistorySchema,
        { locale },
      ),
    enabled: Boolean(domainId),
  });
}

/**
 * Polls a single run while it's in-flight (queued/running), per the bounded
 * polling rule in web/FRONTEND_ARCHITECTURE.md section 4 — the backend's
 * own MONITOR_RUN_TIMEOUT defaults to 60s
 * (internal/config/config.go MonitorRunTimeout), so refetchInterval is
 * capped to stop once the run reaches a terminal status.
 */
export function useMonitoringRun(
  runId: string | undefined,
  options: { pollWhileActive?: boolean } = {},
): UseQueryResult<MonitoringRunDetail> {
  const locale = useLocale();
  return useQuery({
    queryKey: ["monitoring-run", runId],
    queryFn: () =>
      bffFetch(`/api/bff/monitoring-runs/${runId}`, monitoringRunDetailSchema, { locale }),
    enabled: Boolean(runId),
    refetchInterval: (query) => {
      if (!options.pollWhileActive) return false;
      const status = query.state.data?.run.status;
      if (status === "queued" || status === "running") {
        return 3_000;
      }
      return false;
    },
  });
}
