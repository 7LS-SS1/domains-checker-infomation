"use client";

import { useCallback, useMemo, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { useTranslations } from "next-intl";
import { Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import { DomainsFilterBar } from "@/components/domains/domains-filter-bar";
import { DomainsTable } from "@/components/domains/domains-table";
import { AddDomainDialog } from "@/components/domains/add-domain-dialog";
import { useDomains, type DomainListFilters } from "@/lib/domains/queries";
import { hasCapability, toKnownRoles } from "@/lib/auth/capability";

const DEFAULT_PAGE_SIZE = 50;

function readFilters(searchParams: URLSearchParams): DomainListFilters {
  return {
    query: searchParams.get("query") ?? "",
    lifecycleStatus: searchParams.get("lifecycle_status") ?? "",
    sourceStatus: searchParams.get("source_status") ?? "",
    page: Number(searchParams.get("page") ?? "1") || 1,
    pageSize:
      Number(searchParams.get("page_size") ?? String(DEFAULT_PAGE_SIZE)) || DEFAULT_PAGE_SIZE,
    sort: searchParams.get("sort") ?? "",
    direction: searchParams.get("direction") ?? "",
  };
}

/**
 * All filter/sort/page state is the URL (shareable/bookmarkable, per Master
 * Prompt Phase Frontend 4). Every change replaces the URL via router.push,
 * which re-triggers the server-side-filtered useDomains query — nothing is
 * filtered/sorted/paginated client-side.
 */
export function DomainsPageContent({ roles }: { roles: readonly string[] }) {
  const t = useTranslations("domains");
  const tCommon = useTranslations("common");
  const router = useRouter();
  const searchParams = useSearchParams();
  const [addDialogOpen, setAddDialogOpen] = useState(searchParams.get("action") === "create");

  const filters = useMemo(() => readFilters(searchParams), [searchParams]);
  const domainsQuery = useDomains(filters);

  const updateParams = useCallback(
    (updates: Record<string, string | number | undefined>, resetPage = true) => {
      const params = new URLSearchParams(searchParams.toString());
      for (const [key, value] of Object.entries(updates)) {
        if (value === undefined || value === "") {
          params.delete(key);
        } else {
          params.set(key, String(value));
        }
      }
      if (resetPage) {
        params.delete("page");
      }
      params.delete("action");
      router.push(`/domains?${params.toString()}`);
    },
    [router, searchParams],
  );

  const canCreate = hasCapability(toKnownRoles(roles), "editDomains");
  const hasActiveFilters = Boolean(
    filters.query || filters.lifecycleStatus || filters.sourceStatus,
  );
  const totalPages = domainsQuery.data?.total_pages ?? 1;

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-xl font-semibold text-foreground">{t("title")}</h1>
        {canCreate && (
          <Button onClick={() => setAddDialogOpen(true)}>
            <Plus size={16} aria-hidden />
            {t("addDomain")}
          </Button>
        )}
      </div>

      <DomainsFilterBar
        query={filters.query}
        lifecycleStatus={filters.lifecycleStatus}
        sourceStatus={filters.sourceStatus}
        onQueryChange={(value) => updateParams({ query: value })}
        onLifecycleChange={(value) => updateParams({ lifecycle_status: value })}
        onSourceChange={(value) => updateParams({ source_status: value })}
      />

      <DomainsTable
        query={domainsQuery}
        sort={filters.sort}
        direction={filters.direction}
        onSortChange={(sort, direction) => updateParams({ sort, direction }, false)}
        hasActiveFilters={hasActiveFilters}
        roles={roles}
      />

      {domainsQuery.data && domainsQuery.data.total_items > 0 && (
        <div className="flex items-center justify-between gap-3 text-sm text-muted-foreground">
          <p>{tCommon("pageOf", { page: filters.page, totalPages })}</p>
          <div className="flex gap-2">
            <Button
              variant="secondary"
              size="sm"
              disabled={filters.page <= 1}
              onClick={() => updateParams({ page: filters.page - 1 }, false)}
            >
              {tCommon("previous")}
            </Button>
            <Button
              variant="secondary"
              size="sm"
              disabled={filters.page >= totalPages}
              onClick={() => updateParams({ page: filters.page + 1 }, false)}
            >
              {tCommon("next")}
            </Button>
          </div>
        </div>
      )}

      {canCreate && <AddDomainDialog open={addDialogOpen} onOpenChange={setAddDialogOpen} />}
    </div>
  );
}
