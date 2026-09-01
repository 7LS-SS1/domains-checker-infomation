"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useLocale } from "next-intl";
import { bffFetch } from "@/lib/api/client";
import { sheetConfigSchema, sheetImportSchema } from "@/lib/sheets/types";

export interface SaveSheetConfigInput {
  connection_id?: string | null;
  spreadsheet_id: string;
  sheet_name: string;
  range: string;
  column_mapping: Record<string, string>;
  enabled: boolean;
  sync_interval_minutes: number;
  version: number;
  reason: string;
}

export function useSaveSheetConfig() {
  const locale = useLocale();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: SaveSheetConfigInput) =>
      bffFetch("/api/bff/google-sheets/config", sheetConfigSchema, {
        method: "PUT",
        body: JSON.stringify(input),
        locale,
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["sheet-config"] });
    },
  });
}

/** A fresh Idempotency-Key is generated per preview attempt — never reused. */
export function usePreviewSheetImport() {
  const locale = useLocale();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () =>
      bffFetch("/api/bff/google-sheets/previews", sheetImportSchema, {
        method: "POST",
        locale,
        headers: { "Idempotency-Key": crypto.randomUUID() },
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["sheet-imports"] });
    },
  });
}

export function useApplySheetImport() {
  const locale = useLocale();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ importId, reason }: { importId: string; reason: string }) =>
      bffFetch(`/api/bff/google-sheets/imports/${importId}/apply`, sheetImportSchema, {
        method: "POST",
        body: JSON.stringify({ reason }),
        locale,
        headers: { "Idempotency-Key": crypto.randomUUID() },
      }),
    onSuccess: (_data, variables) => {
      void queryClient.invalidateQueries({ queryKey: ["sheet-imports"] });
      void queryClient.invalidateQueries({ queryKey: ["sheet-import", variables.importId] });
      void queryClient.invalidateQueries({ queryKey: ["domains"] });
    },
  });
}

export function useRejectSheetImport() {
  const locale = useLocale();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ importId, reason }: { importId: string; reason: string }) =>
      bffFetch(`/api/bff/google-sheets/imports/${importId}/reject`, sheetImportSchema, {
        method: "POST",
        body: JSON.stringify({ reason }),
        locale,
      }),
    onSuccess: (_data, variables) => {
      void queryClient.invalidateQueries({ queryKey: ["sheet-imports"] });
      void queryClient.invalidateQueries({ queryKey: ["sheet-import", variables.importId] });
    },
  });
}
