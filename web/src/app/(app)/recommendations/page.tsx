import { getSession, SessionUnavailableError } from "@/lib/auth/session";
import { RecommendationsPageContent } from "@/components/recommendations/recommendations-page-content";

export default async function RecommendationsPage() {
  const session = await getSession().catch((error) => {
    if (error instanceof SessionUnavailableError) {
      return null;
    }
    throw error;
  });

  return <RecommendationsPageContent roles={session?.roles ?? []} />;
}
