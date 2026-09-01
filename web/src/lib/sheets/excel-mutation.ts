"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useLocale } from "next-intl";
import { parseApiEnvelope } from "@/lib/api/envelope";
import { sheetImportSchema } from "@/lib/sheets/types";

export interface UploadExcelInput {
  file: File;
  sourceName: string;
  sheetName: string;
  columnMapping: Record<string, string>;
}

/** A fresh Idempotency-Key is generated per upload attempt — never reused. */
export function useUploadExcelPreview() {
  const locale = useLocale();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (input: UploadExcelInput) => {
      const formData = new FormData();
      formData.set("file", input.file);
      formData.set("source_name", input.sourceName);
      formData.set("sheet_name", input.sheetName);
      formData.set("column_mapping", JSON.stringify(input.columnMapping));

      const response = await fetch("/api/bff/google-sheets/excel/previews", {
        method: "POST",
        body: formData,
        headers: {
          "Accept-Language": locale,
          "Idempotency-Key": crypto.randomUUID(),
        },
        credentials: "same-origin",
      });
      return parseApiEnvelope(response, sheetImportSchema);
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["sheet-imports"] });
    },
  });
}
