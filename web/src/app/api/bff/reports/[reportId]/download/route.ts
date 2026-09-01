import { NextResponse, type NextRequest } from "next/server";
import { cookies } from "next/headers";
import { isUuid } from "@/lib/api/bff-allowlist";
import { callBackend } from "@/lib/api/backend-client";
import { jsonBffError } from "@/lib/api/bff-response";
import { SESSION_COOKIE_NAME } from "@/lib/auth/cookies";

export const runtime = "nodejs";

/**
 * Dedicated (non-JSON) passthrough for GET /api/v1/reports/{id}/download —
 * the response is a raw file body with Content-Type/Content-Disposition/
 * X-Content-SHA256 headers, not the {data}/{error} envelope every other
 * BFF route expects, so it can't go through the generic catch-all. Streams
 * the backend response body straight through rather than buffering the
 * whole file in memory, per the Master Prompt's download-streaming rule.
 */
export async function GET(
  request: NextRequest,
  context: { params: Promise<{ reportId: string }> },
): Promise<NextResponse> {
  const { reportId } = await context.params;
  if (!isUuid(reportId)) {
    return jsonBffError(request, 404, "NOT_ALLOWED");
  }

  const cookieStore = await cookies();
  const cookie = cookieStore.get(SESSION_COOKIE_NAME);
  const sessionCookie = cookie ? `${SESSION_COOKIE_NAME}=${cookie.value}` : undefined;
  const acceptLanguage = request.headers.get("accept-language") ?? undefined;

  let backendResponse: Response;
  try {
    backendResponse = await callBackend({
      method: "GET",
      path: `/api/v1/reports/${reportId}/download`,
      acceptLanguage,
      sessionCookie,
    });
  } catch {
    return jsonBffError(request, 502, "UPSTREAM_UNAVAILABLE");
  }

  if (!backendResponse.ok || !backendResponse.body) {
    // Error responses from this endpoint still use the JSON envelope —
    // forward the body/status as-is so the client's normal envelope
    // parser can read the error.
    const text = await backendResponse.text();
    return new NextResponse(text, {
      status: backendResponse.status,
      headers: {
        "Content-Type": backendResponse.headers.get("Content-Type") ?? "application/json",
      },
    });
  }

  const headers = new Headers();
  const contentType = backendResponse.headers.get("Content-Type");
  const contentDisposition = backendResponse.headers.get("Content-Disposition");
  const sha256 = backendResponse.headers.get("X-Content-SHA256");
  if (contentType) headers.set("Content-Type", contentType);
  if (contentDisposition) headers.set("Content-Disposition", contentDisposition);
  if (sha256) headers.set("X-Content-SHA256", sha256);

  return new NextResponse(backendResponse.body, { status: 200, headers });
}
