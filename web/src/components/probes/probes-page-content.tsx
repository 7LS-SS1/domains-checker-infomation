"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { KeyRound, Unplug } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { StatusBadge } from "@/components/ui/status-badge";
import { DateValue } from "@/components/ui/date-value";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { describeQueryError } from "@/lib/api/query-error";
import { ApiError } from "@/lib/api/envelope";
import { useProbes } from "@/lib/probes/queries";
import { useRevokeProbe } from "@/lib/probes/mutations";
import { probeStatusIcon, probeStatusTone } from "@/lib/probes/status";
import { hasCapability, toKnownRoles } from "@/lib/auth/capability";
import { CreateRegistrationTokenDialog } from "@/components/probes/create-registration-token-dialog";
import type { ProbeNode } from "@/lib/probes/types";

export function ProbesPageContent({ roles }: { roles: readonly string[] }) {
  const t = useTranslations("probesPage");
  const tStatus = useTranslations("probeStatus");
  const tCommon = useTranslations("common");
  const [createTokenOpen, setCreateTokenOpen] = useState(false);
  const [revokeTarget, setRevokeTarget] = useState<ProbeNode | undefined>();

  const query = useProbes();
  const revokeMutation = useRevokeProbe();
  const canManage = hasCapability(toKnownRoles(roles), "manageProbes");

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h1 className="text-xl font-semibold text-foreground">{t("title")}</h1>
        {canManage && (
          <Button size="sm" onClick={() => setCreateTokenOpen(true)}>
            <KeyRound size={14} aria-hidden />
            {t("createToken")}
          </Button>
        )}
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
          {Array.from({ length: 4 }).map((_, index) => (
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
          <table className="w-full min-w-[920px] text-sm">
            <thead>
              <tr className="border-b border-border bg-surface text-left text-xs font-medium text-muted-foreground">
                <th scope="col" className="px-3 py-2">
                  {t("columns.name")}
                </th>
                <th scope="col" className="px-3 py-2">
                  {t("columns.region")}
                </th>
                <th scope="col" className="px-3 py-2">
                  {t("columns.status")}
                </th>
                <th scope="col" className="px-3 py-2">
                  {t("columns.version")}
                </th>
                <th scope="col" className="px-3 py-2">
                  {t("columns.lastSeen")}
                </th>
                <th scope="col" className="px-3 py-2">
                  {t("columns.clockOffset")}
                </th>
                <th scope="col" className="px-3 py-2">
                  {t("columns.registeredAt")}
                </th>
                {canManage && (
                  <th scope="col" className="px-3 py-2">
                    <span className="sr-only">{t("revoke")}</span>
                  </th>
                )}
              </tr>
            </thead>
            <tbody>
              {query.data.map((probe) => (
                <tr key={probe.id} className="border-b border-border last:border-0">
                  <td className="max-w-[200px] break-all px-3 py-2 font-medium text-foreground">
                    {probe.name}
                  </td>
                  <td className="px-3 py-2 text-muted-foreground">
                    {probe.region_code} / {probe.country_code}
                    {probe.network_name ? ` · ${probe.network_name}` : ""}
                  </td>
                  <td className="px-3 py-2">
                    <StatusBadge
                      tone={probeStatusTone(probe.status)}
                      icon={probeStatusIcon(probe.status)}
                      label={tStatus(probe.status)}
                    />
                  </td>
                  <td className="px-3 py-2 text-muted-foreground">{probe.version || "—"}</td>
                  <td className="px-3 py-2 text-muted-foreground">
                    {probe.last_seen_at ? <DateValue value={probe.last_seen_at} /> : t("never")}
                  </td>
                  <td className="px-3 py-2 tabular-nums text-muted-foreground">
                    {probe.clock_offset_ms == null ? "—" : `${probe.clock_offset_ms} ms`}
                  </td>
                  <td className="px-3 py-2 text-muted-foreground">
                    <DateValue value={probe.registered_at} />
                  </td>
                  {canManage && (
                    <td className="px-3 py-2 text-right">
                      {probe.status !== "REVOKED" && (
                        <Button
                          variant="secondary"
                          size="sm"
                          onClick={() => setRevokeTarget(probe)}
                        >
                          <Unplug size={14} aria-hidden />
                          {t("revoke")}
                        </Button>
                      )}
                    </td>
                  )}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {canManage && (
        <>
          <CreateRegistrationTokenDialog open={createTokenOpen} onOpenChange={setCreateTokenOpen} />
          <ConfirmDialog
            open={Boolean(revokeTarget)}
            onOpenChange={(next) => {
              if (!next) setRevokeTarget(undefined);
            }}
            title={t("revokeTitle", { name: revokeTarget?.name ?? "" })}
            description={t("revokeDescription")}
            reasonLabel={tCommon("reason")}
            reasonPlaceholder={tCommon("reasonPlaceholder")}
            confirmLabel={t("revoke")}
            cancelLabel={tCommon("cancel")}
            variant="destructive"
            isSubmitting={revokeMutation.isPending}
            errorMessage={
              revokeMutation.error instanceof ApiError ? revokeMutation.error.message : undefined
            }
            onConfirm={(reason) => {
              if (!revokeTarget) return;
              revokeMutation.mutate(
                { probeId: revokeTarget.id, reason },
                { onSuccess: () => setRevokeTarget(undefined) },
              );
            }}
          />
        </>
      )}
    </div>
  );
}
