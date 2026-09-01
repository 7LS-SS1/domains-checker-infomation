import { NextResponse, type NextRequest } from "next/server";
import { cookies } from "next/headers";
import { buildAllowlistedQuery, resolveRoute, type HttpMethod } from "@/lib/api/bff-allowlist";
import { callBackend } from "@/lib/api/backend-client";
import { jsonBffError } from "@/lib/api/bff-response";
import { CSRF_COOKIE_NAME, SESSION_COOKIE_NAME } from "@/lib/auth/cookies";

export const runtime = "nodejs";

const MAX_IDEMPOTENCY_KEY_LENGTH = 200;

/**
 * Generic passthrough for every route in BFF_ROUTES
 * (src/lib/api/bff-allowlist.ts). A request whose method+path do not
 * resolve to a table entry — including any path-parameter segment that
 * isn't a valid UUID — never reaches callBackend at all. Mutating methods
 * attach X-CSRF-Token from the BFF's own HttpOnly cookie automatically;
 * the browser never supplies or sees that header itself.
 */
async function handle(
  request: NextRequest,
  context: { params: Promise<{ path: string[] }> },
  method: HttpMethod,
): Promise<NextResponse> {
  const { path } = await context.params;
  const resolved = resolveRoute(method, path ?? []);
  if (!resolved) {
    return jsonBffError(request, 404, "NOT_ALLOWED");
  }
  const { methodConfig } = resolved;

  const cookieStore = await cookies();
  let sessionCookie: string | undefined;
  if (methodConfig.requiresAuth) {
    const cookie = cookieStore.get(SESSION_COOKIE_NAME);
    if (cookie) {
      sessionCookie = `${SESSION_COOKIE_NAME}=${cookie.value}`;
    }
  }

  let csrfToken: string | undefined;
  if (methodConfig.requiresCsrf) {
    csrfToken = cookieStore.get(CSRF_COOKIE_NAME)?.value;
    if (!csrfToken) {
      return jsonBffError(request, 403, "CSRF_INVALID");
    }
  }

  let idempotencyKey: string | undefined;
  if (methodConfig.forwardsIdempotencyKey) {
    const raw = request.headers.get("Idempotency-Key")?.trim();
    if (raw && raw.length > 0 && raw.length <= MAX_IDEMPOTENCY_KEY_LENGTH) {
      idempotencyKey = raw;
    }
  }

  let body: string | undefined;
  if (methodConfig.forwardsBody) {
    body = await request.text();
  }

  const query = buildAllowlistedQuery(methodConfig, request.nextUrl.searchParams);
  const acceptLanguage = request.headers.get("accept-language") ?? undefined;

  const headers: Record<string, string | undefined> = {};
  if (idempotencyKey) {
    headers["Idempotency-Key"] = idempotencyKey;
  }

  let backendResponse: Response;
  try {
    backendResponse = await callBackend({
      method,
      path: `${resolved.backendPath}${query}`,
      acceptLanguage,
      sessionCookie,
      csrfToken,
      contentType: body !== undefined ? "application/json" : undefined,
      body,
      extraHeaders: headers,
    });
  } catch {
    return jsonBffError(request, 502, "UPSTREAM_UNAVAILABLE");
  }

  const responseBody = await backendResponse.text();
  const responseHeaders: Record<string, string> = {
    "Content-Type": backendResponse.headers.get("Content-Type") ?? "application/json",
  };
  const location = backendResponse.headers.get("Location");
  if (location) {
    responseHeaders["Location"] = location;
  }

  return new NextResponse(responseBody.length > 0 ? responseBody : null, {
    status: backendResponse.status,
    headers: responseHeaders,
  });
}

export async function GET(request: NextRequest, context: { params: Promise<{ path: string[] }> }) {
  return handle(request, context, "GET");
}

export async function POST(request: NextRequest, context: { params: Promise<{ path: string[] }> }) {
  return handle(request, context, "POST");
}

export async function PATCH(
  request: NextRequest,
  context: { params: Promise<{ path: string[] }> },
) {
  return handle(request, context, "PATCH");
}

export async function DELETE(
  request: NextRequest,
  context: { params: Promise<{ path: string[] }> },
) {
  return handle(request, context, "DELETE");
}

export async function PUT(request: NextRequest, context: { params: Promise<{ path: string[] }> }) {
  return handle(request, context, "PUT");
}
