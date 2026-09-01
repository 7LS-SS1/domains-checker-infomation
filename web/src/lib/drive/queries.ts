"use client";

import { useMutation, useQuery, useQueryClient, type UseQueryResult } from "@tanstack/react-query";
import { useLocale } from "next-intl";
import { z } from "zod";
import { bffFetch } from "@/lib/api/client";
import {
  driveAuthorizationSchema,
  driveConnectionSchema,
  driveFilePageSchema,
  type DriveConnection,
  type DriveFile,
} from "@/lib/drive/types";

export function useDriveConnection(): UseQueryResult<DriveConnection> {
  const locale = useLocale();
  return useQuery({
    queryKey: ["drive-connection"],
    queryFn: () => bffFetch("/api/bff/google-drive/connection", driveConnectionSchema, { locale }),
    retry: false,
  });
}

export function useDriveFiles(
  enabled: boolean,
  pageToken?: string,
): UseQueryResult<{ items: DriveFile[]; next_page_token?: string }> {
  const locale = useLocale();
  return useQuery({
    queryKey: ["drive-files", pageToken],
    queryFn: () =>
      bffFetch(
        `/api/bff/google-drive/files${pageToken ? `?page_token=${encodeURIComponent(pageToken)}` : ""}`,
        driveFilePageSchema,
        { locale },
      ),
    enabled,
  });
}

export function useConnectDrive() {
  const locale = useLocale();
  return useMutation({
    mutationFn: () =>
      bffFetch("/api/bff/google-drive/connect", driveAuthorizationSchema, {
        method: "POST",
        locale,
      }),
  });
}

export function useDisconnectDrive() {
  const locale = useLocale();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (reason: string) =>
      bffFetch("/api/bff/google-drive/connection", z.undefined(), {
        method: "DELETE",
        body: JSON.stringify({ reason }),
        locale,
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["drive-connection"] });
    },
  });
}
