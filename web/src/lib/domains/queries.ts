"use client";

import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { useLocale } from "next-intl";
import { bffFetch } from "@/lib/api/client";
import { domainPageSchema, domainSchema } from "@/lib/domains/types";

export interface DomainListFilters {
  query: string;
  lifecycleStatus: string;
  sourceStatus: string;
  page: number;
  pageSize: number;
  sort: string;
  direction: string;
}

export function buildDomainListQuery(filters: DomainListFilters): string {
  const params = new URLSearchParams();
  if (filters.query) params.set("query", filters.query);
  if (filters.lifecycleStatus) params.set("lifecycle_status", filters.lifecycleStatus);
  if (filters.sourceStatus) params.set("source_status", filters.sourceStatus);
  params.set("page", String(filters.page));
  params.set("page_size", String(filters.pageSize));
  if (filters.sort) params.set("sort", filters.sort);
  if (filters.direction) params.set("direction", filters.direction);
  return params.toString();
}

/** keepPreviousData avoids a full-table skeleton flash on every filter/page change. */
export function useDomains(filters: DomainListFilters) {
  const locale = useLocale();
  return useQuery({
    queryKey: ["domains", filters],
    queryFn: () =>
      bffFetch(`/api/bff/domains?${buildDomainListQuery(filters)}`, domainPageSchema, { locale }),
    placeholderData: keepPreviousData,
  });
}

export function useDomain(domainId: string | undefined) {
  const locale = useLocale();
  return useQuery({
    queryKey: ["domain", domainId],
    queryFn: () => bffFetch(`/api/bff/domains/${domainId}`, domainSchema, { locale }),
    enabled: Boolean(domainId),
  });
}
