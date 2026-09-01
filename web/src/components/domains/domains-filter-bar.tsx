"use client";

import { useEffect, useId, useState } from "react";
import { useTranslations } from "next-intl";
import { Search } from "lucide-react";
import { Input } from "@/components/ui/input";
import { LIFECYCLE_STATUSES, SOURCE_STATUSES } from "@/lib/domains/types";

interface DomainsFilterBarProps {
  query: string;
  lifecycleStatus: string;
  sourceStatus: string;
  onQueryChange: (value: string) => void;
  onLifecycleChange: (value: string) => void;
  onSourceChange: (value: string) => void;
}

/** Debounces free-text search input before pushing it into the URL/query. */
export function DomainsFilterBar({
  query,
  lifecycleStatus,
  sourceStatus,
  onQueryChange,
  onLifecycleChange,
  onSourceChange,
}: DomainsFilterBarProps) {
  const t = useTranslations("domains");
  const tStatus = useTranslations("domainStatus");
  const [localQuery, setLocalQuery] = useState(query);
  const searchId = useId();
  const lifecycleId = useId();
  const sourceId = useId();

  // Keeps the input in sync when `query` changes externally (e.g. browser
  // back/forward navigating the URL). A render-time "adjust state" version
  // of this was tried and measurably broke controlled-input typing under
  // this stack (React Compiler + userEvent) — verified by a reproducible
  // scrambled-input test failure on an equivalent pattern in
  // ConfirmDialog — so this legitimate "synchronize with the URL" effect
  // keeps the lint suppression deliberately.
  // eslint-disable-next-line react-hooks/set-state-in-effect
  useEffect(() => setLocalQuery(query), [query]);

  useEffect(() => {
    const handle = setTimeout(() => {
      if (localQuery !== query) {
        onQueryChange(localQuery);
      }
    }, 350);
    return () => clearTimeout(handle);
    // eslint-disable-next-line react-hooks/exhaustive-deps -- only re-run on localQuery changes; onQueryChange/query are stable enough for this debounce
  }, [localQuery]);

  return (
    <div className="flex flex-wrap items-end gap-3">
      <div className="min-w-[220px] flex-1 space-y-1.5">
        <label htmlFor={searchId} className="text-xs font-medium text-muted-foreground">
          {t("searchPlaceholder")}
        </label>
        <div className="relative">
          <Search
            size={16}
            className="absolute left-2.5 top-1/2 -translate-y-1/2 text-muted-foreground"
            aria-hidden
          />
          <Input
            id={searchId}
            value={localQuery}
            onChange={(event) => setLocalQuery(event.target.value)}
            placeholder={t("searchPlaceholder")}
            className="pl-8"
          />
        </div>
      </div>

      <div className="space-y-1.5">
        <label htmlFor={lifecycleId} className="text-xs font-medium text-muted-foreground">
          {t("filterLifecycle")}
        </label>
        <select
          id={lifecycleId}
          value={lifecycleStatus}
          onChange={(event) => onLifecycleChange(event.target.value)}
          className="h-10 rounded-md border border-border bg-background px-2 text-sm text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          <option value="">{t("allOption")}</option>
          {LIFECYCLE_STATUSES.map((status) => (
            <option key={status} value={status}>
              {tStatus(`lifecycle.${status}`)}
            </option>
          ))}
        </select>
      </div>

      <div className="space-y-1.5">
        <label htmlFor={sourceId} className="text-xs font-medium text-muted-foreground">
          {t("filterSource")}
        </label>
        <select
          id={sourceId}
          value={sourceStatus}
          onChange={(event) => onSourceChange(event.target.value)}
          className="h-10 rounded-md border border-border bg-background px-2 text-sm text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          <option value="">{t("allOption")}</option>
          {SOURCE_STATUSES.map((status) => (
            <option key={status} value={status}>
              {tStatus(`source.${status}`)}
            </option>
          ))}
        </select>
      </div>
    </div>
  );
}
