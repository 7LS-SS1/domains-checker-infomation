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

export function buildCreateOverrideBody({
  domainId: _domainId,
  ...body
}: CreateOverrideInput): string {
  // The outer JSON.stringify already encodes override_value as a JSON string.
  // Encoding it once more would send a string containing literal quote
  // characters (for example, "\"2027-10-10\"") and every field-specific
  // validator in the Go API would reject the value.
  return JSON.stringify(body);
}

export function useCreateOverride() {
  const locale = useLocale();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: CreateOverrideInput) =>
      bffFetch(`/api/bff/domains/${input.domainId}/overrides`, overrideRecordSchema, {
        method: "POST",
        body: buildCreateOverrideBody(input),
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
