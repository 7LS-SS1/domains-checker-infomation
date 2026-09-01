import {
  AlertTriangle,
  ArrowUpCircle,
  CheckCircle2,
  HelpCircle,
  XCircle,
  type LucideIcon,
} from "lucide-react";
import type { StatusTone } from "@/lib/status/tokens";

/** probe_status enum — migrations/00002_monitoring_and_intelligence.sql line 4. */
export function probeStatusTone(status: string): StatusTone {
  switch (status) {
    case "ONLINE":
      return "emerald";
    case "DEGRADED":
      return "amber";
    case "UPGRADE_REQUIRED":
      return "violet";
    case "OFFLINE":
      return "rose";
    case "REVOKED":
      return "slate";
    default:
      return "slate";
  }
}

export function probeStatusIcon(status: string): LucideIcon {
  switch (status) {
    case "ONLINE":
      return CheckCircle2;
    case "DEGRADED":
      return AlertTriangle;
    case "UPGRADE_REQUIRED":
      return ArrowUpCircle;
    case "OFFLINE":
      return XCircle;
    case "REVOKED":
      return XCircle;
    default:
      return HelpCircle;
  }
}
