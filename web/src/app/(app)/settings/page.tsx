import { redirect } from "next/navigation";
import { getSession, SessionUnavailableError } from "@/lib/auth/session";
import { SettingsPageContent } from "@/components/settings/settings-page-content";
import packageJson from "../../../../package.json";

export default async function SettingsPage() {
  const session = await getSession().catch((error) => {
    if (error instanceof SessionUnavailableError) {
      return null;
    }
    throw error;
  });

  if (!session) {
    redirect("/login");
  }

  return <SettingsPageContent session={session} buildVersion={packageJson.version} />;
}
