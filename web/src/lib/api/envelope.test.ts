import { describe, expect, it } from "vitest";
import { z } from "zod";
import {
  ApiError,
  MalformedApiResponseError,
  parseApiEnvelope,
  selectErrorMessage,
} from "@/lib/api/envelope";

const dataSchema = z.object({ id: z.string() });

function jsonResponse(body: unknown, status: number): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

describe("parseApiEnvelope", () => {
  it("unwraps data on a 200 success envelope", async () => {
    const response = jsonResponse({ data: { id: "abc" } }, 200);
    await expect(parseApiEnvelope(response, dataSchema)).resolves.toEqual({ id: "abc" });
  });

  it("resolves a 204 No Content response as success without attempting to parse JSON", async () => {
    // Regression test: response.json() on an empty body throws a
    // SyntaxError — parseApiEnvelope must special-case 204 rather than
    // surfacing that as a MalformedApiResponseError (this broke
    // DELETE /google-drive/connection, which the backend answers with a
    // bare 204).
    const response = new Response(null, { status: 204 });
    await expect(parseApiEnvelope(response, z.undefined())).resolves.toBeUndefined();
  });

  it("throws ApiError with the full payload on a non-2xx error envelope", async () => {
    const errorPayload = {
      code: "DOMAIN_NOT_FOUND",
      message: "The domain was not found.",
      messages: { th: "ไม่พบโดเมน", en: "The domain was not found." },
      locale: "en" as const,
      request_id: "11111111-1111-1111-1111-111111111111",
      details: { domain_id: "abc" },
    };
    const response = jsonResponse({ error: errorPayload }, 404);

    await expect(parseApiEnvelope(response, dataSchema)).rejects.toMatchObject({
      code: "DOMAIN_NOT_FOUND",
      requestId: "11111111-1111-1111-1111-111111111111",
      locale: "en",
      status: 404,
      message: "The domain was not found.",
    });
  });

  it("rejects with ApiError instance (not a plain object) so callers can use instanceof", async () => {
    const response = jsonResponse(
      {
        error: {
          code: "UNAUTHORIZED",
          message: "Authentication is required.",
          messages: { th: "กรุณาเข้าสู่ระบบ", en: "Authentication is required." },
          locale: "en",
          request_id: "22222222-2222-2222-2222-222222222222",
        },
      },
      401,
    );

    try {
      await parseApiEnvelope(response, dataSchema);
      expect.unreachable("expected parseApiEnvelope to throw");
    } catch (error) {
      expect(error).toBeInstanceOf(ApiError);
    }
  });

  it("throws MalformedApiResponseError when the body is not JSON", async () => {
    const response = new Response("not json", { status: 200 });
    await expect(parseApiEnvelope(response, dataSchema)).rejects.toBeInstanceOf(
      MalformedApiResponseError,
    );
  });

  it("throws MalformedApiResponseError when a 200 response has neither data nor error", async () => {
    const response = jsonResponse({ unexpected: true }, 200);
    await expect(parseApiEnvelope(response, dataSchema)).rejects.toBeInstanceOf(
      MalformedApiResponseError,
    );
  });

  it("throws MalformedApiResponseError when data does not match the caller's schema", async () => {
    const response = jsonResponse({ data: { id: 123 } }, 200);
    await expect(parseApiEnvelope(response, dataSchema)).rejects.toBeInstanceOf(
      MalformedApiResponseError,
    );
  });

  it("throws MalformedApiResponseError when a non-2xx body does not match the error envelope shape", async () => {
    const response = jsonResponse({ oops: "no error key" }, 500);
    await expect(parseApiEnvelope(response, dataSchema)).rejects.toBeInstanceOf(
      MalformedApiResponseError,
    );
  });
});

describe("selectErrorMessage (localized backend error selection)", () => {
  const payload = {
    message: "The domain was not found.",
    messages: { th: "ไม่พบโดเมน", en: "The domain was not found." },
  };

  it("selects the Thai message when locale is th", () => {
    expect(selectErrorMessage(payload, "th")).toBe("ไม่พบโดเมน");
  });

  it("selects the English message when locale is en", () => {
    expect(selectErrorMessage(payload, "en")).toBe("The domain was not found.");
  });

  it("falls back to the request-time message if the locale bucket is somehow empty", () => {
    const partial = { message: "fallback text", messages: { th: "", en: "" } };
    expect(selectErrorMessage(partial, "th")).toBe("fallback text");
  });
});
