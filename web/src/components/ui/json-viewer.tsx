"use client";

import { useState } from "react";
import { ChevronDown, ChevronRight, Copy } from "lucide-react";
import { useTranslations } from "next-intl";
import { redactSensitiveKeys } from "@/lib/utils/redact";

interface JsonViewerProps {
  data: unknown;
  label: string;
  defaultOpen?: boolean;
}

/**
 * Collapsible raw-evidence viewer. Data is always passed through
 * redactSensitiveKeys before rendering or copying — defense-in-depth
 * against a future backend field accidentally carrying a secret into a
 * "view raw JSON" surface.
 */
export function JsonViewer({ data, label, defaultOpen = false }: JsonViewerProps) {
  const [open, setOpen] = useState(defaultOpen);
  const t = useTranslations("common");
  const safeData = redactSensitiveKeys(data);
  const json = JSON.stringify(safeData, null, 2);

  async function handleCopy() {
    try {
      await navigator.clipboard.writeText(json);
    } catch {
      // Clipboard API can be unavailable (permissions, insecure context) —
      // the JSON remains visible and manually selectable either way.
    }
  }

  return (
    <div className="rounded-md border border-border">
      <button
        type="button"
        onClick={() => setOpen((value) => !value)}
        className="flex w-full items-center gap-1.5 px-3 py-2 text-sm font-medium text-foreground"
        aria-expanded={open}
      >
        {open ? <ChevronDown size={14} aria-hidden /> : <ChevronRight size={14} aria-hidden />}
        {label}
      </button>
      {open && (
        <div className="border-t border-border p-3">
          <div className="mb-2 flex justify-end">
            <button
              type="button"
              onClick={handleCopy}
              className="inline-flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground"
            >
              <Copy size={12} aria-hidden />
              {t("copy")}
            </button>
          </div>
          <pre className="max-h-96 overflow-auto rounded bg-surface p-2 text-xs text-foreground">
            <code>{json}</code>
          </pre>
        </div>
      )}
    </div>
  );
}
