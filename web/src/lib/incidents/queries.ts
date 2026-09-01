"use client";

import { keepPreviousData, useQuery, type UseQueryResult } from "@tanstack/react-query";
import { useLocale } from "next-intl";
import { bffFetch } from "@/lib/api/client";
import { incidentPageSchema, type IncidentPage } from "@/lib/incidents/types";

export interface IncidentListFilters {
  status: string;
  page: number;
  pageSize: number;
}

export function useIncidents(filters: IncidentListFilters): UseQueryResult<IncidentPage> {
  const locale = useLocale();
  return useQuery({
    queryKey: ["incidents", filters],
    queryFn: () => {
      const params = new URLSearchParams();
      if (filters.status) params.set("status", filters.status);
      params.set("page", String(filters.page));
      params.set("page_size", String(filters.pageSize));
      return bffFetch(`/api/bff/incidents?${params.toString()}`, incidentPageSchema, { locale });
    },
    placeholderData: keepPreviousData,
  });
}
