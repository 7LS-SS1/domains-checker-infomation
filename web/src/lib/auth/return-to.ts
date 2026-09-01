/**
 * Validates a `returnTo` value against open-redirect abuse before it is
 * ever used to redirect the browser after login. Required per Master
 * Prompt Phase Frontend 2 task 8: "401 กลับ login พร้อม returnTo ที่
 * validate แล้ว". Must reject anything that could cause the browser to
 * leave this origin: absolute URLs, protocol-relative URLs ("//host"),
 * backslash tricks some user agents normalize to "//", embedded schemes,
 * and control characters.
 */
const DEFAULT_RETURN_TO = "/dashboard";

// Matches a string starting with exactly one "/" not followed by another
// "/" or a "\" — i.e. a same-origin absolute path, never a
// protocol-relative or backslash-confusable one.
const SAFE_LEADING_SLASH_PATTERN = /^\/(?!\/)(?!\\)/;

export function isValidReturnTo(value: string | null | undefined): value is string {
  if (typeof value !== "string") {
    return false;
  }
  if (value.length === 0 || value.length > 2048) {
    return false;
  }
  if (!SAFE_LEADING_SLASH_PATTERN.test(value)) {
    return false;
  }
  if (value.includes("\\")) {
    return false;
  }
  if (value.toLowerCase().includes("://")) {
    return false;
  }

  if (/[\x00-\x1f]/.test(value)) {
    return false;
  }
  return true;
}

export function sanitizeReturnTo(
  value: string | null | undefined,
  fallback: string = DEFAULT_RETURN_TO,
): string {
  return isValidReturnTo(value) ? value : fallback;
}
