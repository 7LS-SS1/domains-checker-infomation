"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";
import { CheckCircle2, HelpCircle, Loader2, XCircle } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { StatusBadge } from "@/components/ui/status-badge";
import { DateValue } from "@/components/ui/date-value";
import { LanguageSelector } from "@/components/shell/language-selector";
import { ApiError } from "@/lib/api/envelope";
import { useHealth, useReady, useLocalesMeta } from "@/lib/system/queries";
import { useDriveConnection } from "@/lib/drive/queries";
import type { SessionUser } from "@/lib/auth/session";
import type { StatusTone } from "@/lib/status/tokens";

function readyStatusTone(status: string): StatusTone {
  return status === "ok" ? "emerald" : status === "degraded" ? "amber" : "rose";
}

export function SettingsPageContent({
  session,
  buildVersion,
}: {
  session: SessionUser;
  buildVersion: string;
}) {
  const t = useTranslations("settingsPage");

  const healthQuery = useHealth();
  const readyQuery = useReady();
  const localesQuery = useLocalesMeta();
  const driveQuery = useDriveConnection();

  const isDriveConnected = driveQuery.isSuccess && driveQuery.data.status === "active";
  const isDriveNotConfigured =
    driveQuery.isError &&
    driveQuery.error instanceof ApiError &&
    driveQuery.error.code === "DRIVE_NOT_CONFIGURED";

  return (
    <div className="space-y-4">
      <h1 className="text-xl font-semibold text-foreground">{t("title")}</h1>

      <Card>
        <CardHeader>
          <CardTitle>{t("profile.title")}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-2 text-sm">
          <p className="text-xs text-muted-foreground">{t("profile.readOnlyNotice")}</p>
          <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
            <div>
              <p className="text-xs text-muted-foreground">{t("profile.email")}</p>
              <p className="text-foreground">{session.email}</p>
            </div>
            <div>
              <p className="text-xs text-muted-foreground">{t("profile.displayName")}</p>
              <p className="text-foreground">{session.displayName}</p>
            </div>
            <div>
              <p className="text-xs text-muted-foreground">{t("profile.roles")}</p>
              <div className="flex flex-wrap gap-1">
                {session.roles.map((role) => (
                  <span
                    key={role}
                    className="rounded-full border border-border px-2 py-0.5 text-xs text-foreground"
                  >
                    {role}
                  </span>
                ))}
              </div>
            </div>
            <div>
              <p className="text-xs text-muted-foreground">{t("profile.accountLocale")}</p>
              <p className="text-foreground">{session.locale}</p>
            </div>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t("language.title")}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-2">
          <p className="text-xs text-muted-foreground">{t("language.description")}</p>
          <LanguageSelector />
          {localesQuery.data && (
            <p className="text-xs text-muted-foreground">
              {t("language.supported")}: {localesQuery.data.supported.join(", ")}
            </p>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t("system.title")}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3 text-sm">
          {healthQuery.isPending ? (
            <Skeleton className="h-16 w-full" />
          ) : healthQuery.isError ? (
            <p className="text-status-rose-fg">{t("system.unreachable")}</p>
          ) : (
            <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
              <div>
                <p className="text-xs text-muted-foreground">{t("system.service")}</p>
                <p className="text-foreground">{healthQuery.data.service}</p>
              </div>
              <div>
                <p className="text-xs text-muted-foreground">{t("system.backendVersion")}</p>
                <p className="text-foreground">
                  {healthQuery.data.version} ({healthQuery.data.commit.slice(0, 12)})
                </p>
              </div>
              <div>
                <p className="text-xs text-muted-foreground">{t("system.startedAt")}</p>
                <p className="text-foreground">
                  <DateValue value={healthQuery.data.started_at} />
                </p>
              </div>
              <div>
                <p className="text-xs text-muted-foreground">{t("system.frontendVersion")}</p>
                <p className="text-foreground">{buildVersion}</p>
              </div>
            </div>
          )}

          <div className="space-y-1.5">
            <p className="text-xs font-medium text-muted-foreground">{t("system.readiness")}</p>
            {readyQuery.isPending ? (
              <Skeleton className="h-10 w-full" />
            ) : readyQuery.isError ? (
              <StatusBadge tone="rose" icon={XCircle} label={t("system.unreachable")} />
            ) : (
              <div className="space-y-1">
                <StatusBadge
                  tone={readyStatusTone(readyQuery.data.status)}
                  icon={readyQuery.data.status === "ok" ? CheckCircle2 : HelpCircle}
                  label={readyQuery.data.status}
                />
                <ul className="space-y-0.5 text-xs text-muted-foreground">
                  {Object.entries(readyQuery.data.checks).map(([check, status]) => (
                    <li key={check}>
                      {check}: <span className="text-foreground">{status}</span>
                    </li>
                  ))}
                </ul>
              </div>
            )}
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t("drive.title")}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-2 text-sm">
          {driveQuery.isPending ? (
            <Skeleton className="h-8 w-full" />
          ) : isDriveNotConfigured ? (
            <p className="text-muted-foreground">{t("drive.notConfigured")}</p>
          ) : isDriveConnected ? (
            <StatusBadge tone="emerald" icon={CheckCircle2} label={t("drive.connected")} />
          ) : (
            <StatusBadge tone="slate" icon={Loader2} label={t("drive.notConnected")} />
          )}
          <Link href="/sync?tab=drive" className="text-xs text-primary hover:underline">
            {t("drive.manageLink")}
          </Link>
        </CardContent>
      </Card>
    </div>
  );
}
