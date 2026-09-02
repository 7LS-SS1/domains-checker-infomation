"use client";

import { CheckCircle2, HelpCircle, ShieldAlert } from "lucide-react";
import { useTranslations } from "next-intl";
import { StatusBadge } from "@/components/ui/status-badge";
import { cn } from "@/lib/utils/cn";
import type { Domain } from "@/lib/domains/types";

type DomainListState = "active" | "cancelled" | "notUsed" | "checkProblem";

function resolveDomainListState(domain: Domain): DomainListState {
  if (domain.lifecycle_status === "archived") return "cancelled";
  if (domain.lifecycle_status === "inactive") return "notUsed";
  if (domain.current_failure_stage || domain.current_error_code) return "checkProblem";
  return "active";
}

const DOMAIN_STATE_DOT_CLASS: Record<DomainListState, string> = {
  active: "bg-emerald-500",
  cancelled: "bg-rose-500",
  notUsed: "bg-blue-500",
  checkProblem: "bg-amber-400",
};

export function DomainLifecycleDot({ domain }: { domain: Domain }) {
  const t = useTranslations("domains.domainState");
  const state = resolveDomainListState(domain);
  const label = t(state);

  return (
    <span
      className={cn("inline-block size-2.5 shrink-0 rounded-full", DOMAIN_STATE_DOT_CLASS[state])}
      aria-label={label}
      title={label}
      role="img"
    />
  );
}

export function ConfidenceRing({ value }: { value: number }) {
  const t = useTranslations("domains");
  const normalizedValue = Math.min(100, Math.max(0, value));

  return (
    <div
      role="progressbar"
      aria-label={`${t("columns.confidence")}: ${normalizedValue}%`}
      aria-valuemin={0}
      aria-valuemax={100}
      aria-valuenow={normalizedValue}
      className="relative size-11 shrink-0 text-primary"
    >
      <svg viewBox="0 0 40 40" className="size-full -rotate-90" aria-hidden>
        <circle cx="20" cy="20" r="16" fill="none" stroke="var(--border)" strokeWidth="4" />
        <circle
          cx="20"
          cy="20"
          r="16"
          fill="none"
          stroke="currentColor"
          strokeWidth="4"
          strokeLinecap="round"
          pathLength="100"
          strokeDasharray="100"
          strokeDashoffset={100 - normalizedValue}
        />
      </svg>
      <span className="absolute inset-0 flex items-center justify-center text-[10px] font-semibold tabular-nums text-foreground">
        {normalizedValue}%
      </span>
    </div>
  );
}

export function DomainListIspBadge({ status }: { status: string }) {
  const t = useTranslations("domains.ispSummary");

  if (status === "NOT_DETECTED") {
    return <StatusBadge tone="emerald" icon={CheckCircle2} label={t("normal")} />;
  }

  if (status === "SUSPECTED" || status === "HIGH_CONFIDENCE_BLOCK") {
    return <StatusBadge tone="rose" icon={ShieldAlert} label={t("vpn")} />;
  }

  return <StatusBadge tone="slate" icon={HelpCircle} label={t("unknown")} />;
}
