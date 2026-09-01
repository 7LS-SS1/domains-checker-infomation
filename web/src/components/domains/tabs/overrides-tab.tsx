"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { Plus } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { DateValue } from "@/components/ui/date-value";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { describeQueryError } from "@/lib/api/query-error";
import { useDomainOverrides } from "@/lib/finance/queries";
import { useRevokeOverride } from "@/lib/finance/mutations";
import { ApiError } from "@/lib/api/envelope";
import { hasCapability, toKnownRoles } from "@/lib/auth/capability";
import { CreateOverrideDialog } from "@/components/domains/tabs/create-override-dialog";
import type { OverrideRecord } from "@/lib/finance/types";

export function OverridesTab({ domainId, roles }: { domainId: string; roles: readonly string[] }) {
  const t = useTranslations("domainDetail.overrides");
  const tCommon = useTranslations("common");
  const [createOpen, setCreateOpen] = useState(false);
  const [revokeTarget, setRevokeTarget] = useState<OverrideRecord | null>(null);
  const query = useDomainOverrides(domainId);
  const revoke = useRevokeOverride();
  const canManage = hasCapability(toKnownRoles(roles), "manageOverrides");

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("title")}</CardTitle>
        {canManage && (
          <Button size="sm" onClick={() => setCreateOpen(true)}>
            <Plus size={14} aria-hidden />
            {t("create")}
          </Button>
        )}
      </CardHeader>
      <CardContent>
        {query.isError ? (
          <ErrorState
            title={tCommon("loadError")}
            description={describeQueryError(query.error).message}
            requestId={describeQueryError(query.error).requestId}
            onRetry={() => query.refetch()}
          />
        ) : query.isPending ? (
          <Skeleton className="h-24 w-full" />
        ) : query.data.length === 0 ? (
          <EmptyState title={t("noOverrides")} />
        ) : (
          <ul className="divide-y divide-border text-sm">
            {query.data.map((override) => (
              <li key={override.id} className="flex items-center justify-between gap-3 py-2">
                <div>
                  <p className="font-medium text-foreground">{override.field_name}</p>
                  <p className="text-xs text-muted-foreground">
                    {override.reason} · <DateValue value={override.effective_from} />
                  </p>
                </div>
                {override.revoked_at ? (
                  <span className="text-xs text-muted-foreground">{t("revoked")}</span>
                ) : (
                  canManage && (
                    <Button variant="ghost" size="sm" onClick={() => setRevokeTarget(override)}>
                      {t("revoke")}
                    </Button>
                  )
                )}
              </li>
            ))}
          </ul>
        )}
      </CardContent>

      {canManage && (
        <>
          <CreateOverrideDialog
            open={createOpen}
            onOpenChange={setCreateOpen}
            domainId={domainId}
          />
          <ConfirmDialog
            open={revokeTarget !== null}
            onOpenChange={(open) => !open && setRevokeTarget(null)}
            title={t("revokeTitle")}
            description={t("revokeDescription")}
            reasonLabel={tCommon("reason")}
            reasonPlaceholder={tCommon("reasonPlaceholder")}
            confirmLabel={tCommon("confirm")}
            cancelLabel={tCommon("cancel")}
            variant="destructive"
            isSubmitting={revoke.isPending}
            errorMessage={revoke.error instanceof ApiError ? revoke.error.message : undefined}
            onConfirm={(reason) => {
              if (!revokeTarget) return;
              revoke.mutate(
                { domainId, overrideId: revokeTarget.id, reason },
                { onSuccess: () => setRevokeTarget(null) },
              );
            }}
          />
        </>
      )}
    </Card>
  );
}
