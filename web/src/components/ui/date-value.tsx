"use client";

import { useLocale } from "next-intl";

interface DateValueProps {
  /** ISO 8601 timestamp string from the backend (always UTC — ARCHITECTURE.md section 2.2.9). */
  value: string;
  className?: string;
  /**
   * For date-only fields (e.g. an expiration date stored as SQL `date`),
   * renders the UTC calendar date verbatim instead of converting to the
   * viewer's local timezone — a local-time conversion near a midnight
   * boundary could otherwise silently shift the displayed day, which the
   * Master Prompt explicitly forbids for expiration dates.
   */
  dateOnly?: boolean;
}

/**
 * Always shows an explicit timezone label so "what timezone is this?" is
 * never ambiguous, per the Master Prompt's date-correctness rule.
 */
export function DateValue({ value, className, dateOnly = false }: DateValueProps) {
  const locale = useLocale();
  const date = new Date(value);

  if (Number.isNaN(date.getTime())) {
    return <span className={className}>{value}</span>;
  }

  if (dateOnly) {
    const isoDate = date.toISOString().slice(0, 10);
    const formatted = new Intl.DateTimeFormat(locale, {
      year: "numeric",
      month: "short",
      day: "2-digit",
      timeZone: "UTC",
    }).format(date);
    return (
      <time dateTime={isoDate} className={className}>
        {formatted} (UTC)
      </time>
    );
  }

  const formatted = new Intl.DateTimeFormat(locale, {
    year: "numeric",
    month: "short",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    timeZoneName: "short",
  }).format(date);

  return (
    <time dateTime={date.toISOString()} className={className}>
      {formatted}
    </time>
  );
}
