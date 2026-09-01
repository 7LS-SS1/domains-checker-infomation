import { describe, expect, it } from "vitest";
import { isValidReturnTo, sanitizeReturnTo } from "@/lib/auth/return-to";

describe("isValidReturnTo (open-redirect prevention)", () => {
  it("accepts a plain same-origin absolute path", () => {
    expect(isValidReturnTo("/dashboard")).toBe(true);
    expect(isValidReturnTo("/")).toBe(true);
    expect(isValidReturnTo("/domains?tab=active")).toBe(true);
    expect(isValidReturnTo("/domains/123e4567-e89b-12d3-a456-426614174000")).toBe(true);
  });

  it("rejects protocol-relative URLs (//host)", () => {
    expect(isValidReturnTo("//evil.example.com")).toBe(false);
    expect(isValidReturnTo("//evil.example.com/dashboard")).toBe(false);
  });

  it("rejects backslash tricks some user agents normalize to protocol-relative", () => {
    expect(isValidReturnTo("/\\evil.example.com")).toBe(false);
    expect(isValidReturnTo("\\\\evil.example.com")).toBe(false);
    expect(isValidReturnTo("/foo\\bar")).toBe(false);
  });

  it("rejects absolute URLs with any scheme", () => {
    expect(isValidReturnTo("https://evil.example.com")).toBe(false);
    expect(isValidReturnTo("http://evil.example.com")).toBe(false);
    expect(isValidReturnTo("javascript:alert(1)")).toBe(false);
    expect(isValidReturnTo("/redirect?next=https://evil.example.com")).toBe(false);
  });

  it("rejects empty, non-string, or missing values", () => {
    expect(isValidReturnTo("")).toBe(false);
    expect(isValidReturnTo(null)).toBe(false);
    expect(isValidReturnTo(undefined)).toBe(false);
  });

  it("rejects values that do not start with a single leading slash", () => {
    expect(isValidReturnTo("dashboard")).toBe(false);
    expect(isValidReturnTo("evil.example.com/dashboard")).toBe(false);
  });

  it("rejects embedded control characters", () => {
    expect(isValidReturnTo("/dashboard\n")).toBe(false);
    expect(isValidReturnTo("/dash\tboard")).toBe(false);
  });

  it("rejects excessively long values", () => {
    expect(isValidReturnTo("/" + "a".repeat(3000))).toBe(false);
  });
});

describe("sanitizeReturnTo", () => {
  it("returns the value unchanged when valid", () => {
    expect(sanitizeReturnTo("/dashboard")).toBe("/dashboard");
  });

  it("falls back to the default when invalid", () => {
    expect(sanitizeReturnTo("//evil.example.com")).toBe("/dashboard");
    expect(sanitizeReturnTo(null)).toBe("/dashboard");
  });

  it("falls back to a caller-supplied default when invalid", () => {
    expect(sanitizeReturnTo("https://evil.example.com", "/login")).toBe("/login");
  });
});
