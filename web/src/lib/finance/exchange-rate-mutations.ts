"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useLocale } from "next-intl";
import { bffFetch } from "@/lib/api/client";
import { rateRecordSchema } from "@/lib/finance/types";

export interface AddExchangeRateInput {
  base_currency: string;
  quote_currency: string;
  rate: string;
  source: string;
  observed_at: string;
  reason: string;
}

export function useAddExchangeRate() {
  const locale = useLocale();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: AddExchangeRateInput) =>
      bffFetch("/api/bff/finance/exchange-rates", rateRecordSchema, {
        method: "POST",
        body: JSON.stringify(input),
        locale,
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["finance-summary"] });
    },
  });
}
