"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";
import { Loader2, Pencil, Play, Radar } from "lucide-react";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { DomainStatusBadge } from "@/components/domains/domain-status-badge";
import { EditDomainDialog } from "@/components/domains/edit-domain-dialog";
import { ApiError } from "@/lib/api/envelope";
import {
  useArchiveDomain,
  useForceISPCheck,
  useManualCheck,
  useRestoreDomain,
} from "@/lib/domains/mutations";
import { hasCapability, toKnownRoles } from "@/lib/auth/capability";
import type { Domain } from "@/lib/domains/types";

export function DomainDetailHeader({
  domain,
  roles,
}: {
  domain: Domain;
  roles: readonly string[];
}) {
  const t = useTranslations("domainDetail");
  const tActions = useTranslations("domains.actions");
  const tCommon = useTranslations("common");
  const [lifecycleDialogOpen, setLifecycleDialogOpen] = useState(false);
  const [editDialogOpen, setEditDialogOpen] = useState(false);

  const archive = useArchiveDomain();
  const restore = useRestoreDomain();
  const manualCheck = useManualCheck();
  const forceIspCheck = useForceISPCheck();

  const knownRoles = toKnownRoles(roles);
  const canEdit = hasCapability(knownRoles, "editDomains");
  const isArchived = domain.lifecycle_status === "archived";
  const lifecycleMutation = isArchived ? restore : archive;
  const lifecycleError =
    lifecycleMutation.error instanceof ApiError ? lifecycleMutation.error.message : undefined;

  function handleManualCheck() {
    manualCheck.mutate(domain.id, {
      onSuccess: () => toast.success(t("actions.manualCheckAccepted")),
      onError: (error) => {
        toast.error(error instanceof ApiError ? error.message : tCommon("loadError"));
      },
    });
  }

  function handleForceIspCheck() {
    forceIspCheck.mutate(domain.id, {
      onSuccess: () => toast.success(t("actions.forceIspCheckAccepted")),
      onError: (error) => {
        toast.error(error instanceof ApiError ? error.message : tCommon("loadError"));
      },
    });
  }

  const isIdn = domain.domain_unicode && domain.domain_unicode !== domain.domain_ascii;

  return (
    <div className="rounded-lg border border-border bg-background p-4">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold text-foreground">
            {isIdn ? domain.domain_unicode : domain.domain_ascii}
          </h1>
          {isIdn && <p className="text-sm text-muted-foreground">{domain.domain_ascii}</p>}
        </div>

        {canEdit && (
          <div className="flex flex-wrap gap-2">
            <Button variant="secondary" size="sm" onClick={() => setEditDialogOpen(true)}>
              <Pencil size={14} aria-hidden />
              {t("overview.editButton")}
            </Button>
            <Button
              variant="secondary"
              size="sm"
              onClick={handleManualCheck}
              disabled={manualCheck.isPending || domain.lifecycle_status !== "active"}
            >
              {manualCheck.isPending ? (
                <Loader2 size={14} className="animate-spin" aria-hidden />
              ) : (
                <Play size={14} aria-hidden />
              )}
              {manualCheck.isPending ? t("actions.manualCheckRunning") : t("actions.manualCheck")}
            </Button>
            <Button
              variant="secondary"
              size="sm"
              onClick={handleForceIspCheck}
              disabled={forceIspCheck.isPending || domain.lifecycle_status !== "active"}
            >
              {forceIspCheck.isPending ? (
                <Loader2 size={14} className="animate-spin" aria-hidden />
              ) : (
                <Radar size={14} aria-hidden />
              )}
              {forceIspCheck.isPending
                ? t("actions.forceIspCheckRunning")
                : t("actions.forceIspCheck")}
            </Button>
            <Button
              variant={isArchived ? "secondary" : "destructive"}
              size="sm"
              onClick={() => setLifecycleDialogOpen(true)}
            >
              {isArchived ? tActions("restore") : tActions("archive")}
            </Button>
          </div>
        )}
      </div>

      <div className="mt-3 flex flex-wrap gap-1.5">
        <DomainStatusBadge dimension="lifecycle" value={domain.lifecycle_status} />
        <DomainStatusBadge dimension="source" value={domain.source_status} />
        <DomainStatusBadge dimension="availability" value={domain.availability_status} />
        <DomainStatusBadge dimension="dns" value={domain.dns_status} />
        <DomainStatusBadge dimension="http" value={domain.http_status} />
        <DomainStatusBadge dimension="redirect" value={domain.redirect_status} />
        <DomainStatusBadge dimension="isp" value={domain.isp_status} />
        <DomainStatusBadge dimension="tls" value={domain.tls_status} />
        <DomainStatusBadge dimension="priority" value={domain.business_priority} />
      </div>

      {canEdit && (
        <>
          <ConfirmDialog
            open={lifecycleDialogOpen}
            onOpenChange={setLifecycleDialogOpen}
            title={isArchived ? tActions("restoreTitle") : tActions("archiveTitle")}
            description={
              isArchived ? tActions("restoreDescription") : tActions("archiveDescription")
            }
            reasonLabel={tCommon("reason")}
            reasonPlaceholder={tCommon("reasonPlaceholder")}
            confirmLabel={tCommon("confirm")}
            cancelLabel={tCommon("cancel")}
            variant={isArchived ? "default" : "destructive"}
            isSubmitting={lifecycleMutation.isPending}
            errorMessage={lifecycleError}
            onConfirm={(reason) => {
              lifecycleMutation.mutate(
                { domainId: domain.id, version: domain.version, reason },
                { onSuccess: () => setLifecycleDialogOpen(false) },
              );
            }}
          />
          <EditDomainDialog
            open={editDialogOpen}
            onOpenChange={setEditDialogOpen}
            domain={domain}
          />
        </>
      )}
    </div>
  );
}
