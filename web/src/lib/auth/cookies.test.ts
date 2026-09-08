import { afterEach, describe, expect, it, vi } from "vitest";
import { shouldUseSecureSessionCookies } from "@/lib/auth/cookies";

describe("shouldUseSecureSessionCookies", () => {
  afterEach(() => {
    vi.unstubAllEnvs();
  });

  it("allows an explicit insecure policy for an HTTP preview deployment", () => {
    vi.stubEnv("NODE_ENV", "production");
    vi.stubEnv("SESSION_COOKIE_SECURE", "false");
    expect(shouldUseSecureSessionCookies()).toBe(false);
  });

  it("uses secure cookies when explicitly enabled", () => {
    vi.stubEnv("NODE_ENV", "development");
    vi.stubEnv("SESSION_COOKIE_SECURE", "true");
    expect(shouldUseSecureSessionCookies()).toBe(true);
  });

  it("defaults to the Node environment when no policy is configured", () => {
    vi.stubEnv("NODE_ENV", "production");
    vi.stubEnv("SESSION_COOKIE_SECURE", "");
    expect(shouldUseSecureSessionCookies()).toBe(true);
  });
});
