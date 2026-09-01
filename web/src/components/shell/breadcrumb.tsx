"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useTranslations } from "next-intl";
import { ChevronRight } from "lucide-react";

const SEGMENT_LABEL_KEYS: Record<string, string> = {
  dashboard: "dashboard",
  domains: "domains",
  incidents: "incidents",
  finance: "finance",
  recommendations: "recommendations",
  reports: "reports",
  sync: "sync",
  probes: "probes",
  settings: "settings",
};

export function Breadcrumb() {
  const pathname = usePathname();
  const t = useTranslations("nav");
  const tCommon = useTranslations("common");
  const segments = pathname.split("/").filter(Boolean);

  if (segments.length === 0) {
    return null;
  }

  return (
    <nav aria-label={tCommon("breadcrumb")} className="flex items-center gap-1 text-sm">
      {segments.map((segment, index) => {
        const href = `/${segments.slice(0, index + 1).join("/")}`;
        const isLast = index === segments.length - 1;
        const labelKey = SEGMENT_LABEL_KEYS[segment];
        const label = labelKey ? t(labelKey) : segment;
        return (
          <span key={href} className="flex items-center gap-1">
            {index > 0 && <ChevronRight size={14} className="text-muted-foreground" aria-hidden />}
            {isLast ? (
              <span aria-current="page" className="font-medium text-foreground">
                {label}
              </span>
            ) : (
              <Link
                href={href}
                className="text-muted-foreground hover:text-foreground hover:underline"
              >
                {label}
              </Link>
            )}
          </span>
        );
      })}
    </nav>
  );
}
