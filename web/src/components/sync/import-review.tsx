"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { Check, X } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { StatusBadge } from "@/components/ui/status-badge";
import { ApiError } from "@/lib/api/envelope";
import { IMPORT_ROW_ACTIONS, type SheetImport } from "@/lib/sheets/types";
import type { StatusTone } from "@/lib/status/tokens";
import { hasCapability, toKnownRoles } from "@/lib/auth/capability";

function actionTone(action: string): StatusTone {
  switch (action) {
    case "ADD":
      return "emerald";
    case "MODIFY":
      return "amber";
    case "UNCHANGED":
      return "slate";
    case "MISSING":
      return "violet";
    case "INVALID":
      return "rose";
    default:
      return "slate";
  }
}

interface ImportReviewProps {
  importRecord: SheetImport;
  roles: readonly string[];
  onApply: (reason: string) => void;
  onReject: (reason: string) => void;
  isApplying: boolean;
  isRejecting: boolean;
  applyError?: unknown;
  rejectError?: unknown;
}

/**
 * Shared staged-rows review UI for both Google Sheet and Excel imports —
 * same backend contract (sheets.Import/ImportRow), same ADD/MODIFY/
 * UNCHANGED/MISSING/INVALID semantics either way. Invalid rows are always
 * rendered with their validation errors and are never styled as if they
 * will be applied; MISSING rows are labeled as changing source status, not
 * a deletion, per the Master Prompt.
 */
export function ImportReview({
  importRecord,
  roles,
  onApply,
  onReject,
  isApplying,
  isRejecting,
  applyError,
  rejectError,
}: ImportReviewProps) {
  const t = useTranslations("sync.review");
  const tCommon = useTranslations("common");
  const [filter, setFilter] = useState("");
  const [applyDialogOpen, setApplyDialogOpen] = useState(false);
  const [rejectDialogOpen, setRejectDialogOpen] = useState(false);

  const canApply = hasCapability(toKnownRoles(roles), "applyImport");
  const rows = importRecord.rows ?? [];
  const filteredRows = filter ? rows.filter((row) => row.action === filter) : rows;
  const canBeApplied = importRecord.status === "preview";

  const counts: Record<string, number> = {
    ADD: importRecord.added_count,
    MODIFY: importRecord.modified_count,
    UNCHANGED: importRecord.unchanged_count,
    MISSING: importRecord.missing_count,
    INVALID: importRecord.invalid_count,
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("title")}</CardTitle>
        {canApply && canBeApplied && (
          <div className="flex gap-2">
            <Button variant="secondary" size="sm" onClick={() => setRejectDialogOpen(true)}>
              <X size={14} aria-hidden />
              {t("reject")}
            </Button>
            <Button size="sm" onClick={() => setApplyDialogOpen(true)}>
              <Check size={14} aria-hidden />
              {t("apply")}
            </Button>
          </div>
        )}
      </CardHeader>
      <CardContent className="space-y-3">
        {importRecord.status === "applied" && (
          <p className="text-sm text-status-emerald-fg">{t("alreadyApplied")}</p>
        )}
        {importRecord.status === "rejected" && (
          <p className="text-sm text-muted-foreground">{t("alreadyRejected")}</p>
        )}

        <div className="flex flex-wrap gap-2">
          <button
            type="button"
            onClick={() => setFilter("")}
            aria-pressed={filter === ""}
            className={`rounded-full px-2.5 py-1 text-xs font-medium ${filter === "" ? "bg-primary text-primary-foreground" : "border border-border text-muted-foreground"}`}
          >
            {t("all")} ({rows.length})
          </button>
          {IMPORT_ROW_ACTIONS.map((action) => (
            <button
              key={action}
              type="button"
              onClick={() => setFilter(action)}
              aria-pressed={filter === action}
            >
              <StatusBadge
                tone={actionTone(action)}
                icon={Check}
                label={`${t(`actions.${action}`)} (${counts[action] ?? 0})`}
                className={filter === action ? "ring-2 ring-ring" : undefined}
              />
            </button>
          ))}
        </div>

        {importRecord.missing_count > 0 && (
          <p className="rounded-md border border-status-violet-border bg-status-violet-bg p-2 text-xs text-status-violet-fg">
            {t("missingWarning")}
          </p>
        )}

        <div
          className="max-h-[480px] overflow-y-auto overflow-x-auto rounded-md border border-border"
          tabIndex={0}
          role="region"
          aria-label={t("title")}
        >
          <table className="w-full min-w-[720px] text-sm">
            <thead>
              <tr className="border-b border-border bg-surface text-left text-xs font-medium text-muted-foreground">
                <th scope="col" className="px-3 py-2">
                  {t("columns.row")}
                </th>
                <th scope="col" className="px-3 py-2">
                  {t("columns.action")}
                </th>
                <th scope="col" className="px-3 py-2">
                  {t("columns.domain")}
                </th>
                <th scope="col" className="px-3 py-2">
                  {t("columns.errors")}
                </th>
              </tr>
            </thead>
            <tbody>
              {filteredRows.map((row) => (
                <tr key={row.id} className="border-b border-border last:border-0">
                  <td className="px-3 py-2 tabular-nums text-muted-foreground">{row.row_number}</td>
                  <td className="px-3 py-2">
                    <StatusBadge
                      tone={actionTone(row.action)}
                      icon={Check}
                      label={t(`actions.${row.action}`)}
                    />
                  </td>
                  <td className="px-3 py-2 text-foreground">
                    {row.normalized_values?.domain ?? row.raw_values.domain ?? "—"}
                  </td>
                  <td className="px-3 py-2 text-xs text-status-rose-fg">
                    {row.validation_errors.join(", ")}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </CardContent>

      {canApply && (
        <>
          <ConfirmDialog
            open={applyDialogOpen}
            onOpenChange={setApplyDialogOpen}
            title={t("applyTitle")}
            description={t("applyDescription", {
              add: importRecord.added_count,
              modify: importRecord.modified_count,
              missing: importRecord.missing_count,
              invalid: importRecord.invalid_count,
            })}
            reasonLabel={tCommon("reason")}
            reasonPlaceholder={tCommon("reasonPlaceholder")}
            confirmLabel={t("apply")}
            cancelLabel={tCommon("cancel")}
            isSubmitting={isApplying}
            errorMessage={applyError instanceof ApiError ? applyError.message : undefined}
            onConfirm={onApply}
          />
          <ConfirmDialog
            open={rejectDialogOpen}
            onOpenChange={setRejectDialogOpen}
            title={t("rejectTitle")}
            reasonLabel={tCommon("reason")}
            reasonPlaceholder={tCommon("reasonPlaceholder")}
            confirmLabel={t("reject")}
            cancelLabel={tCommon("cancel")}
            variant="destructive"
            isSubmitting={isRejecting}
            errorMessage={rejectError instanceof ApiError ? rejectError.message : undefined}
            onConfirm={onReject}
          />
        </>
      )}
    </Card>
  );
}
