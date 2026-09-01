"use client";

import type { z } from "zod";
import { parseApiEnvelope } from "@/lib/api/envelope";

export interface BffFetchOptions extends Omit<RequestInit, "headers"> {
  /**
   * The app's currently selected UI locale (from next-intl's useLocale()),
   * sent explicitly as Accept-Language so BFF/backend error copy matches
   * what the user picked — the browser's own Accept-Language header may
   * differ from the in-app language switch.
   */
  locale?: string;
  headers?: HeadersInit;
}

/**
 * Client-side fetch wrapper for same-origin /api/bff/** routes only. Every
 * BFF route responds with the same {data}/{error} envelope as the backend
 * (see src/lib/api/bff-response.ts), so this reuses parseApiEnvelope
 * unchanged — the client never needs a second error-parsing code path.
 */
export async function bffFetch<Schema extends z.ZodTypeAny>(
  input: string,
  dataSchema: Schema,
  options: BffFetchOptions = {},
): Promise<z.infer<Schema>> {
  const { locale, headers, ...rest } = options;
  const mergedHeaders = new Headers(headers);
  if (locale) {
    mergedHeaders.set("Accept-Language", locale);
  }
  if (rest.body && !mergedHeaders.has("Content-Type")) {
    mergedHeaders.set("Content-Type", "application/json");
  }

  const response = await fetch(input, {
    ...rest,
    headers: mergedHeaders,
    credentials: "same-origin",
  });

  return parseApiEnvelope(response, dataSchema);
}
