import { describe, expect, it } from "vitest";
import { redactSensitiveKeys } from "@/lib/utils/redact";

describe("redactSensitiveKeys", () => {
  it("redacts top-level sensitive keys", () => {
    const result = redactSensitiveKeys({
      access_token: "secret-value",
      password: "hunter2",
      api_key: "abc",
      normal_field: "safe",
    }) as Record<string, unknown>;

    expect(result.access_token).toBe("[redacted]");
    expect(result.password).toBe("[redacted]");
    expect(result.api_key).toBe("[redacted]");
    expect(result.normal_field).toBe("safe");
  });

  it("redacts nested sensitive keys inside objects and arrays", () => {
    const result = redactSensitiveKeys({
      results: [
        { id: "1", authorization: "Bearer xyz" },
        { id: "2", cookie: "session=abc" },
      ],
    }) as { results: Array<Record<string, unknown>> };

    expect(result.results[0]?.authorization).toBe("[redacted]");
    expect(result.results[1]?.cookie).toBe("[redacted]");
    expect(result.results[0]?.id).toBe("1");
  });

  it("leaves non-sensitive nested data untouched", () => {
    const input = { domain: "example.com", checks: { dns: "OK", http: "OK" } };
    expect(redactSensitiveKeys(input)).toEqual(input);
  });

  it("handles primitives and null without crashing", () => {
    expect(redactSensitiveKeys("plain string")).toBe("plain string");
    expect(redactSensitiveKeys(42)).toBe(42);
    expect(redactSensitiveKeys(null)).toBeNull();
  });
});
