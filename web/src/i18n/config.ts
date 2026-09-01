export const locales = ["th", "en"] as const;
export type AppLocale = (typeof locales)[number];
export const defaultLocale: AppLocale = "th";

export function isAppLocale(value: string | null | undefined): value is AppLocale {
  return value === "th" || value === "en";
}
