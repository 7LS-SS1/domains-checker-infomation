"use client";

import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import { useLocale } from "next-intl";
import { bffFetch } from "@/lib/api/client";
import {
  healthSchema,
  readySchema,
  localesMetaSchema,
  type Health,
  type Ready,
  type LocalesMeta,
} from "@/lib/system/types";

const REFRESH_INTERVAL_MS = 60_000;

export function useHealth(): UseQueryResult<Health> {
  const locale = useLocale();
  return useQuery({
    queryKey: ["dashboard", "system-health"],
    queryFn: () => bffFetch("/api/bff/system/health", healthSchema, { locale }),
    refetchInterval: REFRESH_INTERVAL_MS,
    refetchIntervalInBackground: false,
    retry: false,
  });
}

export function useReady(): UseQueryResult<Ready> {
  const locale = useLocale();
  return useQuery({
    queryKey: ["dashboard", "system-ready"],
    queryFn: () => bffFetch("/api/bff/system/ready", readySchema, { locale }),
    refetchInterval: REFRESH_INTERVAL_MS,
    refetchIntervalInBackground: false,
    retry: false,
  });
}

export function useLocalesMeta(): UseQueryResult<LocalesMeta> {
  const locale = useLocale();
  return useQuery({
    queryKey: ["system-locales-meta"],
    queryFn: () => bffFetch("/api/bff/meta/locales", localesMetaSchema, { locale }),
    retry: false,
    staleTime: Infinity,
  });
}
