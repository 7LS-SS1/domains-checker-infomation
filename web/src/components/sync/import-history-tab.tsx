"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import {
  AlertTriangle,
  ArrowLeft,
  Check,
  Clock,
  FileSpreadsheet,
  FileUp,
  Loader2,
  X,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/empty-state";
import { StatusBadge } from "@/components/ui/status-badge";
import { DateValue } from "@/components/ui/date-value";
import { ApiError } from "@/lib/api/envelope";
import { useSheetImport, useSheetImports } from "@/lib/sheets/queries";
import { useApplySheetImport, useRejectSheetImport } from "@/lib/sheets/mutations";
import { ImportReview } from "@/components/sync/import-review";
import type { StatusTone } from "@/lib/status/tokens";

const HISTORY_LIMIT = 50;

function statusTone(status: string): StatusTone {
  switch (status) {
    case "applied":
      return "emerald";
    case "applying":
      return "amber";
    case "rejected":
      return "slate";
    case "failed":
      return "rose";
    default:
      return "slate";
  }
}

function statusIcon(status: string): LucideIcon {
  switch (status) {
    case "applied":
      return Check;
    case "applying":
      return Loader2;
    case "rejected":
      return X;
    case "failed":
      return AlertTriangle;
    default:
      return Clock;
  }
}

/**
 * Import history/detail using the real GET /google-sheets/imports (list) and
 * GET /google-sheets/imports/{id} (detail) endpoints — covers both Google
 * Sheet and Excel imports, which share the same backend record shape
 * (distinguished only by source_kind). Per Prompt 6, this must use the real
 * endpoint rather than reconstructing history from session-local state.
 */
export function ImportHistoryTab({ roles }: { roles: readonly string[] }) {
  const t = useTranslations("sync.history");
  const tCommon = useTranslations("common");
  const [selectedId, setSelectedId] = useState<string | undefined>();

  const listQuery = useSheetImports(HISTORY_LIMIT);
  const detailQuery = useSheetImport(selectedId);
  const applyMutation = useApplySheetImport();
  const rejectMutation = useRejectSheetImport();

  if (selectedId) {
    return (
      <div className="space-y-4">
        <Button variant="secondary" size="sm" onClick={() => setSelectedId(undefined)}>
          <ArrowLeft size={14} aria-hidden />
          {t("backToList")}
        </Button>
        {detailQuery.isPending ? (
          <Skeleton className="h-40 w-full" />
        ) : detailQuery.isError ? (
          <p role="alert" className="text-sm text-status-rose-fg">
            {detailQuery.error instanceof ApiError
              ? detailQuery.error.message
              : tCommon("loadError")}
          </p>
        ) : detailQuery.data ? (
          <ImportReview
            importRecord={detailQuery.data}
            roles={roles}
            onApply={(reason) => applyMutation.mutate({ importId: selectedId, reason })}
            onReject={(reason) => rejectMutation.mutate({ importId: selectedId, reason })}
            isApplying={applyMutation.isPending}
            isRejecting={rejectMutation.isPending}
            applyError={applyMutation.error}
            rejectError={rejectMutation.error}
          />
        ) : null}
      </div>
    );
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("title")}</CardTitle>
      </CardHeader>
      <CardContent>
        {listQuery.isPending ? (
          <Skeleton className="h-40 w-full" />
        ) : listQuery.isError ? (
          <p role="alert" className="text-sm text-status-rose-fg">
            {listQuery.error instanceof ApiError ? listQuery.error.message : tCommon("loadError")}
          </p>
        ) : listQuery.data && listQuery.data.length > 0 ? (
          <div
            className="overflow-x-auto rounded-md border border-border"
            tabIndex={0}
            role="region"
            aria-label={t("title")}
          >
            <table className="w-full min-w-[640px] text-sm">
              <thead>
                <tr className="border-b border-border bg-surface text-left text-xs font-medium text-muted-foreground">
                  <th scope="col" className="px-3 py-2">
                    {t("columns.source")}
                  </th>
                  <th scope="col" className="px-3 py-2">
                    {t("columns.target")}
                  </th>
                  <th scope="col" className="px-3 py-2">
                    {t("columns.status")}
                  </th>
                  <th scope="col" className="px-3 py-2">
                    {t("columns.rows")}
                  </th>
                  <th scope="col" className="px-3 py-2">
                    {t("columns.previewedAt")}
                  </th>
                  <th scope="col" className="px-3 py-2">
                    <span className="sr-only">{t("viewDetail")}</span>
                  </th>
                </tr>
              </thead>
              <tbody>
                {listQuery.data.map((item) => (
                  <tr key={item.id} className="border-b border-border last:border-0">
                    <td className="px-3 py-2 text-foreground">
                      <span className="inline-flex items-center gap-1.5">
                        {item.source_kind === "excel" ? (
                          <FileUp size={14} className="text-muted-foreground" aria-hidden />
                        ) : (
                          <FileSpreadsheet
                            size={14}
                            className="text-muted-foreground"
                            aria-hidden
                          />
                        )}
                        {t(`sourceKind.${item.source_kind}`)}
                      </span>
                    </td>
                    <td className="px-3 py-2 text-foreground">
                      {item.spreadsheet_id} / {item.sheet_name}
                    </td>
                    <td className="px-3 py-2">
                      <StatusBadge
                        tone={statusTone(item.status)}
                        icon={statusIcon(item.status)}
                        label={t(`status.${item.status}`)}
                      />
                    </td>
                    <td className="px-3 py-2 tabular-nums text-muted-foreground">
                      {item.total_rows}
                    </td>
                    <td className="px-3 py-2 text-muted-foreground">
                      <DateValue value={item.previewed_at} />
                    </td>
                    <td className="px-3 py-2 text-right">
                      <Button variant="secondary" size="sm" onClick={() => setSelectedId(item.id)}>
                        {t("viewDetail")}
                      </Button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <EmptyState title={t("empty")} />
        )}
      </CardContent>
    </Card>
  );
}
