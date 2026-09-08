import { describe, expect, it } from "vitest";
import { jsonBffError } from "@/lib/api/bff-response";

describe("jsonBffError", () => {
  it("returns an actionable CSRF message instead of an upstream error", async () => {
    const response = jsonBffError(
      new Request("http://example.test/api/bff/domains", {
        headers: { "Accept-Language": "th" },
      }),
      403,
      "CSRF_INVALID",
    );

    expect(response.status).toBe(403);
    await expect(response.json()).resolves.toMatchObject({
      error: {
        code: "CSRF_INVALID",
        message: "เซสชันสำหรับส่งข้อมูลหมดอายุ กรุณาเข้าสู่ระบบใหม่แล้วลองอีกครั้ง",
      },
    });
  });
});
