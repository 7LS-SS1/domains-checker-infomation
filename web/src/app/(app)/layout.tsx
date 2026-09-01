import { redirect } from "next/navigation";
import type { ReactNode } from "react";
import { getSession, SessionUnavailableError } from "@/lib/auth/session";
import { AppShell } from "@/components/shell/app-shell";
import { SystemUnavailable } from "@/components/shell/system-unavailable";

export default async function AuthenticatedLayout({ children }: { children: ReactNode }) {
  let session;
  try {
    session = await getSession();
  } catch (error) {
    if (error instanceof SessionUnavailableError) {
      return <SystemUnavailable />;
    }
    throw error;
  }

  if (!session) {
    // Defense in depth: src/middleware.ts already validates every request
    // under the matched prefixes and redirects with a returnTo before this
    // layout ever renders. This branch only fires if that guard is somehow
    // bypassed (e.g. a future route added here without updating the
    // middleware matcher) or the session was revoked in the few
    // milliseconds between the two checks.
    redirect("/login");
  }

  return <AppShell user={session}>{children}</AppShell>;
}
