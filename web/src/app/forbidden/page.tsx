import Link from "next/link";
import { ShieldAlert } from "lucide-react";
import { getTranslations } from "next-intl/server";

/**
 * Rendered when a backend call returns 403 FORBIDDEN for an authenticated
 * user who simply lacks permission for that specific action — distinct
 * from 401, which means "not authenticated" and belongs on /login. This
 * page contains no redirect logic of any kind, satisfying the Master
 * Prompt requirement "403 เป็น permission page ไม่วน redirect".
 */
export default async function ForbiddenPage() {
  const t = await getTranslations("forbidden");

  return (
    <main
      id="main-content"
      className="flex min-h-screen items-center justify-center bg-surface px-4"
    >
      <div
        role="alert"
        className="flex max-w-sm flex-col items-center gap-3 rounded-lg border border-status-amber-border bg-background p-8 text-center shadow-sm"
      >
        <ShieldAlert size={32} className="text-status-amber-fg" aria-hidden />
        <h1 className="text-lg font-semibold text-foreground">{t("title")}</h1>
        <p className="text-sm text-muted-foreground">{t("description")}</p>
        <Link
          href="/dashboard"
          className="mt-2 rounded-md border border-border px-4 py-2 text-sm font-medium text-foreground hover:bg-surface"
        >
          {t("backToDashboard")}
        </Link>
      </div>
    </main>
  );
}
