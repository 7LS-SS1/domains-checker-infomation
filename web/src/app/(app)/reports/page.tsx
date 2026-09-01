import { getSession, SessionUnavailableError } from "@/lib/auth/session";
import { ReportsPageContent } from "@/components/reports/reports-page-content";

export default async function ReportsPage() {
  const session = await getSession().catch((error) => {
    if (error instanceof SessionUnavailableError) {
      return null;
    }
    throw error;
  });

  return <ReportsPageContent roles={session?.roles ?? []} />;
}
