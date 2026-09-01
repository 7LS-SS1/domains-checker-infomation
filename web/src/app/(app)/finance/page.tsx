import { getSession, SessionUnavailableError } from "@/lib/auth/session";
import { FinancePageContent } from "@/components/finance/finance-page-content";

export default async function FinancePage() {
  const session = await getSession().catch((error) => {
    if (error instanceof SessionUnavailableError) {
      return null;
    }
    throw error;
  });

  return <FinancePageContent roles={session?.roles ?? []} />;
}
