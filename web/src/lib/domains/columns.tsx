"use client";

import { useMemo } from "react";
import { createColumnHelper } from "@tanstack/react-table";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { DomainStatusBadge } from "@/components/domains/domain-status-badge";
import {
  ConfidenceRing,
  DomainLifecycleDot,
  DomainListIspBadge,
} from "@/components/domains/domain-list-indicators";
import { DateValue } from "@/components/ui/date-value";
import type { Domain } from "@/lib/domains/types";

/**
 * Only fields actually present on domain.Domain (internal/domain/model.go)
 * are columns here. Renewal cost and recommendation are NOT in the list
 * response (they live behind /domains/{id}/costs and
 * /domains/{id}/recommendation) — per the Master Prompt's rule against
 * guessing missing list fields, they are deliberately absent from this
 * table, not faked. See web/API_GAPS.md.
 */
const columnHelper = createColumnHelper<Domain>();

/** Backend sort whitelist — internal/domain/store.go's sortExpression map. Only these are clickable-sortable. */
export const SORTABLE_COLUMNS: ReadonlySet<string> = new Set(["domain", "expiration"]);

function daysUntil(iso: string): number {
  const target = new Date(iso);
  const now = new Date();
  const targetUtcDay = Date.UTC(target.getUTCFullYear(), target.getUTCMonth(), target.getUTCDate());
  const nowUtcDay = Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate());
  return Math.round((targetUtcDay - nowUtcDay) / 86_400_000);
}

export function useDomainColumns() {
  const t = useTranslations("domains");

  return useMemo(
    () => [
      columnHelper.accessor("domain_ascii", {
        id: "domain",
        header: t("columns.domain"),
        cell: (info) => {
          const domain = info.row.original;
          const isIdn = domain.domain_unicode && domain.domain_unicode !== domain.domain_ascii;
          return (
            <div className="flex items-center gap-2">
              <DomainLifecycleDot domain={domain} />
              <Link
                href={`/domains/${domain.id}`}
                className="font-medium text-foreground hover:underline"
              >
                {isIdn ? (
                  <span>
                    {domain.domain_unicode}{" "}
                    <span className="text-xs text-muted-foreground">({domain.domain_ascii})</span>
                  </span>
                ) : (
                  domain.domain_ascii
                )}
              </Link>
            </div>
          );
        },
      }),
      columnHelper.accessor("renewal_price", {
        id: "renewal_price",
        header: t("columns.price"),
        cell: (info) => {
          const amount = info.getValue();
          const currency = info.row.original.renewal_currency;
          return amount && currency ? (
            <span className="tabular-nums">
              {amount} {currency}
            </span>
          ) : (
            <span className="text-muted-foreground">—</span>
          );
        },
      }),
      columnHelper.accessor("renewal_decision", {
        id: "renewal_decision",
        header: t("columns.renewalDecision"),
        cell: (info) => <span>{t(`renewalDecision.${info.getValue()}`)}</span>,
      }),
      columnHelper.accessor("source_status", {
        id: "source_status",
        header: t("columns.source"),
        cell: (info) => <DomainStatusBadge dimension="source" value={info.getValue()} />,
      }),
      columnHelper.accessor("availability_status", {
        id: "main_status",
        header: t("columns.mainStatus"),
        cell: (info) => {
          const code = info.row.original.latest_http_status_code;
          return (
            <div className="space-y-1">
              <DomainStatusBadge dimension="availability" value={info.getValue()} />
              {code ? (
                <span className="block text-xs text-muted-foreground">HTTP {code}</span>
              ) : null}
            </div>
          );
        },
      }),
      columnHelper.accessor("confidence_score", {
        id: "confidence_score",
        header: t("columns.confidence"),
        cell: (info) => <ConfidenceRing value={info.getValue()} />,
      }),
      columnHelper.accessor("http_status", {
        id: "http_status",
        header: t("columns.http"),
        cell: (info) => <DomainStatusBadge dimension="http" value={info.getValue()} />,
      }),
      columnHelper.accessor("redirect_status", {
        id: "redirect_status",
        header: t("columns.redirect"),
        cell: (info) => <DomainStatusBadge dimension="redirect" value={info.getValue()} />,
      }),
      columnHelper.accessor("redirect_target_url", {
        id: "redirect_target_url",
        header: t("columns.redirectTarget"),
        cell: (info) => {
          const value = info.getValue();
          return value ? (
            <span className="block max-w-56 truncate text-xs" title={value}>
              {value}
            </span>
          ) : (
            <span className="text-muted-foreground">—</span>
          );
        },
      }),
      columnHelper.accessor("isp_status", {
        id: "isp_status",
        header: t("columns.isp"),
        cell: (info) => <DomainListIspBadge status={info.getValue()} />,
      }),
      columnHelper.accessor("tls_status", {
        id: "tls_status",
        header: t("columns.tls"),
        cell: (info) => <DomainStatusBadge dimension="tls" value={info.getValue()} />,
      }),
      columnHelper.accessor("business_priority", {
        id: "business_priority",
        header: t("columns.priority"),
        cell: (info) => <DomainStatusBadge dimension="priority" value={info.getValue()} />,
      }),
      columnHelper.accessor("expiration_at", {
        id: "expiration",
        header: t("columns.expiry"),
        cell: (info) => {
          const value = info.getValue();
          if (!value) return <span className="text-muted-foreground">{t("noExpiration")}</span>;
          const days = daysUntil(value);
          return (
            <div>
              <DateValue value={value} dateOnly className="block" />
              <span className="text-xs text-muted-foreground">
                {days < 0 ? t("expired", { days: Math.abs(days) }) : t("expiresIn", { days })}
              </span>
            </div>
          );
        },
      }),
      columnHelper.accessor("last_checked_at", {
        id: "last_checked_at",
        header: t("columns.lastChecked"),
        cell: (info) => {
          const value = info.getValue();
          return value ? (
            <DateValue value={value} />
          ) : (
            <span className="text-muted-foreground">{t("neverChecked")}</span>
          );
        },
      }),
    ],
    [t],
  );
}
