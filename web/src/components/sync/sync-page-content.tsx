"use client";

import { useEffect } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { useTranslations } from "next-intl";
import { toast } from "sonner";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { DriveTab } from "@/components/sync/drive-tab";
import { SheetConfigTab } from "@/components/sync/sheet-config-tab";
import { ExcelTab } from "@/components/sync/excel-tab";
import { ImportHistoryTab } from "@/components/sync/import-history-tab";

const VALID_TABS = ["drive", "sheet", "excel", "history"] as const;

export function SyncPageContent({ roles }: { roles: readonly string[] }) {
  const t = useTranslations("sync");
  const tDrive = useTranslations("sync.drive");
  const router = useRouter();
  const searchParams = useSearchParams();
  const requestedTab = searchParams.get("tab");
  const activeTab = VALID_TABS.includes(requestedTab as (typeof VALID_TABS)[number])
    ? (requestedTab as (typeof VALID_TABS)[number])
    : "drive";

  const driveConnected = searchParams.get("driveConnected");

  useEffect(() => {
    if (driveConnected === "true") {
      toast.success(tDrive("connectedBanner"));
    } else if (driveConnected === "false") {
      toast.error(tDrive("connectFailedBanner"));
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps -- fire once per redirect landing, not on every param object identity change
  }, [driveConnected]);

  return (
    <div className="space-y-4">
      <h1 className="text-xl font-semibold text-foreground">{t("title")}</h1>

      <Tabs value={activeTab} onValueChange={(value) => router.replace(`/sync?tab=${value}`)}>
        <TabsList aria-label={t("title")}>
          <TabsTrigger value="drive">{t("tabs.drive")}</TabsTrigger>
          <TabsTrigger value="sheet">{t("tabs.sheet")}</TabsTrigger>
          <TabsTrigger value="excel">{t("tabs.excel")}</TabsTrigger>
          <TabsTrigger value="history">{t("tabs.history")}</TabsTrigger>
        </TabsList>
        <TabsContent value="drive">
          <DriveTab roles={roles} />
        </TabsContent>
        <TabsContent value="sheet">
          <SheetConfigTab roles={roles} />
        </TabsContent>
        <TabsContent value="excel">
          <ExcelTab roles={roles} />
        </TabsContent>
        <TabsContent value="history">
          <ImportHistoryTab roles={roles} />
        </TabsContent>
      </Tabs>
    </div>
  );
}
