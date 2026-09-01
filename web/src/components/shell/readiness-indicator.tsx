"use client";

import { useQuery } from "@tanstack/react-query";
import { useLocale, useTranslations } from "next-intl";
import { z } from "zod";
import { CheckCircle2, HelpCircle, Loader2, XCircle } from "lucide-react";
import { bffFetch } from "@/lib/api/client";
import { ApiError } from "@/lib/api/envelope";
import { cn } from "@/lib/utils/cn";

const readyDataSchema = z.object({
  status: z.string(),
  checks: z.record(z.string(), z.string()),
});

type ReadinessState = "checking" | "ready" | "degraded" | "unreachable";

const STATE_STYLES: Record<ReadinessState, string> = {
  checking: "bg-status-slate-bg text-status-slate-fg border-status-slate-border",
  ready: "bg-status-emerald-bg text-status-emerald-fg border-status-emerald-border",
  degraded: "bg-status-amber-bg text-status-amber-fg border-status-amber-border",
  unreachable: "bg-status-rose-bg text-status-rose-fg border-status-rose-border",
};

/**
 * Polls the real /ready endpoint (proxied via the BFF allowlist) every 30s.
 * refetchIntervalInBackground:false pauses polling while the tab is
 * hidden, per web/FRONTEND_ARCHITECTURE.md section 4. Never fabricates a
 * "ready" state — degraded/unreachable are shown explicitly and distinctly
 * from a true 200 response.
 */
export function ReadinessIndicator() {
  const t = useTranslations("readiness");
  const locale = useLocale();

  const query = useQuery({
    queryKey: ["system-ready"],
    queryFn: () =>
      bffFetch("/api/bff/system/ready", readyDataSchema, {
        method: "GET",
        locale,
      }),
    refetchInterval: 30_000,
    refetchIntervalInBackground: false,
    retry: false,
  });

  let state: ReadinessState;
  if (query.isLoading) {
    state = "checking";
  } else if (query.isSuccess) {
    state = "ready";
  } else if (query.error instanceof ApiError && query.error.status === 503) {
    state = "degraded";
  } else {
    state = "unreachable";
  }

  const icon = {
    checking: <Loader2 size={14} className="animate-spin" aria-hidden />,
    ready: <CheckCircle2 size={14} aria-hidden />,
    degraded: <HelpCircle size={14} aria-hidden />,
    unreachable: <XCircle size={14} aria-hidden />,
  }[state];

  return (
    <div
      role="status"
      aria-label={t("label")}
      className={cn(
        "inline-flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-xs font-medium",
        STATE_STYLES[state],
      )}
    >
      {icon}
      <span>{t(state)}</span>
    </div>
  );
}
