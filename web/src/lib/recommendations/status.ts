import {
  AlertTriangle,
  CheckCircle2,
  HelpCircle,
  Sparkles,
  XCircle,
  type LucideIcon,
} from "lucide-react";
import type { StatusTone } from "@/lib/status/tokens";
import { RECOMMENDATION_ACTIONS } from "@/lib/recommendations/types";

/**
 * recommendation_action enum (migrations 00002 + 00006 ALTER TYPE).
 * PROFIT_OPPORTUNITY reuses the "violet" tone defined for ISP risk in the
 * Master Prompt's fixed 5-color palette — the two never appear in the same
 * UI context (ISP risk is a domain status badge, this is a recommendation
 * action badge), and the distinct Sparkles icon plus explicit text label
 * keep them unambiguous.
 */
export function recommendationActionTone(action: string): StatusTone {
  switch (action) {
    case "RENEW":
      return "emerald";
    case "REVIEW":
      return "amber";
    case "DROP":
      return "rose";
    case "PROFIT_OPPORTUNITY":
      return "violet";
    default:
      return "slate";
  }
}

export function recommendationActionIcon(action: string): LucideIcon {
  switch (action) {
    case "RENEW":
      return CheckCircle2;
    case "REVIEW":
      return AlertTriangle;
    case "DROP":
      return XCircle;
    case "PROFIT_OPPORTUNITY":
      return Sparkles;
    default:
      return HelpCircle;
  }
}

export { RECOMMENDATION_ACTIONS };
