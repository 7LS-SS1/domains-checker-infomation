import { describe, expect, it, vi, beforeEach } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { NextIntlClientProvider } from "next-intl";
import type { ReactNode } from "react";
import { useCreateReport } from "@/lib/reports/queries";
import enMessages from "@/messages/en.json";

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return (
    <NextIntlClientProvider locale="en" messages={enMessages}>
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    </NextIntlClientProvider>
  );
}

function mockReportResponse() {
  return new Response(
    JSON.stringify({
      data: {
        id: "11111111-1111-4111-8111-111111111111",
        report_type: "summary",
        format: "json",
        status: "completed",
        filters: {},
        snapshot_at: "2026-08-29T00:00:00Z",
        policy_versions: {},
        completeness_warnings: [],
        row_count: 1,
        storage_reference: "database:report_payloads",
        sha256: "abc123",
        requested_by: "22222222-2222-4222-8222-222222222222",
        requested_at: "2026-08-29T00:00:00Z",
        completed_at: "2026-08-29T00:00:00Z",
      },
    }),
    { status: 201, headers: { "Content-Type": "application/json" } },
  );
}

describe("useCreateReport (idempotency-key freshness)", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("generates a fresh Idempotency-Key on every distinct submission, never reusing one", async () => {
    const fetchSpy = vi.spyOn(globalThis, "fetch").mockResolvedValue(mockReportResponse());
    const { result } = renderHook(() => useCreateReport(), { wrapper });

    result.current.mutate({ format: "json", reporting_currency: "THB" });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    result.current.mutate({ format: "csv", reporting_currency: "USD" });
    await waitFor(() => expect(fetchSpy).toHaveBeenCalledTimes(2));

    const firstCallHeaders = new Headers(fetchSpy.mock.calls[0]?.[1]?.headers);
    const secondCallHeaders = new Headers(fetchSpy.mock.calls[1]?.[1]?.headers);
    const firstKey = firstCallHeaders.get("Idempotency-Key");
    const secondKey = secondCallHeaders.get("Idempotency-Key");

    expect(firstKey).toBeTruthy();
    expect(secondKey).toBeTruthy();
    expect(firstKey).not.toBe(secondKey);
  });
});
