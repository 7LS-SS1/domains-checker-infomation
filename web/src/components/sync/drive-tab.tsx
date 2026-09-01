"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";
import { ExternalLink, Loader2, Plug, Unplug } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/empty-state";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { ApiError } from "@/lib/api/envelope";
import { useDisconnectDrive, useDriveConnection, useDriveFiles } from "@/lib/drive/queries";
import { useDriveOAuthPopup } from "@/components/sync/use-drive-oauth-popup";
import { hasCapability, toKnownRoles } from "@/lib/auth/capability";

export function DriveTab({ roles }: { roles: readonly string[] }) {
  const t = useTranslations("sync.drive");
  const tCommon = useTranslations("common");
  const [disconnectOpen, setDisconnectOpen] = useState(false);

  const connectionQuery = useDriveConnection();
  const disconnectMutation = useDisconnectDrive();
  const canManage = hasCapability(toKnownRoles(roles), "manageDriveConnection");

  const oauth = useDriveOAuthPopup(() => {
    toast.success(t("connectedBanner"));
  });

  const isConnected = connectionQuery.isSuccess && connectionQuery.data.status === "active";
  const isNotConnected =
    connectionQuery.isError &&
    connectionQuery.error instanceof ApiError &&
    connectionQuery.error.code === "DRIVE_NOT_CONNECTED";
  const isNotConfigured =
    connectionQuery.isError &&
    connectionQuery.error instanceof ApiError &&
    connectionQuery.error.code === "DRIVE_NOT_CONFIGURED";

  const filesQuery = useDriveFiles(isConnected);

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle>{t("filesTitle")}</CardTitle>
          {canManage && (
            <div className="flex gap-2">
              {isConnected ? (
                <Button variant="secondary" size="sm" onClick={() => setDisconnectOpen(true)}>
                  <Unplug size={14} aria-hidden />
                  {t("disconnect")}
                </Button>
              ) : (
                !isNotConfigured && (
                  <Button size="sm" onClick={oauth.connect} disabled={oauth.state === "connecting"}>
                    {oauth.state === "connecting" ? (
                      <Loader2 size={14} className="animate-spin" aria-hidden />
                    ) : (
                      <Plug size={14} aria-hidden />
                    )}
                    {t("connect")}
                  </Button>
                )
              )}
            </div>
          )}
        </CardHeader>
        <CardContent className="space-y-3">
          {oauth.state === "popup-blocked" && (
            <p role="alert" className="text-sm text-status-amber-fg">
              {t("popupBlocked")}
            </p>
          )}
          {oauth.state === "error" && oauth.errorMessage && (
            <p role="alert" className="text-sm text-status-rose-fg">
              {oauth.errorMessage}
            </p>
          )}

          {isNotConfigured ? (
            <EmptyState title={t("notConfigured")} />
          ) : connectionQuery.isPending ? (
            <Skeleton className="h-10 w-full" />
          ) : isNotConnected ? (
            <p className="text-sm text-muted-foreground">{t("notConnected")}</p>
          ) : isConnected ? (
            <>
              <p className="text-sm text-status-emerald-fg">
                {connectionQuery.data.google_email
                  ? t("connectedAs", { email: connectionQuery.data.google_email })
                  : t("connected")}
              </p>
              {filesQuery.isPending ? (
                <Skeleton className="h-24 w-full" />
              ) : filesQuery.isError ? (
                <p className="text-sm text-status-rose-fg">{tCommon("loadError")}</p>
              ) : filesQuery.data.items.length === 0 ? (
                <EmptyState title={t("noFiles")} />
              ) : (
                <ul className="divide-y divide-border text-sm">
                  {filesQuery.data.items.map((file) => (
                    <li key={file.id} className="flex items-center justify-between gap-2 py-2">
                      <span className="text-foreground">{file.name}</span>
                      {file.web_view_link && (
                        <a
                          href={file.web_view_link}
                          target="_blank"
                          rel="noreferrer"
                          className="inline-flex items-center gap-1 text-xs text-primary hover:underline"
                        >
                          <ExternalLink size={12} aria-hidden />
                        </a>
                      )}
                    </li>
                  ))}
                </ul>
              )}
            </>
          ) : null}
        </CardContent>
      </Card>

      {canManage && (
        <ConfirmDialog
          open={disconnectOpen}
          onOpenChange={setDisconnectOpen}
          title={t("disconnectTitle")}
          description={t("disconnectDescription")}
          reasonLabel={tCommon("reason")}
          reasonPlaceholder={tCommon("reasonPlaceholder")}
          confirmLabel={t("disconnect")}
          cancelLabel={tCommon("cancel")}
          variant="destructive"
          isSubmitting={disconnectMutation.isPending}
          errorMessage={
            disconnectMutation.error instanceof ApiError
              ? disconnectMutation.error.message
              : undefined
          }
          onConfirm={(reason) => {
            disconnectMutation.mutate(reason, { onSuccess: () => setDisconnectOpen(false) });
          }}
        />
      )}
    </div>
  );
}
