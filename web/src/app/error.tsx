"use client";

import { useEffect } from "react";
import { useTranslations } from "next-intl";
import { AlertTriangle } from "lucide-react";
import { Button } from "@/components/ui/button";

export default function GlobalError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  const t = useTranslations("errors");
  const tCommon = useTranslations("common");

  useEffect(() => {
    // Client-side error boundary — nothing more sensitive than a stack
    // trace goes to the console here; no request body/secret is ever
    // available at this layer.

    console.error(error);
  }, [error]);

  return (
    <div
      role="alert"
      className="flex min-h-screen flex-col items-center justify-center gap-3 bg-surface px-4 text-center"
    >
      <AlertTriangle size={32} className="text-status-rose-fg" aria-hidden />
      <h1 className="text-lg font-semibold text-foreground">{t("unexpectedTitle")}</h1>
      <p className="max-w-sm text-sm text-muted-foreground">{t("unexpectedDescription")}</p>
      {error.digest && (
        <p className="text-xs text-muted-foreground">
          {tCommon("requestId")}: {error.digest}
        </p>
      )}
      <Button onClick={reset}>{tCommon("retry")}</Button>
    </div>
  );
}
