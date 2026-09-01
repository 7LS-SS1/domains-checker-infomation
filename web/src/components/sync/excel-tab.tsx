"use client";

import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useTranslations } from "next-intl";
import { Loader2, Upload } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { ApiError } from "@/lib/api/envelope";
import { useUploadExcelPreview } from "@/lib/sheets/excel-mutation";
import { useApplySheetImport, useRejectSheetImport } from "@/lib/sheets/mutations";
import { useSheetImport } from "@/lib/sheets/queries";
import { hasCapability, toKnownRoles } from "@/lib/auth/capability";
import { ImportReview } from "@/components/sync/import-review";

const MAX_EXCEL_BYTES_CLIENT_HINT = 10 * 1024 * 1024; // matches the backend default (EXCEL_IMPORT_MAX_BYTES) — the server remains authoritative

export function ExcelTab({ roles }: { roles: readonly string[] }) {
  const t = useTranslations("sync.excel");
  const tCommon = useTranslations("common");
  const [previewImportId, setPreviewImportId] = useState<string | undefined>();

  const uploadMutation = useUploadExcelPreview();
  const applyMutation = useApplySheetImport();
  const rejectMutation = useRejectSheetImport();
  const importQuery = useSheetImport(previewImportId);

  const canPreview = hasCapability(toKnownRoles(roles), "previewImport");

  const schema = z.object({
    // z.instanceof(FileList) evaluates the FileList global the moment this
    // schema is constructed — which happens during Next.js's SSR pass even
    // for a "use client" component, and FileList does not exist in Node.
    // z.custom defers the check into the validator function, which only
    // ever runs client-side (react-hook-form never validates during SSR).
    file: z
      .custom<FileList>((value) => typeof FileList !== "undefined" && value instanceof FileList, {
        message: "A file is required",
      })
      .refine((files) => files.length === 1, "A file is required")
      .refine((files) => files[0]!.name.toLowerCase().endsWith(".xlsx"), "Must be a .xlsx file")
      .refine((files) => files[0]!.size <= MAX_EXCEL_BYTES_CLIENT_HINT, t("tooLarge")),
    source_name: z.string().min(1),
    sheet_name: z.string().min(1),
  });
  type FormValues = z.infer<typeof schema>;

  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<FormValues>({ resolver: zodResolver(schema) });

  const onSubmit = handleSubmit((values) => {
    uploadMutation.mutate(
      {
        file: values.file[0]!,
        sourceName: values.source_name,
        sheetName: values.sheet_name,
        columnMapping: {},
      },
      { onSuccess: (result) => setPreviewImportId(result.id) },
    );
  });

  if (!canPreview) {
    return null;
  }

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle>{t("title")}</CardTitle>
        </CardHeader>
        <CardContent>
          <form onSubmit={onSubmit} noValidate className="space-y-4">
            {uploadMutation.isError && (
              <p role="alert" className="text-sm text-status-rose-fg">
                {uploadMutation.error instanceof ApiError
                  ? uploadMutation.error.message
                  : tCommon("loadError")}
              </p>
            )}

            <div className="space-y-1.5">
              <Label htmlFor="excel_file">{t("file")}</Label>
              <input
                id="excel_file"
                type="file"
                accept=".xlsx"
                className="block w-full text-sm text-foreground file:mr-3 file:rounded-md file:border-0 file:bg-primary file:px-3 file:py-1.5 file:text-primary-foreground"
                {...register("file")}
              />
              {errors.file && (
                <p role="alert" className="text-sm text-status-rose-fg">
                  {String(errors.file.message)}
                </p>
              )}
            </div>

            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label htmlFor="source_name">{t("sourceName")}</Label>
                <Input id="source_name" {...register("source_name")} />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="excel_sheet_name">{t("sheetName")}</Label>
                <Input id="excel_sheet_name" {...register("sheet_name")} />
              </div>
            </div>

            <Button type="submit" disabled={uploadMutation.isPending}>
              {uploadMutation.isPending ? (
                <Loader2 className="animate-spin" size={16} aria-hidden />
              ) : (
                <Upload size={16} aria-hidden />
              )}
              {t("upload")}
            </Button>
          </form>
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
