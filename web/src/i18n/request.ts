import { getRequestConfig } from "next-intl/server";
import { cookies } from "next/headers";
import { LOCALE_COOKIE_NAME } from "@/lib/auth/cookies";
import { defaultLocale, isAppLocale } from "@/i18n/config";

/**
 * Locale is resolved from a plain (non-HttpOnly) cookie rather than a URL
 * segment — the product has no /th/ or /en/ routing prefix, per
 * web/FRONTEND_ARCHITECTURE.md section 6. Every server render reads the
 * same cookie so SSR and client hydration never disagree on locale.
 */
export default getRequestConfig(async () => {
  const cookieStore = await cookies();
  const cookieLocale = cookieStore.get(LOCALE_COOKIE_NAME)?.value;
  const locale = isAppLocale(cookieLocale) ? cookieLocale : defaultLocale;

  const messages = (await import(`../messages/${locale}.json`)).default as Record<string, unknown>;

  return { locale, messages };
});
