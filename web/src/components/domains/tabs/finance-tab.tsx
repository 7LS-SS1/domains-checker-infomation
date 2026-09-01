"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { Plus } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { DateValue } from "@/components/ui/date-value";
import { MoneyValue } from "@/components/ui/money-value";
import { describeQueryError } from "@/lib/api/query-error";
import { useDomainCosts } from "@/lib/finance/queries";
import { hasCapability, toKnownRoles } from "@/lib/auth/capability";
import { AddCostDialog } from "@/components/domains/tabs/add-cost-dialog";

export function FinanceTab({ domainId, roles }: { domainId: string; roles: readonly string[] }) {
  const t = useTranslations("domainDetail.finance");
  const tCommon = useTranslations("common");
  const [dialogOpen, setDialogOpen] = useState(false);
  const query = useDomainCosts(domainId);
  const canAdd = hasCapability(toKnownRoles(roles), "editDomains");

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("title")}</CardTitle>
        {canAdd && (
          <Button size="sm" onClick={() => setDialogOpen(true)}>
            <Plus size={14} aria-hidden />
            {t("addCost")}
          </Button>
        )}
      </CardHeader>
      <CardContent>
        {query.isError ? (
          <ErrorState
            title={tCommon("loadError")}
            description={describeQueryError(query.error).message}
            requestId={describeQueryError(query.error).requestId}
            onRetry={() => query.refetch()}
          />
        ) : query.isPending ? (
          <Skeleton className="h-24 w-full" />
        ) : query.data.length === 0 ? (
          <EmptyState title={t("noCosts")} />
        ) : (
          <div className="overflow-x-auto" tabIndex={0} role="region" aria-label={t("title")}>
            <table className="w-full min-w-[640px] text-sm">
              <tbody>
                {query.data.map((cost) => (
                  <tr key={cost.id} className="border-b border-border last:border-0">
                    <td className="py-2 pr-3 font-medium text-foreground">{cost.cost_type}</td>
                    <td className="py-2 pr-3">
                      <MoneyValue amount={cost.amount} currency={cost.currency} />
                    </td>
                    <td className="py-2 pr-3 text-muted-foreground">{cost.tax_mode}</td>
                    <td className="py-2 pr-3 text-muted-foreground">{cost.price_source}</td>
                    <td className="py-2 text-muted-foreground">
                      <DateValue value={cost.effective_from} dateOnly />
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </CardContent>

      {canAdd && (
        <AddCostDialog open={dialogOpen} onOpenChange={setDialogOpen} domainId={domainId} />
      )}
    </Card>
  );
}
