"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";
import { ArrowLeft } from "lucide-react";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Skeleton } from "@/components/ui/skeleton";
import { ErrorState } from "@/components/ui/error-state";
import { describeQueryError } from "@/lib/api/query-error";
import { useDomain } from "@/lib/domains/queries";
import { DomainDetailHeader } from "@/components/domains/domain-detail-header";
import { OverviewTab } from "@/components/domains/tabs/overview-tab";
import { MonitoringTab } from "@/components/domains/tabs/monitoring-tab";
import { RdapTab } from "@/components/domains/tabs/rdap-tab";
import { FinanceTab } from "@/components/domains/tabs/finance-tab";
import { OverridesTab } from "@/components/domains/tabs/overrides-tab";
import { RecommendationTab } from "@/components/domains/tabs/recommendation-tab";
import { ProvenanceTab } from "@/components/domains/tabs/provenance-tab";

export function DomainDetailContent({
  domainId,
  roles,
}: {
  domainId: string;
  roles: readonly string[];
}) {
  const t = useTranslations("domainDetail");
  const tCommon = useTranslations("common");
  const domainQuery = useDomain(domainId);

  if (domainQuery.isPending) {
    return (
      <div className="space-y-4">
        <Skeleton className="h-8 w-64" />
        <Skeleton className="h-40 w-full" />
      </div>
    );
  }

  if (domainQuery.isError) {
    const info = describeQueryError(domainQuery.error);
    return (
      <ErrorState
        title={tCommon("loadError")}
        description={info.message ?? t("notFound")}
        requestId={info.requestId}
        onRetry={() => domainQuery.refetch()}
      />
    );
  }

  const domain = domainQuery.data;

  return (
    <div className="space-y-4">
      <Link
        href="/domains"
        className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
      >
        <ArrowLeft size={14} aria-hidden />
        {t("backToDomains")}
      </Link>

      <DomainDetailHeader domain={domain} roles={roles} />

      <Tabs defaultValue="overview">
        <TabsList aria-label={t("tabs.overview")}>
          <TabsTrigger value="overview">{t("tabs.overview")}</TabsTrigger>
          <TabsTrigger value="monitoring">{t("tabs.monitoring")}</TabsTrigger>
          <TabsTrigger value="rdap">{t("tabs.rdap")}</TabsTrigger>
          <TabsTrigger value="finance">{t("tabs.finance")}</TabsTrigger>
          <TabsTrigger value="overrides">{t("tabs.overrides")}</TabsTrigger>
          <TabsTrigger value="recommendation">{t("tabs.recommendation")}</TabsTrigger>
          <TabsTrigger value="provenance">{t("tabs.provenance")}</TabsTrigger>
        </TabsList>

        <TabsContent value="overview">
          <OverviewTab domain={domain} roles={roles} />
        </TabsContent>
        <TabsContent value="monitoring">
          <MonitoringTab domainId={domainId} />
        </TabsContent>
        <TabsContent value="rdap">
          <RdapTab domainId={domainId} roles={roles} />
        </TabsContent>
        <TabsContent value="finance">
          <FinanceTab domainId={domainId} roles={roles} />
        </TabsContent>
        <TabsContent value="overrides">
          <OverridesTab domainId={domainId} roles={roles} />
        </TabsContent>
        <TabsContent value="recommendation">
          <RecommendationTab domainId={domainId} roles={roles} />
        </TabsContent>
        <TabsContent value="provenance">
          <ProvenanceTab domainId={domainId} />
        </TabsContent>
      </Tabs>
    </div>
  );
}
