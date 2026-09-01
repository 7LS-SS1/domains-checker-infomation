"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { Archive, ArchiveRestore } from "lucide-react";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { ApiError } from "@/lib/api/envelope";
import { useArchiveDomain, useRestoreDomain } from "@/lib/domains/mutations";
import { hasCapability, toKnownRoles } from "@/lib/auth/capability";
import type { Domain } from "@/lib/domains/types";

export function DomainRowActions({ domain, roles }: { domain: Domain; roles: readonly string[] }) {
  const t = useTranslations("domains.actions");
  const tCommon = useTranslations("common");
  const [dialogOpen, setDialogOpen] = useState(false);
  const archive = useArchiveDomain();
  const restore = useRestoreDomain();

  const canEdit = hasCapability(toKnownRoles(roles), "editDomains");
  if (!canEdit) {
    return null;
  }

  const isArchived = domain.lifecycle_status === "archived";
  const mutation = isArchived ? restore : archive;
  const errorMessage = mutation.error instanceof ApiError ? mutation.error.message : undefined;

  return (
    <>
      <Button
        type="button"
        variant="ghost"
        size="sm"
        onClick={() => setDialogOpen(true)}
        aria-label={isArchived ? t("restore") : t("archive")}
      >
        {isArchived ? <ArchiveRestore size={16} aria-hidden /> : <Archive size={16} aria-hidden />}
      </Button>
      <ConfirmDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        title={isArchived ? t("restoreTitle") : t("archiveTitle")}
        description={isArchived ? t("restoreDescription") : t("archiveDescription")}
        reasonLabel={tCommon("reason")}
        reasonPlaceholder={tCommon("reasonPlaceholder")}
        confirmLabel={tCommon("confirm")}
        cancelLabel={tCommon("cancel")}
        variant={isArchived ? "default" : "destructive"}
        isSubmitting={mutation.isPending}
        errorMessage={errorMessage}
        onConfirm={(reason) => {
          mutation.mutate(
            { domainId: domain.id, version: domain.version, reason },
            { onSuccess: () => setDialogOpen(false) },
          );
        }}
      />
    </>
  );
}
