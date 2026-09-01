import { redirect } from "next/navigation";
import { getTranslations } from "next-intl/server";
import { getSession, SessionUnavailableError } from "@/lib/auth/session";
import { sanitizeReturnTo } from "@/lib/auth/return-to";
import { LoginForm } from "@/components/auth/login-form";

interface LoginPageProps {
  searchParams: Promise<{ returnTo?: string; reason?: string }>;
}

export default async function LoginPage({ searchParams }: LoginPageProps) {
  const params = await searchParams;
  const returnTo = sanitizeReturnTo(params.returnTo);

  try {
    const session = await getSession();
    if (session) {
      redirect(returnTo);
    }
  } catch (error) {
    if (!(error instanceof SessionUnavailableError)) {
      throw error;
    }
    // Backend unreachable: fall through and still render the login form so
    // the user can attempt a sign-in, which will surface the same network
    // error inline via LoginForm's own error handling.
  }

  const t = await getTranslations("auth");
  const showExpiredNotice = params.reason === "expired";

  return (
    <main
      id="main-content"
      className="flex min-h-screen items-center justify-center bg-surface px-4 py-12"
    >
      <div className="w-full max-w-sm space-y-6 rounded-lg border border-border bg-background p-8 shadow-sm">
        <div className="space-y-1 text-center">
          <h1 className="text-xl font-semibold text-foreground">{t("loginTitle")}</h1>
          <p className="text-sm text-muted-foreground">{t("loginSubtitle")}</p>
        </div>
        {showExpiredNotice && (
          <p
            role="status"
            className="rounded-md border border-status-amber-border bg-status-amber-bg px-3 py-2 text-sm text-status-amber-fg"
          >
            {t("sessionExpired")}
          </p>
        )}
        <LoginForm returnTo={returnTo} />
      </div>
    </main>
  );
}
