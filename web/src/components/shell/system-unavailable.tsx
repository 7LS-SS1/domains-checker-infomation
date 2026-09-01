import { getTranslations } from "next-intl/server";
import { AlertTriangle } from "lucide-react";

/**
 * Rendered when the auth service itself could not be reached — distinct
 * from "not logged in". Never redirects (that would silently masquerade a
 * backend outage as a login prompt), per
 * web/FRONTEND_ARCHITECTURE.md section 1.3.
 */
export async function SystemUnavailable() {
  const t = await getTranslations("systemUnavailable");

  return (
    <main
      id="main-content"
      className="flex min-h-screen items-center justify-center bg-surface px-4"
    >
      <div
        role="alert"
        className="flex max-w-sm flex-col items-center gap-3 rounded-lg border border-status-rose-border bg-background p-8 text-center shadow-sm"
      >
        <AlertTriangle size={32} className="text-status-rose-fg" aria-hidden />
        <h1 className="text-lg font-semibold text-foreground">{t("title")}</h1>
        <p className="text-sm text-muted-foreground">{t("description")}</p>
        <a
          href="."
          className="mt-2 rounded-md border border-border px-4 py-2 text-sm font-medium text-foreground hover:bg-surface"
        >
          {t("retry")}
        </a>
      </div>
    </main>
  );
}
