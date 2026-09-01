"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import { CurrencySelector } from "@/components/dashboard/currency-selector";
import { BudgetOverview } from "@/components/dashboard/budget-overview";
import { useFinanceSummary } from "@/lib/finance/queries";
import { hasCapability, toKnownRoles } from "@/lib/auth/capability";
import { AddExchangeRateDialog } from "@/components/finance/add-exchange-rate-dialog";

export function FinancePageContent({ roles }: { roles: readonly string[] }) {
  const t = useTranslations("financePage");
  const [reportingCurrency, setReportingCurrency] = useState("THB");
  const [dialogOpen, setDialogOpen] = useState(false);
  const query = useFinanceSummary(reportingCurrency);
  const canManageRates = hasCapability(toKnownRoles(roles), "manageExchangeRates");

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-xl font-semibold text-foreground">{t("title")}</h1>
        <div className="flex items-center gap-3">
          <CurrencySelector value={reportingCurrency} onChange={setReportingCurrency} />
          {canManageRates && (
            <Button size="sm" onClick={() => setDialogOpen(true)}>
              <Plus size={14} aria-hidden />
              {t("addRate")}
            </Button>
          )}
        </div>
      </div>

      <BudgetOverview query={query} />

      {canManageRates && <AddExchangeRateDialog open={dialogOpen} onOpenChange={setDialogOpen} />}
    </div>
  );
}
