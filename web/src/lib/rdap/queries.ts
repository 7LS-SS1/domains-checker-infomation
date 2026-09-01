"use client";

import { useMutation, useQuery, useQueryClient, type UseQueryResult } from "@tanstack/react-query";
import { useLocale } from "next-intl";
import { bffFetch } from "@/lib/api/client";
import { rdapResultSchema, type RdapResult } from "@/lib/rdap/types";

export function useRdap(domainId: string | undefined): UseQueryResult<RdapResult> {
  const locale = useLocale();
  return useQuery({
    queryKey: ["domain", domainId, "rdap"],
    queryFn: () => bffFetch(`/api/bff/domains/${domainId}/rdap`, rdapResultSchema, { locale }),
    enabled: Boolean(domainId),
    retry: false,
  });
}

export function useRdapCheck() {
  const locale = useLocale();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ domainId, force }: { domainId: string; force: boolean }) =>
      bffFetch(`/api/bff/domains/${domainId}/rdap-check`, rdapResultSchema, {
        method: "POST",
        body: JSON.stringify({ force }),
        locale,
      }),
    onSuccess: (_data, variables) => {
      void queryClient.invalidateQueries({ queryKey: ["domain", variables.domainId, "rdap"] });
    },
  });
}
