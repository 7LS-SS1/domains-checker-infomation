"use client";

import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useTranslations } from "next-intl";
import { Loader2, RefreshCw, Save } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/empty-state";
import { ApiError } from "@/lib/api/envelope";
import { useSheetConfig, useSheetImport } from "@/lib/sheets/queries";
import { useSaveSheetConfig, usePreviewSheetImport } from "@/lib/sheets/mutations";
import { useApplySheetImport, useRejectSheetImport } from "@/lib/sheets/mutations";
import { useDriveConnection } from "@/lib/drive/queries";
import { hasCapability, toKnownRoles } from "@/lib/auth/capability";
import { ImportReview } from "@/components/sync/import-review";

export function SheetConfigTab({ roles }: { roles: readonly string[] }) {
  const t = useTranslations("sync.sheetConfig");
  const tCommon = useTranslations("common");
  const [previewImportId, setPreviewImportId] = useState<string | undefined>();

  const configQuery = useSheetConfig();
  const driveConnectionQuery = useDriveConnection();
  const saveConfig = useSaveSheetConfig();
  const previewMutation = usePreviewSheetImport();
  const applyMutation = useApplySheetImport();
  const rejectMutation = useRejectSheetImport();
  const importQuery = useSheetImport(previewImportId);

  const knownRoles = toKnownRoles(roles);
  const canManageConfig = hasCapability(knownRoles, "manageSheetConfig");
  const canPreview = hasCapability(knownRoles, "previewImport");

  const configNotFound =
    configQuery.isError &&
    configQuery.error instanceof ApiError &&
    configQuery.error.code === "SHEETS_NOT_FOUND";

  const schema = z.object({
    spreadsheet_id: z.string().min(1),
    sheet_name: z.string().min(1),
    range: z.string().min(1),
    enabled: z.boolean(),
    sync_interval_minutes: z.number().int().min(5).max(10080),
    reason: z.string().min(1),
  });
  type FormValues = z.infer<typeof schema>;

  const { register, handleSubmit } = useForm<FormValues>({
    resolver: zodResolver(schema),
    values: configQuery.data
      ? {
          spreadsheet_id: configQuery.data.spreadsheet_id,
          sheet_name: configQuery.data.sheet_name,
          range: configQuery.data.range || "A:Z",
          enabled: configQuery.data.enabled,
          sync_interval_minutes: configQuery.data.sync_interval_minutes,
          reason: "",
        }
      : undefined,
  });

  // The saved config must carry the active Drive connection's ID so the
  // backend's ConnectedSource fetches the sheet using that OAuth token
  // (internal/sheets/connected.go) instead of falling back to the static
  // GOOGLE_SHEETS_API_KEY/ACCESS_TOKEN/CREDENTIALS_FILE env vars — which are
  // deliberately left unset when Drive OAuth is the intended auth path, and
  // previously never got linked here, so every preview failed with
  // SHEETS_CREDENTIALS_UNAVAILABLE even after a successful Drive connect.
  const activeConnectionId =
    driveConnectionQuery.isSuccess && driveConnectionQuery.data.status === "active"
      ? driveConnectionQuery.data.id
      : (configQuery.data?.connection_id ?? null);

  const onSubmit = handleSubmit((values) => {
    saveConfig.mutate({
      connection_id: activeConnectionId,
      spreadsheet_id: values.spreadsheet_id,
      sheet_name: values.sheet_name,
      range: values.range,
      column_mapping: configQuery.data?.column_mapping ?? {},
      enabled: values.enabled,
      sync_interval_minutes: values.sync_interval_minutes,
      version: configQuery.data?.version ?? 0,
      reason: values.reason,
    });
  });

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle>{t("title")}</CardTitle>
          {canPreview && (
            <Button
              variant="secondary"
              size="sm"
              disabled={previewMutation.isPending || configNotFound}
              onClick={() =>
                previewMutation.mutate(undefined, {
                  onSuccess: (result) => setPreviewImportId(result.id),
                })
              }
            >
              {previewMutation.isPending ? (
                <Loader2 size={14} className="animate-spin" aria-hidden />
              ) : (
                <RefreshCw size={14} aria-hidden />
              )}
              {t("preview")}
            </Button>
          )}
        </CardHeader>
        <CardContent>
          <p className="mb-3 text-xs text-muted-foreground">
            {activeConnectionId ? t("driveLinked") : t("driveNotLinked")}
          </p>
          {configQuery.isPending ? (
            <Skeleton className="h-40 w-full" />
          ) : configNotFound && !canManageConfig ? (
            <EmptyState title={t("notConfigured")} />
          ) : (
            <form onSubmit={onSubmit} className="space-y-4">
              {saveConfig.isError && (
                <p role="alert" className="text-sm text-status-rose-fg">
                  {saveConfig.error instanceof ApiError &&
                  saveConfig.error.code === "SHEETS_CONFLICT"
                    ? t("conflict")
                    : saveConfig.error instanceof ApiError
                      ? saveConfig.error.message
                      : tCommon("loadError")}
                </p>
              )}
              {previewMutation.isError && (
                <p role="alert" className="text-sm text-status-rose-fg">
                  {previewMutation.error instanceof ApiError
                    ? previewMutation.error.message
                    : tCommon("loadError")}
                </p>
              )}

              <div className="grid grid-cols-2 gap-3">
                <div className="space-y-1.5">
                  <Label htmlFor="spreadsheet_id">{t("spreadsheetId")}</Label>
                  <Input
                    id="spreadsheet_id"
                    disabled={!canManageConfig}
                    {...register("spreadsheet_id")}
                  />
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="sheet_name">{t("sheetName")}</Label>
                  <Input id="sheet_name" disabled={!canManageConfig} {...register("sheet_name")} />
                </div>
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div className="space-y-1.5">
                  <Label htmlFor="range">{t("range")}</Label>
                  <Input id="range" disabled={!canManageConfig} {...register("range")} />
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="sync_interval_minutes">{t("syncInterval")}</Label>
                  <Input
                    id="sync_interval_minutes"
                    type="number"
                    min={5}
                    max={10080}
                    disabled={!canManageConfig}
                    {...register("sync_interval_minutes", { valueAsNumber: true })}
                  />
                </div>
              </div>

              <div className="flex items-center gap-2">
                <input
                  id="sheet_enabled"
                  type="checkbox"
                  disabled={!canManageConfig}
                  className="h-4 w-4 rounded border-border"
                  {...register("enabled")}
                />
                <Label htmlFor="sheet_enabled" className="font-normal">
                  {t("enabled")}
                </Label>
              </div>

              {canManageConfig && (
                <>
                  <div className="space-y-1.5">
                    <Label htmlFor="sheet_config_reason">{tCommon("reason")}</Label>
                    <Input
                      id="sheet_config_reason"
                      placeholder={tCommon("reasonPlaceholder")}
                      {...register("reason")}
                    />
                  </div>
                  <Button type="submit" disabled={saveConfig.isPending}>
                    {saveConfig.isPending ? (
                      <Loader2 className="animate-spin" size={16} aria-hidden />
                    ) : (
                      <Save size={16} aria-hidden />
                    )}
                    {t("save")}
                  </Button>
                </>
              )}
            </form>
          )}
        </CardContent>
      </Card>

      {previewImportId && importQuery.data && (
        <ImportReview
          importRecord={importQuery.data}
          roles={roles}
          onApply={(reason) => applyMutation.mutate({ importId: previewImportId, reason })}
          onReject={(reason) => rejectMutation.mutate({ importId: previewImportId, reason })}
          isApplying={applyMutation.isPending}
          isRejecting={rejectMutation.isPending}
          applyError={applyMutation.error}
          rejectError={rejectMutation.error}
        />
      )}
    </div>
  );
}
