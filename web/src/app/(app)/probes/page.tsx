import { Suspense } from "react";
import { getSession, SessionUnavailableError } from "@/lib/auth/session";
import { ProbesPageContent } from "@/components/probes/probes-page-content";
import { Skeleton } from "@/components/ui/skeleton";

export default async function ProbesPage() {
  const session = await getSession().catch((error) => {
    if (error instanceof SessionUnavailableError) {
      return null;
    }
    throw error;
  });

  return (
    <Suspense fallback={<Skeleton className="h-96 w-full" />}>
      <ProbesPageContent roles={session?.roles ?? []} />
    </Suspense>
  );
}
