"use client";

import { useTranslations } from "next-intl";
import { AlertTriangle, CheckCircle2, HelpCircle, ShieldAlert, XCircle } from "lucide-react";
import { StatusBadge } from "@/components/ui/status-badge";
import {
  availabilityTone,
  dnsTone,
  httpTone,
  ispTone,
  lifecycleTone,
  redirectTone,
  sourceStatusTone,
  tlsTone,
} from "@/lib/domains/status";
import type { StatusTone } from "@/lib/status/tokens";

type Dimension =
  | "availability"
  | "dns"
  | "http"
  | "redirect"
  | "isp"
  | "tls"
  | "lifecycle"
  | "source"
  | "priority";

const TONE_FN: Record<Exclude<Dimension, "priority">, (value: string) => StatusTone> = {
  availability: availabilityTone,
  dns: dnsTone,
  http: httpTone,
  redirect: redirectTone,
  isp: ispTone,
  tls: tlsTone,
  lifecycle: lifecycleTone,
  source: sourceStatusTone,
};

function priorityTone(value: string): StatusTone {
  switch (value) {
    case "critical":
      return "rose";
    case "high":
      return "amber";
    case "medium":
      return "slate";
    default:
      return "emerald";
  }
}

function iconForTone(tone: StatusTone) {
  switch (tone) {
    case "emerald":
      return CheckCircle2;
    case "amber":
      return AlertTriangle;
    case "rose":
      return XCircle;
    case "violet":
      return ShieldAlert;
    default:
      return HelpCircle;
  }
}

interface DomainStatusBadgeProps {
  dimension: Dimension;
  value: string;
  className?: string;
}

/**
 * Resolves any of the eight+ domain-related status dimensions to a
 * StatusBadge (icon + color + translated text). Falls back to the raw enum
 * value when a translation is missing (a future backend enum addition)
 * rather than rendering blank — status must always be shown honestly, per
 * the Master Prompt.
 */
export function DomainStatusBadge({ dimension, value, className }: DomainStatusBadgeProps) {
  const t = useTranslations(`domainStatus.${dimension}`);
  const tone = dimension === "priority" ? priorityTone(value) : TONE_FN[dimension](value);
  const label = t.has(value) ? t(value) : value;

  return <StatusBadge tone={tone} icon={iconForTone(tone)} label={label} className={className} />;
}
