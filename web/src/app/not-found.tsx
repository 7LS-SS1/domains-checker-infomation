import Link from "next/link";
import { getTranslations } from "next-intl/server";

export default async function NotFound() {
  const t = await getTranslations("errors");

  return (
    <div className="flex min-h-screen flex-col items-center justify-center gap-3 bg-surface px-4 text-center">
      <h1 className="text-lg font-semibold text-foreground">{t("notFoundTitle")}</h1>
      <p className="max-w-sm text-sm text-muted-foreground">{t("notFoundDescription")}</p>
      <Link
        href="/dashboard"
        className="rounded-md border border-border px-4 py-2 text-sm font-medium text-foreground hover:bg-surface"
      >
        {t("backHome")}
      </Link>
    </div>
  );
}
