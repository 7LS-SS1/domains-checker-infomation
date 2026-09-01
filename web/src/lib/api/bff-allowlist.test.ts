import { describe, expect, it } from "vitest";
import { buildAllowlistedQuery, isUuid, resolveRoute } from "@/lib/api/bff-allowlist";

const VALID_DOMAIN_ID = "11111111-1111-4111-8111-111111111111";
const VALID_OVERRIDE_ID = "22222222-2222-4222-8222-222222222222";

describe("isUuid", () => {
  it("accepts a well-formed v4-shaped UUID", () => {
    expect(isUuid(VALID_DOMAIN_ID)).toBe(true);
  });

  it("rejects non-UUID strings, including path-traversal attempts", () => {
    expect(isUuid("not-a-uuid")).toBe(false);
    expect(isUuid("../../etc/passwd")).toBe(false);
    expect(isUuid("")).toBe(false);
    expect(isUuid("11111111-1111-1111-1111-111111111111-extra")).toBe(false);
  });
});

describe("resolveRoute (BFF open-proxy protection)", () => {
  it("resolves a static GET path to its backend mapping", () => {
    const resolved = resolveRoute("GET", ["auth", "me"]);
    expect(resolved?.backendPath).toBe("/api/v1/auth/me");
    expect(resolved?.methodConfig.requiresAuth).toBe(true);
    expect(resolved?.methodConfig.requiresCsrf).toBe(false);
  });

  it("is case-insensitive on the HTTP method", () => {
    expect(resolveRoute("get", ["meta", "locales"])).not.toBeNull();
  });

  it("rejects a disallowed method on an otherwise-known path", () => {
    expect(resolveRoute("POST", ["auth", "me"])).toBeNull();
    expect(resolveRoute("DELETE", ["meta", "locales"])).toBeNull();
  });

  it("rejects any path that is not explicitly listed", () => {
    expect(resolveRoute("GET", ["something-unknown"])).toBeNull();
    expect(resolveRoute("GET", ["auth", "login"])).toBeNull();
  });

  it("rejects path traversal / segment-count mismatches — exact match only", () => {
    expect(resolveRoute("GET", ["auth", "me", "extra"])).toBeNull();
    expect(resolveRoute("GET", ["auth"])).toBeNull();
  });

  it("rejects an empty path", () => {
    expect(resolveRoute("GET", [])).toBeNull();
  });

  it("resolves a parametrized path when the param is a valid UUID", () => {
    const resolved = resolveRoute("GET", ["domains", VALID_DOMAIN_ID]);
    expect(resolved?.backendPath).toBe(`/api/v1/domains/${VALID_DOMAIN_ID}`);
  });

  it("rejects a parametrized path when the captured segment is not a UUID", () => {
    expect(resolveRoute("GET", ["domains", "not-a-uuid"])).toBeNull();
    expect(resolveRoute("GET", ["domains", "..", "..", "etc"])).toBeNull();
  });

  it("resolves a nested parametrized path with two captured UUIDs", () => {
    const resolved = resolveRoute("DELETE", [
      "domains",
      VALID_DOMAIN_ID,
      "overrides",
      VALID_OVERRIDE_ID,
    ]);
    expect(resolved?.backendPath).toBe(
      `/api/v1/domains/${VALID_DOMAIN_ID}/overrides/${VALID_OVERRIDE_ID}`,
    );
  });

  it("marks every mutating method as requiring CSRF and every GET as not requiring it", () => {
    const create = resolveRoute("POST", ["domains"]);
    expect(create?.methodConfig.requiresCsrf).toBe(true);
    const list = resolveRoute("GET", ["domains"]);
    expect(list?.methodConfig.requiresCsrf).toBe(false);
    const patch = resolveRoute("PATCH", ["domains", VALID_DOMAIN_ID]);
    expect(patch?.methodConfig.requiresCsrf).toBe(true);
    const archive = resolveRoute("POST", ["domains", VALID_DOMAIN_ID, "archive"]);
    expect(archive?.methodConfig.requiresCsrf).toBe(true);
  });

  it("only the manual-check route forwards Idempotency-Key", () => {
    const check = resolveRoute("POST", ["domains", VALID_DOMAIN_ID, "check"]);
    expect(check?.methodConfig.forwardsIdempotencyKey).toBe(true);
    const archive = resolveRoute("POST", ["domains", VALID_DOMAIN_ID, "archive"]);
    expect(archive?.methodConfig.forwardsIdempotencyKey).toBeFalsy();
  });
});

describe("buildAllowlistedQuery (query-string allowlist)", () => {
  const incidents = resolveRoute("GET", ["incidents"])!;

  it("forwards only the params listed for the entry", () => {
    const params = new URLSearchParams({ status: "open", page: "2", page_size: "10" });
    expect(buildAllowlistedQuery(incidents.methodConfig, params)).toBe(
      "?status=open&page=2&page_size=10",
    );
  });

  it("drops params not in the entry's allowlist", () => {
    const params = new URLSearchParams({ status: "open", evil: "1", inject_sql: "'; DROP" });
    const query = buildAllowlistedQuery(incidents.methodConfig, params);
    expect(query).toBe("?status=open");
    expect(query).not.toContain("evil");
    expect(query).not.toContain("DROP");
  });

  it("returns an empty string when no allowed params are present", () => {
    expect(buildAllowlistedQuery(incidents.methodConfig, new URLSearchParams())).toBe("");
  });

  it("returns an empty string for a method config with no queryParams declared", () => {
    const me = resolveRoute("GET", ["auth", "me"])!;
    const params = new URLSearchParams({ status: "open" });
    expect(buildAllowlistedQuery(me.methodConfig, params)).toBe("");
  });
});
