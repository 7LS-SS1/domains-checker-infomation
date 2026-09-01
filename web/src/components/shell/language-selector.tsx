"use client";

import { useTransition } from "react";
import { useLocale, useTranslations } from "next-intl";
import { useRouter } from "next/navigation";
import { Languages } from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { setLocaleAction } from "@/lib/actions/set-locale";
import { locales, type AppLocale } from "@/i18n/config";
import { cn } from "@/lib/utils/cn";

export function LanguageSelector() {
  const locale = useLocale();
  const t = useTranslations("common");
  const router = useRouter();
  const [isPending, startTransition] = useTransition();

  function handleSelect(next: AppLocale) {
    if (next === locale) return;
    startTransition(async () => {
      await setLocaleAction(next);
      router.refresh();
    });
  }

  const labels: Record<AppLocale, string> = {
    th: t("languageThai"),
    en: t("languageEnglish"),
  };

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          disabled={isPending}
          aria-label={t("languageLabel")}
          className="inline-flex h-9 items-center gap-1.5 rounded-md border border-border px-2.5 text-sm text-foreground hover:bg-surface disabled:opacity-60"
        >
          <Languages size={16} aria-hidden />
          <span>{labels[locale as AppLocale] ?? locale}</span>
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        {locales.map((code) => (
          <DropdownMenuItem
            key={code}
            onSelect={() => handleSelect(code)}
            aria-current={code === locale}
            className={cn(code === locale && "font-semibold")}
          >
            {labels[code]}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
