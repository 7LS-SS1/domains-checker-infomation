import { Suspense } from "react";
import { IncidentsPageContent } from "@/components/incidents/incidents-page-content";
import { Skeleton } from "@/components/ui/skeleton";

export default function IncidentsPage() {
  return (
    <Suspense fallback={<Skeleton className="h-96 w-full" />}>
      <IncidentsPageContent />
    </Suspense>
  );
}
