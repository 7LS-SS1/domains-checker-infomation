"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useLocale } from "next-intl";
import { z } from "zod";
import { bffFetch } from "@/lib/api/client";
import { probeRegistrationTokenSchema } from "@/lib/probes/types";

const messageResponseSchema = z.object({ message: z.string() });

export interface CreateRegistrationTokenInput {
  name: string;
  region_code: string;
  country_code: string;
  network_name?: string;
  ttl_seconds: number;
}

const createRegistrationTokenResponseSchema = z.object({
  registration_token: probeRegistrationTokenSchema,
});

export function useCreateRegistrationToken() {
  const locale = useLocale();
  return useMutation({
    mutationFn: (input: CreateRegistrationTokenInput) =>
      bffFetch("/api/bff/probes/registration-tokens", createRegistrationTokenResponseSchema, {
        method: "POST",
        body: JSON.stringify(input),
        locale,
      }).then((result) => result.registration_token),
  });
}

export function useRevokeProbe() {
  const locale = useLocale();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ probeId, reason }: { probeId: string; reason: string }) =>
      bffFetch(`/api/bff/probes/${probeId}/revoke`, messageResponseSchema, {
        method: "POST",
        body: JSON.stringify({ reason }),
        locale,
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["probes"] });
    },
  });
}
