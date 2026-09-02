"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useLocale } from "next-intl";
import { z } from "zod";
import { bffFetch } from "@/lib/api/client";
import { domainSchema } from "@/lib/domains/types";
import { monitoringRunSchema } from "@/lib/monitoring/types";

const manualCheckResponseSchema = z.object({
  run: monitoringRunSchema,
  created: z.boolean(),
});

/**
 * Idempotency-Key is generated fresh per call (crypto.randomUUID()) —
 * never reused across two distinct clicks, per the Master Prompt's
 * idempotency rule and web/API_GAPS.md GAP-08.
 */
export function useManualCheck() {
  const locale = useLocale();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (domainId: string) =>
      bffFetch(`/api/bff/domains/${domainId}/check`, manualCheckResponseSchema, {
        method: "POST",
        locale,
        headers: { "Idempotency-Key": crypto.randomUUID() },
      }),
    onSuccess: (_data, domainId) => {
      void queryClient.invalidateQueries({ queryKey: ["domain", domainId, "monitoring-runs"] });
    },
  });
}

export interface CreateDomainInput {
  domain: string;
  registrar_id?: string | null;
  business_priority: string;
  monitoring_enabled: boolean;
  expected_content_mode: string;
  expiration_at?: string | null;
  notes: string;
}

export function useCreateDomain() {
  const locale = useLocale();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: CreateDomainInput) =>
      bffFetch("/api/bff/domains", domainSchema, {
        method: "POST",
        body: JSON.stringify(input),
        locale,
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["domains"] });
    },
  });
}

export interface PatchDomainInput {
  domainId: string;
  version: number;
  domain?: string;
  registrar_id?: string;
  clear_registrar?: boolean;
  business_priority?: string;
  monitoring_enabled?: boolean;
  expected_content_mode?: string;
  expiration_at?: string;
  clear_expiration?: boolean;
  notes?: string;
  renewal_decision?: string;
  reason: string;
}

export function usePatchDomain() {
  const locale = useLocale();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ domainId, ...body }: PatchDomainInput) =>
      bffFetch(`/api/bff/domains/${domainId}`, domainSchema, {
        method: "PATCH",
        body: JSON.stringify(body),
        locale,
      }),
    onSuccess: (_data, variables) => {
      void queryClient.invalidateQueries({ queryKey: ["domains"] });
      void queryClient.invalidateQueries({ queryKey: ["domain", variables.domainId] });
    },
  });
}

export interface LifecycleActionInput {
  domainId: string;
  version: number;
  reason: string;
}

export function useArchiveDomain() {
  const locale = useLocale();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ domainId, version, reason }: LifecycleActionInput) =>
      bffFetch(`/api/bff/domains/${domainId}/archive`, domainSchema, {
        method: "POST",
        body: JSON.stringify({ version, reason }),
        locale,
      }),
    onSuccess: (_data, variables) => {
      void queryClient.invalidateQueries({ queryKey: ["domains"] });
      void queryClient.invalidateQueries({ queryKey: ["domain", variables.domainId] });
    },
  });
}

export function useRestoreDomain() {
  const locale = useLocale();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ domainId, version, reason }: LifecycleActionInput) =>
      bffFetch(`/api/bff/domains/${domainId}/restore`, domainSchema, {
        method: "POST",
        body: JSON.stringify({ version, reason }),
        locale,
      }),
    onSuccess: (_data, variables) => {
      void queryClient.invalidateQueries({ queryKey: ["domains"] });
      void queryClient.invalidateQueries({ queryKey: ["domain", variables.domainId] });
    },
  });
}
