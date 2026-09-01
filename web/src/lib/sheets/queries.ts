"use client";

import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import { useLocale } from "next-intl";
import { bffFetch } from "@/lib/api/client";
import { sheetConfigSchema, sheetImportListSchema, sheetImportSchema } from "@/lib/sheets/types";
import type { SheetConfig, SheetImport } from "@/lib/sheets/types";

export function useSheetConfig(): UseQueryResult<SheetConfig> {
  const locale = useLocale();
  return useQuery({
    queryKey: ["sheet-config"],
    queryFn: () => bffFetch("/api/bff/google-sheets/config", sheetConfigSchema, { locale }),
    retry: false,
  });
}

export function useSheetImports(limit: number): UseQueryResult<SheetImport[]> {
  const locale = useLocale();
  return useQuery({
    queryKey: ["sheet-imports", limit],
    queryFn: async () => {
      const result = await bffFetch(
        `/api/bff/google-sheets/imports?limit=${limit}`,
        sheetImportListSchema,
        { locale },
      );
      return result.items;
    },
  });
}

export function useSheetImport(importId: string | undefined): UseQueryResult<SheetImport> {
  const locale = useLocale();
  return useQuery({
    queryKey: ["sheet-import", importId],
    queryFn: () =>
      bffFetch(`/api/bff/google-sheets/imports/${importId}`, sheetImportSchema, { locale }),
    enabled: Boolean(importId),
  });
}
