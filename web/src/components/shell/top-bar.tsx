"use client";

import { Menu } from "lucide-react";
import { useTranslations } from "next-intl";
import { Breadcrumb } from "@/components/shell/breadcrumb";
import { LanguageSelector } from "@/components/shell/language-selector";
import { ReadinessIndicator } from "@/components/shell/readiness-indicator";
import { UserMenu } from "@/components/shell/user-menu";
import type { AuthUser } from "@/lib/auth/types";

interface TopBarProps {
  user: AuthUser;
  onMenuClick: () => void;
}

export function TopBar({ user, onMenuClick }: TopBarProps) {
  const t = useTranslations("common");

  return (
    <header className="flex h-14 shrink-0 items-center gap-3 border-b border-border bg-background px-4">
      <button
        type="button"
        onClick={onMenuClick}
        aria-label={t("menu")}
        className="rounded-md p-2 text-foreground hover:bg-surface lg:hidden"
      >
        <Menu size={20} aria-hidden />
      </button>
      <div className="min-w-0 flex-1">
        <Breadcrumb />
      </div>
      <div className="flex shrink-0 items-center gap-2">
        <ReadinessIndicator />
        <LanguageSelector />
        <UserMenu user={user} />
      </div>
    </header>
  );
}
