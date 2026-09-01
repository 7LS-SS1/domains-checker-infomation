import { NextResponse } from "next/server";
import type { ApiError, ApiErrorPayload, Locale } from "@/lib/api/envelope";

/**
 * Small catalog for errors the BFF itself raises (never reached the Go
 * API, or the Go API's response could not be trusted). Reuses the backend's
 * exact copy for codes that mirror internal/i18n/i18n.go so the same error
 * looks identical regardless of which layer detected it.
 */
const BFF_ERROR_MESSAGES: Record<string, { th: string; en: string }> = {
  INVALID_JSON: { th: "ข้อมูล JSON ไม่ถูกต้อง", en: "The JSON payload is invalid." },
  VALIDATION_FAILED: { th: "ข้อมูลไม่ผ่านการตรวจสอบ", en: "The supplied data is invalid." },
  UPSTREAM_UNAVAILABLE: {
    th: "ไม่สามารถเชื่อมต่อกับระบบหลังบ้านได้ในขณะนี้",
    en: "Could not reach the backend service right now.",
  },
  UPSTREAM_INVALID_RESPONSE: {
    th: "ระบบหลังบ้านส่งข้อมูลที่ไม่ถูกต้องกลับมา",
    en: "The backend service returned an unexpected response.",
  },
  NOT_ALLOWED: {
    th: "ไม่อนุญาตให้เรียกใช้เส้นทางนี้",
    en: "This route is not permitted.",
  },
};

function resolveLocale(request: Request): Locale {
  const header = request.headers.get("accept-language")?.toLowerCase() ?? "";
  return header.startsWith("en") ? "en" : "th";
}

export function apiErrorPayload(error: ApiError): ApiErrorPayload {
  return {
    code: error.code,
    message: error.message,
    messages: error.messages,
    locale: error.locale,
    request_id: error.requestId,
    ...(error.details !== undefined ? { details: error.details } : {}),
  };
}

/** Re-emits an ApiError thrown by parseApiEnvelope, unchanged, as BFF JSON. */
export function jsonApiError(error: ApiError): NextResponse {
  return NextResponse.json({ error: apiErrorPayload(error) }, { status: error.status });
}

/** Builds a BFF-synthesized error response for failures that never reached (or never trusted) the backend. */
export function jsonBffError(request: Request, status: number, code: string): NextResponse {
  const locale = resolveLocale(request);
  const messages = BFF_ERROR_MESSAGES[code] ?? BFF_ERROR_MESSAGES.UPSTREAM_UNAVAILABLE!;
  const payload: ApiErrorPayload = {
    code,
    message: messages[locale],
    messages,
    locale,
    request_id: crypto.randomUUID(),
  };
  return NextResponse.json({ error: payload }, { status });
}
