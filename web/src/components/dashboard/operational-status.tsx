"use client";

import { useTranslations } from "next-intl";
import type { UseQueryResult } from "@tanstack/react-query";
import { CheckCircle2, HelpCircle, XCircle } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { StatusBadge } from "@/components/ui/status-badge";
import { ApiError } from "@/lib/api/envelope";
import type { Health, Ready } from "@/lib/system/types";
import type { StatusTone } from "@/lib/status/tokens";

function checkTone(value: string | undefined): StatusTone {
  if (value === "ok") return "emerald";
  if (value === undefined) return "slate";
  return "rose";
}

function checkIcon(value: string | undefined) {
  if (value === "ok") return CheckCircle2;
  if (value === undefined) return HelpCircle;
  return XCircle;
}

interface OperationalStatusProps {
  healthQuery: UseQueryResult<Health>;
  readyQuery: UseQueryResult<Ready>;
}

export function OperationalStatus({ healthQuery, readyQuery }: OperationalStatusProps) {
  const t = useTranslations("dashboard.operational");
  const tReadiness = useTranslations("readiness");

  // On a 503 the backend still reports which dependency failed inside
  // error.details.checks (READINESS_FAILED, internal/api/server.go ready()) —
  // use that instead of collapsing to an undifferentiated "down" state.
  const checksFromError =
    readyQuery.error instanceof ApiError &&
    readyQuery.error.details &&
    typeof readyQuery.error.details === "object" &&
    "checks" in readyQuery.error.details
      ? (readyQuery.error.details.checks as Record<string, string>)
      : undefined;
  const checks = readyQuery.data?.checks ?? checksFromError;

  const apiTone: StatusTone = healthQuery.isSuccess
    ? "emerald"
    : healthQuery.isError
      ? "rose"
      : "slate";
  const apiIcon = healthQuery.isSuccess ? CheckCircle2 : healthQuery.isError ? XCircle : HelpCircle;
  const apiLabel = healthQuery.isSuccess
    ? tReadiness("ready")
    : healthQuery.isError
      ? tReadiness("unreachable")
      : tReadiness("checking");

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("title")}</CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        <div className="flex items-center justify-between">
          <span className="text-sm text-foreground">{t("api")}</span>
          {healthQuery.isPending ? (
            <Skeleton className="h-6 w-20" />
          ) : (
            <StatusBadge tone={apiTone} icon={apiIcon} label={apiLabel} />
          )}
        </div>
        {healthQuery.isSuccess && (
          <p className="text-xs text-muted-foreground">
            {t("version")}: {healthQuery.data.version}
            {healthQuery.data.commit ? ` (${healthQuery.data.commit})` : ""}
          </p>
        )}

        <div className="flex items-center justify-between">
          <span className="text-sm text-foreground">{t("postgres")}</span>
          {readyQuery.isPending ? (
            <Skeleton className="h-6 w-20" />
          ) : (
            <StatusBadge
              tone={checkTone(checks?.postgres)}
              icon={checkIcon(checks?.postgres)}
              label={checks?.postgres ?? tReadiness("unreachable")}
            />
          )}
        </div>

        <div className="flex items-center justify-between">
          <span className="text-sm text-foreground">{t("redis")}</span>
          {readyQuery.isPending ? (
            <Skeleton className="h-6 w-20" />
          ) : (
            <StatusBadge
              tone={checkTone(checks?.redis)}
              icon={checkIcon(checks?.redis)}
              label={checks?.redis ?? tReadiness("unreachable")}
            />
          )}
        </div>
      </CardContent>
    </Card>
  );
}
