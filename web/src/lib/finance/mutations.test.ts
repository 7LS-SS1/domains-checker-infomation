import { describe, expect, it } from "vitest";
import { buildCreateOverrideBody } from "@/lib/finance/mutations";

describe("buildCreateOverrideBody", () => {
  it("encodes override_value exactly once as a JSON string", () => {
    const body = buildCreateOverrideBody({
      domainId: "a466a919-2d64-4366-ac61-be412dbefb9c",
      field_name: "expiration_date",
      override_value: "2027-10-10",
      expires_at: "2027-10-10T00:00:00.000Z",
      reason: "แก้วันหมดอายุ",
    });

    expect(JSON.parse(body)).toEqual({
      field_name: "expiration_date",
      override_value: "2027-10-10",
      expires_at: "2027-10-10T00:00:00.000Z",
      reason: "แก้วันหมดอายุ",
    });
    expect(body).not.toContain('\\\"2027-10-10\\\"');
  });

  it("does not forward the route-only domainId in the request body", () => {
    const body = buildCreateOverrideBody({
      domainId: "a466a919-2d64-4366-ac61-be412dbefb9c",
      field_name: "business_priority",
      override_value: "high",
      reason: "ปรับลำดับความสำคัญ",
    });

    expect(JSON.parse(body)).not.toHaveProperty("domainId");
  });
});
