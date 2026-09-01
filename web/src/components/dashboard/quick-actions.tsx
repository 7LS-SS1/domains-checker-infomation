"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";
import { FileSpreadsheet, FileText, HardDrive, PlusCircle, Sparkles } from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { hasCapability, toKnownRoles, type Capability } from "@/lib/auth/capability";

interface QuickAction {
  key: string;
  href: string;
  icon: LucideIcon;
  capability: Capability;
}

const ACTIONS: readonly QuickAction[] = [
  {
    key: "addDomain",
    href: "/domains?action=create",
    icon: PlusCircle,
    capability: "editDomains",
  },
  {
    key: "runRecommendations",
    href: "/recommendations",
    icon: Sparkles,
    capability: "generateRecommendation",
  },
  {
    key: "importExcel",
    href: "/sync?tab=excel",
    icon: FileSpreadsheet,
    capability: "previewImport",
  },
  {
    key: "connectDrive",
    href: "/sync?tab=drive",
    icon: HardDrive,
    capability: "manageDriveConnection",
  },
  {
    key: "createReport",
    href: "/reports?action=create",
    icon: FileText,
    capability: "createReport",
  },
];

/**
 * Every entry is gated by the same RBAC capability matrix the backend
 * enforces (web/API_CONTRACT_MATRIX.md), so a role that cannot use an
 * action never even renders its button — no API call is ever attempted for
 * a capability the current session lacks.
 */
export function QuickActions({ roles }: { roles: readonly string[] }) {
  const t = useTranslations("dashboard.quickActions");
  const knownRoles = toKnownRoles(roles);
  const visible = ACTIONS.filter((action) => hasCapability(knownRoles, action.capability));

  if (visible.length === 0) {
    return null;
  }

  return (
    <section aria-labelledby="quick-actions-heading">
      <h2 id="quick-actions-heading" className="mb-2 text-sm font-semibold text-foreground">
        {t("title")}
      </h2>
      <div className="flex flex-wrap gap-2">
        {visible.map((action) => (
          <Button key={action.key} asChild variant="secondary" size="sm">
            <Link href={action.href}>
              <action.icon size={16} aria-hidden />
              {t(action.key)}
            </Link>
          </Button>
        ))}
      </div>
    </section>
  );
}
