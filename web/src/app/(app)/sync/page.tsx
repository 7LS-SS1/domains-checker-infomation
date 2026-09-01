import { Suspense } from "react";
import { getSession, SessionUnavailableError } from "@/lib/auth/session";
import { SyncPageContent } from "@/components/sync/sync-page-content";
import { Skeleton } from "@/components/ui/skeleton";

export default async function SyncPage() {
  const session = await getSession().catch((error) => {
    if (error instanceof SessionUnavailableError) {
      return null;
    }
    throw error;
  });

  return (
    <Suspense fallback={<Skeleton className="h-96 w-full" />}>
      <SyncPageContent roles={session?.roles ?? []} />
    </Suspense>
  );
}
