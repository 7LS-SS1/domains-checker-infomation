const SENSITIVE_KEY_PATTERN = /token|password|secret|authorization|cookie|api[_-]?key/i;
const REDACTED_PLACEHOLDER = "[redacted]";

/**
 * Recursively strips any object key that looks sensitive before content is
 * shown in a raw JSON viewer or copied to the clipboard. Defense-in-depth:
 * none of the current backend evidence schemas are expected to carry
 * secrets, but a raw-evidence viewer is exactly the kind of surface where a
 * future backend field addition could leak one silently.
 */
export function redactSensitiveKeys(value: unknown): unknown {
  if (Array.isArray(value)) {
    return value.map(redactSensitiveKeys);
  }
  if (value !== null && typeof value === "object") {
    const result: Record<string, unknown> = {};
    for (const [key, entry] of Object.entries(value as Record<string, unknown>)) {
      result[key] = SENSITIVE_KEY_PATTERN.test(key)
        ? REDACTED_PLACEHOLDER
        : redactSensitiveKeys(entry);
    }
    return result;
  }
  return value;
}
