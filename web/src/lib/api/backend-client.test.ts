import { describe, expect, it } from "vitest";
import { buildBackendRequestHeaders, isMutatingMethod } from "@/lib/api/backend-client";

describe("isMutatingMethod", () => {
  it("classifies POST/PUT/PATCH/DELETE as mutating", () => {
    expect(isMutatingMethod("POST")).toBe(true);
    expect(isMutatingMethod("PUT")).toBe(true);
    expect(isMutatingMethod("PATCH")).toBe(true);
    expect(isMutatingMethod("DELETE")).toBe(true);
    expect(isMutatingMethod("post")).toBe(true);
  });

  it("classifies GET/HEAD/OPTIONS as non-mutating", () => {
    expect(isMutatingMethod("GET")).toBe(false);
    expect(isMutatingMethod("HEAD")).toBe(false);
    expect(isMutatingMethod("OPTIONS")).toBe(false);
    expect(isMutatingMethod("get")).toBe(false);
  });
});

describe("buildBackendRequestHeaders (CSRF attachment rule)", () => {
  it("attaches X-CSRF-Token for a mutating request when a token is supplied", () => {
    const headers = buildBackendRequestHeaders({
      method: "POST",
      csrfToken: "csrf-secret-value",
    });
    expect(headers.get("X-CSRF-Token")).toBe("csrf-secret-value");
  });

  it("never attaches X-CSRF-Token on a GET request, even if a token is supplied", () => {
    const headers = buildBackendRequestHeaders({
      method: "GET",
      csrfToken: "csrf-secret-value",
    });
    expect(headers.has("X-CSRF-Token")).toBe(false);
  });

  it("attaches X-CSRF-Token for PATCH and DELETE as well as POST", () => {
    for (const method of ["PATCH", "PUT", "DELETE"]) {
      const headers = buildBackendRequestHeaders({ method, csrfToken: "token" });
      expect(headers.get("X-CSRF-Token")).toBe("token");
    }
  });

  it("omits X-CSRF-Token on a mutating request when no token is supplied", () => {
    const headers = buildBackendRequestHeaders({ method: "POST" });
    expect(headers.has("X-CSRF-Token")).toBe(false);
  });

  it("forwards the session cookie and Accept-Language when provided", () => {
    const headers = buildBackendRequestHeaders({
      method: "GET",
      sessionCookie: "domainintel_session=abc123",
      acceptLanguage: "th",
    });
    expect(headers.get("Cookie")).toBe("domainintel_session=abc123");
    expect(headers.get("Accept-Language")).toBe("th");
  });

  it("sets Content-Type only when explicitly provided", () => {
    const withType = buildBackendRequestHeaders({
      method: "POST",
      contentType: "application/json",
    });
    expect(withType.get("Content-Type")).toBe("application/json");

    const withoutType = buildBackendRequestHeaders({ method: "GET" });
    expect(withoutType.has("Content-Type")).toBe(false);
  });

  it("forwards an explicit extraHeaders entry such as Idempotency-Key", () => {
    const headers = buildBackendRequestHeaders({
      method: "POST",
      extraHeaders: { "Idempotency-Key": "client-generated-key-123" },
    });
    expect(headers.get("Idempotency-Key")).toBe("client-generated-key-123");
  });

  it("omits an extraHeaders entry whose value is undefined", () => {
    const headers = buildBackendRequestHeaders({
      method: "POST",
      extraHeaders: { "Idempotency-Key": undefined },
    });
    expect(headers.has("Idempotency-Key")).toBe(false);
  });
});
