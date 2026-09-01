"use client";

import { useMemo } from "react";
import {
  createColumnHelper,
  flexRender,
  getCoreRowModel,
  useReactTable,
} from "@tanstack/react-table";
import { ArrowDown, ArrowUp, ArrowUpDown } from "lucide-react";
import { useTranslations } from "next-intl";
import Link from "next/link";
import { useDomainColumns, SORTABLE_COLUMNS } from "@/lib/domains/columns";
import { DomainStatusBadge } from "@/components/domains/domain-status-badge";
import { DomainRowActions } from "@/components/domains/domain-row-actions";
import { DateValue } from "@/components/ui/date-value";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { describeQueryError } from "@/lib/api/query-error";
import type { Domain, DomainPage } from "@/lib/domains/types";
import type { UseQueryResult } from "@tanstack/react-query";

const actionsColumnHelper = createColumnHelper<Domain>();

interface DomainsTableProps {
  query: UseQueryResult<DomainPage>;
  sort: string;
  direction: string;
  onSortChange: (sort: string, direction: string) => void;
  hasActiveFilters: boolean;
  roles: readonly string[];
}

export function DomainsTable({
  query,
  sort,
  direction,
  onSortChange,
  hasActiveFilters,
  roles,
}: DomainsTableProps) {
  const dataColumns = useDomainColumns();
  const t = useTranslations("domains");
  const tCommon = useTranslations("common");

  const columns = useMemo(
    () => [
      ...dataColumns,
      actionsColumnHelper.display({
        id: "actions",
        header: "",
        cell: (info) => <DomainRowActions domain={info.row.original} roles={roles} />,
      }),
    ],
    [dataColumns, roles],
  );

  const table = useReactTable({
    data: query.data?.items ?? [],
    columns,
    getCoreRowModel: getCoreRowModel(),
    manualPagination: true,
    manualSorting: true,
  });

  if (query.isError) {
    const info = describeQueryError(query.error);
    return (
      <ErrorState
        title={tCommon("loadError")}
        description={info.message}
        requestId={info.requestId}
        onRetry={() => query.refetch()}
      />
    );
  }

  if (query.isPending) {
    return (
      <div className="space-y-2">
        {Array.from({ length: 8 }).map((_, index) => (
          <Skeleton key={index} className="h-12 w-full" />
        ))}
      </div>
    );
  }

  if (query.data.items.length === 0) {
    return <EmptyState title={hasActiveFilters ? t("empty") : t("emptyNoData")} />;
  }

  function handleHeaderClick(columnId: string) {
    if (!SORTABLE_COLUMNS.has(columnId)) return;
    if (sort !== columnId) {
      onSortChange(columnId, "asc");
    } else {
      onSortChange(columnId, direction === "asc" ? "desc" : "asc");
    }
  }

  return (
    <>
      {/* Desktop dense table */}
      <div
        className="hidden overflow-x-auto rounded-lg border border-border md:block"
        tabIndex={0}
        role="region"
        aria-label={t("title")}
      >
        <table className="w-full min-w-[1100px] text-sm">
          <thead>
            {table.getHeaderGroups().map((headerGroup) => (
              <tr key={headerGroup.id} className="border-b border-border bg-surface">
                {headerGroup.headers.map((header) => {
                  const sortable = SORTABLE_COLUMNS.has(header.column.id);
                  const isActive = sort === header.column.id;
                  return (
                    <th
                      key={header.id}
                      scope="col"
                      className="whitespace-nowrap px-3 py-2 text-left text-xs font-medium text-muted-foreground"
                    >
                      {sortable ? (
                        <button
                          type="button"
                          onClick={() => handleHeaderClick(header.column.id)}
                          className="inline-flex items-center gap-1 hover:text-foreground"
                          aria-pressed={isActive}
                        >
                          {flexRender(header.column.columnDef.header, header.getContext())}
                          {isActive ? (
                            direction === "asc" ? (
                              <ArrowUp size={12} aria-hidden />
                            ) : (
                              <ArrowDown size={12} aria-hidden />
                            )
                          ) : (
                            <ArrowUpDown size={12} className="opacity-40" aria-hidden />
                          )}
                        </button>
                      ) : (
                        flexRender(header.column.columnDef.header, header.getContext())
                      )}
                    </th>
                  );
                })}
              </tr>
            ))}
          </thead>
          <tbody>
            {table.getRowModel().rows.map((row) => (
              <tr key={row.id} className="border-b border-border last:border-0 hover:bg-surface/60">
                {row.getVisibleCells().map((cell) => (
                  <td key={cell.id} className="whitespace-nowrap px-3 py-2 align-middle">
                    {flexRender(cell.column.columnDef.cell, cell.getContext())}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* Mobile card adaptation — no horizontal page scroll required */}
      <ul className="space-y-2 md:hidden">
        {query.data.items.map((domain) => (
          <DomainCard key={domain.id} domain={domain} />
        ))}
      </ul>
    </>
  );
}

function DomainCard({ domain }: { domain: Domain }) {
  const t = useTranslations("domains");
  const isIdn = domain.domain_unicode && domain.domain_unicode !== domain.domain_ascii;

  return (
    <li className="rounded-lg border border-border p-3">
      <Link href={`/domains/${domain.id}`} className="font-medium text-foreground hover:underline">
        {isIdn ? `${domain.domain_unicode} (${domain.domain_ascii})` : domain.domain_ascii}
      </Link>
      <div className="mt-2 flex flex-wrap gap-1.5">
        <DomainStatusBadge dimension="availability" value={domain.availability_status} />
        <DomainStatusBadge dimension="lifecycle" value={domain.lifecycle_status} />
        <DomainStatusBadge dimension="isp" value={domain.isp_status} />
      </div>
      <p className="mt-2 text-xs text-muted-foreground">
        {t("columns.lastChecked")}:{" "}
        {domain.last_checked_at ? <DateValue value={domain.last_checked_at} /> : t("neverChecked")}
      </p>
    </li>
  );
}
