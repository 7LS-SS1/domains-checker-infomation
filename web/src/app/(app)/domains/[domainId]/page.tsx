import { getSession, SessionUnavailableError } from "@/lib/auth/session";
import { DomainDetailContent } from "@/components/domains/domain-detail-content";

interface DomainDetailPageProps {
  params: Promise<{ domainId: string }>;
}

export default async function DomainDetailPage({ params }: DomainDetailPageProps) {
  const { domainId } = await params;
  const session = await getSession().catch((error) => {
    if (error instanceof SessionUnavailableError) {
      return null;
    }
    throw error;
  });

  return <DomainDetailContent domainId={domainId} roles={session?.roles ?? []} />;
}
