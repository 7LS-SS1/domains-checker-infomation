import { describe, expect, it } from "vitest";
import {
  capabilitiesFor,
  hasAnyCapability,
  hasCapability,
  toKnownRoles,
} from "@/lib/auth/capability";

describe("hasCapability (RBAC capability matrix)", () => {
  it("grants ADMIN every capability a STAFF or VIEWER has, and more", () => {
    expect(hasCapability(["ADMIN"], "manageOverrides")).toBe(true);
    expect(hasCapability(["ADMIN"], "applyImport")).toBe(true);
    expect(hasCapability(["ADMIN"], "manageProbes")).toBe(true);
  });

  it("denies STAFF the ADMIN-only capabilities per web/API_GAPS.md GAP-05", () => {
    expect(hasCapability(["STAFF"], "manageOverrides")).toBe(false);
    expect(hasCapability(["STAFF"], "applyImport")).toBe(false);
    expect(hasCapability(["STAFF"], "manageExchangeRates")).toBe(false);
    expect(hasCapability(["STAFF"], "manageDriveConnection")).toBe(false);
    expect(hasCapability(["STAFF"], "manageProbes")).toBe(false);
  });

  it("grants STAFF the shared ADMIN+STAFF capabilities", () => {
    expect(hasCapability(["STAFF"], "editDomains")).toBe(true);
    expect(hasCapability(["STAFF"], "previewImport")).toBe(true);
    expect(hasCapability(["STAFF"], "generateRecommendation")).toBe(true);
    expect(hasCapability(["STAFF"], "createReport")).toBe(true);
  });

  it("restricts VIEWER to read-only capabilities", () => {
    expect(hasCapability(["VIEWER"], "viewDomains")).toBe(true);
    expect(hasCapability(["VIEWER"], "viewRecommendations")).toBe(true);
    expect(hasCapability(["VIEWER"], "viewReports")).toBe(true);
    expect(hasCapability(["VIEWER"], "viewProbes")).toBe(true);
    expect(hasCapability(["VIEWER"], "viewIncidents")).toBe(true);

    expect(hasCapability(["VIEWER"], "editDomains")).toBe(false);
    expect(hasCapability(["VIEWER"], "previewImport")).toBe(false);
    expect(hasCapability(["VIEWER"], "createReport")).toBe(false);
    expect(hasCapability(["VIEWER"], "manageOverrides")).toBe(false);
  });

  it("returns true if any of the user's roles grants the capability", () => {
    expect(hasCapability(["VIEWER", "STAFF"], "editDomains")).toBe(true);
  });

  it("returns false for a user with no roles", () => {
    expect(hasCapability([], "viewDomains")).toBe(false);
  });
});

describe("hasAnyCapability", () => {
  it("returns true if at least one listed capability is granted", () => {
    expect(hasAnyCapability(["STAFF"], ["manageOverrides", "editDomains"])).toBe(true);
  });

  it("returns false if none of the listed capabilities are granted", () => {
    expect(hasAnyCapability(["VIEWER"], ["manageOverrides", "editDomains"])).toBe(false);
  });
});

describe("capabilitiesFor", () => {
  it("is a strict superset relationship: ADMIN's set contains STAFF's set", () => {
    const adminCapabilities = capabilitiesFor(["ADMIN"]);
    const staffCapabilities = capabilitiesFor(["STAFF"]);
    for (const capability of staffCapabilities) {
      expect(adminCapabilities.has(capability)).toBe(true);
    }
    expect(adminCapabilities.size).toBeGreaterThan(staffCapabilities.size);
  });
});

describe("toKnownRoles", () => {
  it("filters out role strings the frontend does not recognize", () => {
    expect(toKnownRoles(["ADMIN", "SYSTEM", "STAFF", "unknown"])).toEqual(["ADMIN", "STAFF"]);
  });

  it("returns an empty array for an all-unknown input", () => {
    expect(toKnownRoles(["SYSTEM"])).toEqual([]);
  });
});
