import { describe, expect, it } from "vitest";
import { formatDecimalString } from "@/lib/utils/decimal-format";

describe("formatDecimalString (exact-money rendering, no Number()/parseFloat)", () => {
  it("groups a whole-number integer part by thousands", () => {
    expect(formatDecimalString("1234567")).toBe("1,234,567");
  });

  it("preserves the fractional part exactly, however many digits", () => {
    expect(formatDecimalString("1234567.999999")).toBe("1,234,567.999999");
    expect(formatDecimalString("0.10")).toBe("0.10");
  });

  it("preserves a huge value beyond Number.MAX_SAFE_INTEGER without precision loss", () => {
    const huge = "90071992547409999999.123456";
    expect(formatDecimalString(huge)).toBe("90,071,992,547,409,999,999.123456");
  });

  it("handles negative amounts", () => {
    expect(formatDecimalString("-1234.50")).toBe("-1,234.50");
  });

  it("handles small integers without inserting separators", () => {
    expect(formatDecimalString("42")).toBe("42");
    expect(formatDecimalString("999")).toBe("999");
  });

  it("handles a zero value", () => {
    expect(formatDecimalString("0")).toBe("0");
    expect(formatDecimalString("0.000000")).toBe("0.000000");
  });

  it("never produces NaN/Infinity artifacts for a large decimal-scale string", () => {
    const value = formatDecimalString("1000000000000000000000.5");
    expect(value).not.toContain("NaN");
    expect(value).not.toContain("Infinity");
    expect(value).toBe("1,000,000,000,000,000,000,000.5");
  });
});
