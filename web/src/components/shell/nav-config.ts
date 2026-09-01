import {
  AlertTriangle,
  DollarSign,
  FileText,
  Globe,
  LayoutDashboard,
  RefreshCw,
  Settings,
  Sparkles,
  Radio,
  type LucideIcon,
} from "lucide-react";

export interface NavItem {
  href: string;
  /** Key inside the "nav" i18n namespace. */
  labelKey: string;
  icon: LucideIcon;
}

/**
 * Only real, built routes belong here. Master Prompt Phase Frontend 2
 * explicitly forbids faking completeness — a nav entry pointing at a page
 * that doesn't exist yet would do exactly that, so this list grows one
 * entry per future phase as each page actually ships.
 */
export const NAV_ITEMS: readonly NavItem[] = [
  { href: "/dashboard", labelKey: "dashboard", icon: LayoutDashboard },
  { href: "/domains", labelKey: "domains", icon: Globe },
  { href: "/incidents", labelKey: "incidents", icon: AlertTriangle },
  { href: "/finance", labelKey: "finance", icon: DollarSign },
  { href: "/recommendations", labelKey: "recommendations", icon: Sparkles },
  { href: "/reports", labelKey: "reports", icon: FileText },
  { href: "/sync", labelKey: "sync", icon: RefreshCw },
  { href: "/probes", labelKey: "probes", icon: Radio },
  { href: "/settings", labelKey: "settings", icon: Settings },
];
