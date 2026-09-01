"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useLocale } from "next-intl";
import { z } from "zod";
import { bffFetch } from "@/lib/api/client";
import { costRecordSchema, overrideRecordSchema } from "@/lib/finance/types";

const messageResponseSchema = z.object({ message: z.string() });

export interface AddCostInput {
  domainId: string;
  cost_type: string;
  amount: string;
  currency: string;
  tax_rate?: string | null;
  tax_mode: string;
  billing_cycle_months: number;
  effective_from?: string;
  source_reference?: string;
  reason: string;
}

export function useAddDomainCost() {
  const locale = useLocale();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ domainId, ...body }: AddCostInput) =>
      bffFetch(`/api/bff/domains/${domainId}/costs`, costRecordSchema, {
        method: "POST",
        body: JSON.stringify(body),
        locale,
      }),
    onSuccess: (_data, variables) => {
      void queryClient.invalidateQueries({ queryKey: ["domain", variables.domainId, "costs"] });
    },
  });
}

export interface CreateOverrideInput {
  domainId: string;
  field_name: string;
  override_value: string;
  reason: string;
  expires_at?: string | null;
}

export function useCreateOverride() {
  const locale = useLocale();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ domainId, override_value, ...body }: CreateOverrideInput) =>
      bffFetch(`/api/bff/domains/${domainId}/overrides`, overrideRecordSchema, {
        method: "POST",
        // override_value must be a JSON string per the backend contract
        // (openapi.yaml ManualOverrideInput) — the field itself carries a
        // JSON-encoded string value, not a bare string.
        body: JSON.stringify({ ...body, override_value: JSON.stringify(override_value) }),
        locale,
      }),
    onSuccess: (_data, variables) => {
      void queryClient.invalidateQueries({ queryKey: ["domain", variables.domainId, "overrides"] });
      void queryClient.invalidateQueries({
        queryKey: ["domain", variables.domainId, "recommendation"],
      });
      void queryClient.invalidateQueries({ queryKey: ["domain", variables.domainId] });
    },
  });
}

export interface RevokeOverrideInput {
  domainId: string;
  overrideId: string;
  reason: string;
}

export function useRevokeOverride() {
  const locale = useLocale();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ domainId, overrideId, reason }: RevokeOverrideInput) =>
      bffFetch(`/api/bff/domains/${domainId}/overrides/${overrideId}`, messageResponseSchema, {
        method: "DELETE",
        body: JSON.stringify({ reason }),
        locale,
      }),
    onSuccess: (_data, variables) => {
      void queryClient.invalidateQueries({ queryKey: ["domain", variables.domainId, "overrides"] });
      void queryClient.invalidateQueries({
        queryKey: ["domain", variables.domainId, "recommendation"],
      });
      void queryClient.invalidateQueries({ queryKey: ["domain", variables.domainId] });
    },
  });
}
