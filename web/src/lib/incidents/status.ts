import { AlertTriangle, CheckCircle2, HelpCircle, XCircle, type LucideIcon } from "lucide-react";
import type { StatusTone } from "@/lib/status/tokens";
import { INCIDENT_STATUSES } from "@/lib/incidents/types";

/** incident_status enum — migrations/00002_monitoring_and_intelligence.sql line 6. */
export function incidentStatusTone(status: string): StatusTone {
  switch (status) {
    case "open":
      return "rose";
    case "acknowledged":
      return "amber";
    case "closed":
      return "emerald";
    default:
      return "slate";
  }
}

export function incidentStatusIcon(status: string): LucideIcon {
  switch (status) {
    case "open":
      return XCircle;
    case "acknowledged":
      return AlertTriangle;
    case "closed":
      return CheckCircle2;
    default:
      return HelpCircle;
  }
}

export { INCIDENT_STATUSES };
