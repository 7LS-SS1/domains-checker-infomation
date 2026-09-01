"use client";

import { useId } from "react";
import { useTranslations } from "next-intl";

/**
 * The backend has no currency-list endpoint (see web/API_GAPS.md) — this is
 * a fixed client-side list of currencies known to be valid seed data
 * (migrations create a `currencies` table but expose no route to read it).
 */
const CURRENCIES = ["THB", "USD", "EUR", "SGD", "GBP", "JPY"] as const;

interface CurrencySelectorProps {
  value: string;
  onChange: (value: string) => void;
}

export function CurrencySelector({ value, onChange }: CurrencySelectorProps) {
  const t = useTranslations("dashboard.currency");
  const id = useId();

  return (
    <div className="flex items-center gap-2">
      <label htmlFor={id} className="text-xs font-medium text-muted-foreground">
        {t("label")}
      </label>
      <select
        id={id}
        value={value}
        onChange={(event) => onChange(event.target.value)}
        className="h-9 rounded-md border border-border bg-background px-2 text-sm text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      >
        {CURRENCIES.map((code) => (
          <option key={code} value={code}>
            {code}
          </option>
        ))}
      </select>
    </div>
  );
}
