import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { QueryClient, QueryClientProvider, useQuery } from "@tanstack/react-query";
import { BudgetOverview } from "@/components/dashboard/budget-overview";
import type { FinanceSummary } from "@/lib/finance/types";
import enMessages from "@/messages/en.json";

function Harness({
  summary,
  shouldError = false,
}: {
  summary: FinanceSummary;
  shouldError?: boolean;
}) {
  const query = useQuery({
    queryKey: ["test-finance-summary", shouldError],
    queryFn: () => (shouldError ? Promise.reject(new Error("boom")) : Promise.resolve(summary)),
    retry: false,
  });
  return <BudgetOverview query={query} />;
}

function renderHarness(summary: FinanceSummary, shouldError = false) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <NextIntlClientProvider locale="en" messages={enMessages}>
      <QueryClientProvider client={queryClient}>
        <Harness summary={summary} shouldError={shouldError} />
      </QueryClientProvider>
    </NextIntlClientProvider>,
  );
}

const completeSummary: FinanceSummary = {
  reporting_currency: "THB",
  generated_at: "2026-08-29T10:00:00Z",
  total_current_domain_cost: "1234567.999999",
  total_renewal_cost: "45230.500000",
  estimated_tax: "3166.135000",
  total_annual_budget: "180922.000000",
  complete: true,
  unknown_cost_count: 0,
  unknown_tax_count: 0,
  fx_incomplete_count: 0,
  warnings: [],
  windows: {
    next_30_days: {
      domain_count: 4,
      known_renewals: 4,
      unknown_costs: 0,
      renewal_total: "5000.00",
    },
    next_60_days: {
      domain_count: 6,
      known_renewals: 6,
      unknown_costs: 0,
      renewal_total: "7000.00",
    },
    next_90_days: {
      domain_count: 9,
      known_renewals: 9,
      unknown_costs: 0,
      renewal_total: "9000.00",
    },
    this_year: {
      domain_count: 40,
      known_renewals: 38,
      unknown_costs: 2,
      renewal_total: "40000.00",
    },
  },
};

describe("BudgetOverview", () => {
  it("renders exact decimal amounts with thousands separators, never a float-coerced value", async () => {
    renderHarness(completeSummary);
    expect(await screen.findByText("1,234,567.999999 THB")).toBeInTheDocument();
    expect(screen.getByText("Data complete")).toBeInTheDocument();
  });

  it("shows the incomplete badge and translated + raw fallback warnings", async () => {
    const incomplete: FinanceSummary = {
      ...completeSummary,
      complete: false,
      warnings: ["FX_RATE_MISSING_OR_STALE", "SOME_FUTURE_UNKNOWN_CODE"],
    };
    renderHarness(incomplete);

    expect(await screen.findByText("Data incomplete")).toBeInTheDocument();
    expect(
      screen.getByText("An exchange rate is missing or too old for some currencies"),
    ).toBeInTheDocument();
    // Unknown codes must still render (honestly) as the raw code, never be hidden.
    expect(screen.getByText("SOME_FUTURE_UNKNOWN_CODE")).toBeInTheDocument();
  });

  it("shows an error state with a retry button when the query fails", async () => {
    renderHarness(completeSummary, true);
    expect(await screen.findByRole("alert")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /retry/i })).toBeInTheDocument();
    expect(screen.queryByText("Data complete")).not.toBeInTheDocument();
  });
});
