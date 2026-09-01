import { getTranslations } from "next-intl/server";
import { getSession, SessionUnavailableError } from "@/lib/auth/session";
import { DashboardContent } from "@/components/dashboard/dashboard-content";

export default async function DashboardPage() {
  const t = await getTranslations("dashboard");

  // AuthenticatedLayout already guarantees a session by the time this page
  // renders; re-reading it here only to greet the user by name and to know
  // their roles for RBAC-gated quick actions, so a SessionUnavailableError
  // this deep would be extremely unusual (session revoked in the instant
  // between layout and page render) — treat it as a generic render error
  // rather than duplicating the layout's own system-unavailable UI.
  const session = await getSession().catch((error) => {
    if (error instanceof SessionUnavailableError) {
      return null;
    }
    throw error;
  });

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-xl font-semibold text-foreground">{t("title")}</h1>
        {session && (
          <p className="mt-1 text-sm text-muted-foreground">
            {t("welcome", { name: session.displayName })} · {t("rolesLabel")}:{" "}
            {session.roles.join(", ")}
          </p>
        )}
      </div>

      <DashboardContent roles={session?.roles ?? []} />
    </div>
  );
}
