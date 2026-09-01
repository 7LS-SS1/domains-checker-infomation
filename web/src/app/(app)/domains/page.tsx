import { Suspense } from "react";
import { getSession, SessionUnavailableError } from "@/lib/auth/session";
import { DomainsPageContent } from "@/components/domains/domains-page-content";
import { Skeleton } from "@/components/ui/skeleton";

export default async function DomainsPage() {
  const session = await getSession().catch((error) => {
    if (error instanceof SessionUnavailableError) {
      return null;
    }
    throw error;
  });

  return (
    <Suspense fallback={<Skeleton className="h-96 w-full" />}>
      <DomainsPageContent roles={session?.roles ?? []} />
    </Suspense>
  );
}
