"use server";

import { cookies } from "next/headers";
import { isAppLocale } from "@/i18n/config";
import { LOCALE_COOKIE_NAME } from "@/lib/auth/cookies";

const ONE_YEAR_SECONDS = 60 * 60 * 24 * 365;

/**
 * Persists the chosen UI locale in a plain (non-HttpOnly) cookie — it holds
 * no secret, so client JS reading it back is fine, but writing it happens
 * server-side via this action rather than document.cookie for a single
 * source of truth with SSR (src/i18n/request.ts reads the same cookie).
 */
export async function setLocaleAction(locale: string): Promise<void> {
  if (!isAppLocale(locale)) {
    return;
  }
  const cookieStore = await cookies();
  cookieStore.set(LOCALE_COOKIE_NAME, locale, {
    path: "/",
    maxAge: ONE_YEAR_SECONDS,
    sameSite: "lax",
  });
}
