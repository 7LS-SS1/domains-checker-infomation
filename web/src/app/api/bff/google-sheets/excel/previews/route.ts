import { NextResponse, type NextRequest } from "next/server";
import { cookies } from "next/headers";
import { callBackend } from "@/lib/api/backend-client";
import { jsonBffError } from "@/lib/api/bff-response";
import { CSRF_COOKIE_NAME, SESSION_COOKIE_NAME } from "@/lib/auth/cookies";

export const runtime = "nodejs";

const MAX_IDEMPOTENCY_KEY_LENGTH = 200;

/**
 * Dedicated multipart passthrough for POST
 * /api/v1/google-sheets/excel/previews — the generic JSON catch-all
 * (src/app/api/bff/[...path]/route.ts) always sets Content-Type:
 * application/json and reads the body as text, which would corrupt a
 * multipart/form-data upload. This streams the incoming request body
 * straight through with its original Content-Type (including the
 * multipart boundary) so the uploaded .xlsx file's bytes are never
 * buffered/re-encoded in the BFF.
 */
export async function POST(request: NextRequest): Promise<NextResponse> {
  const cookieStore = await cookies();
  const sessionCookie = cookieStore.get(SESSION_COOKIE_NAME);
  if (!sessionCookie) {
    return jsonBffError(request, 401, "UNAUTHORIZED");
  }
  const csrfToken = cookieStore.get(CSRF_COOKIE_NAME)?.value;
  if (!csrfToken) {
    return jsonBffError(request, 403, "CSRF_INVALID");
  }

  const contentType = request.headers.get("Content-Type");
  if (!contentType || !contentType.startsWith("multipart/form-data")) {
    return jsonBffError(request, 422, "VALIDATION_FAILED");
  }

  const idempotencyKeyRaw = request.headers.get("Idempotency-Key")?.trim();
  const idempotencyKey =
    idempotencyKeyRaw &&
    idempotencyKeyRaw.length > 0 &&
    idempotencyKeyRaw.length <= MAX_IDEMPOTENCY_KEY_LENGTH
      ? idempotencyKeyRaw
      : undefined;

  // Buffered rather than streamed: the backend already bounds this upload
  // to EXCEL_IMPORT_MAX_BYTES (10MB default, 50MB hard cap —
  // internal/config/config.go), so holding one upload in memory here is
  // acceptable; streaming would additionally require Node fetch's
  // duplex:"half" option, which is not yet in this project's DOM lib types.
  const body = await request.arrayBuffer();

  let backendResponse: Response;
  try {
    backendResponse = await callBackend({
      method: "POST",
      path: "/api/v1/google-sheets/excel/previews",
      acceptLanguage: request.headers.get("accept-language") ?? undefined,
      sessionCookie: `${SESSION_COOKIE_NAME}=${sessionCookie.value}`,
      csrfToken,
      contentType,
      body,
      extraHeaders: idempotencyKey ? { "Idempotency-Key": idempotencyKey } : undefined,
    });
  } catch {
    return jsonBffError(request, 502, "UPSTREAM_UNAVAILABLE");
  }

  const responseBody = await backendResponse.text();
  return new NextResponse(responseBody.length > 0 ? responseBody : null, {
    status: backendResponse.status,
    headers: {
      "Content-Type": backendResponse.headers.get("Content-Type") ?? "application/json",
    },
  });
}
