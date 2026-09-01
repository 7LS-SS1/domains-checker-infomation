import { NextResponse, type NextRequest } from "next/server";
import { z } from "zod";
import { callBackend } from "@/lib/api/backend-client";
import { ApiError, MalformedApiResponseError, parseApiEnvelope } from "@/lib/api/envelope";
import { jsonApiError, jsonBffError } from "@/lib/api/bff-response";
import { CSRF_COOKIE_NAME, SESSION_MAX_AGE_FALLBACK_SECONDS } from "@/lib/auth/cookies";

export const runtime = "nodejs";

const loginRequestSchema = z.object({
  email: z.string().min(1),
  password: z.string().min(1),
});

const loginResponseSchema = z.object({
  user: z.object({
    id: z.string(),
    email: z.string(),
    display_name: z.string(),
    locale: z.string(),
    roles: z.array(z.string()),
  }),
  csrf_token: z.string().min(1),
  expires_at: z.string(),
});

/**
 * Public login endpoint. Never requires CSRF (there is no session yet to
 * protect) and is the only BFF route that both (a) forwards the backend's
 * raw Set-Cookie for the session and (b) mints the BFF's own HttpOnly CSRF
 * cookie from the csrf_token field the Go API only ever returns in the JSON
 * body — see web/FRONTEND_ARCHITECTURE.md section 1.2.
 */
export async function POST(request: NextRequest) {
  let body: unknown;
  try {
    body = await request.json();
  } catch {
    return jsonBffError(request, 400, "INVALID_JSON");
  }

  const parsedBody = loginRequestSchema.safeParse(body);
  if (!parsedBody.success) {
    return jsonBffError(request, 422, "VALIDATION_FAILED");
  }

  const acceptLanguage = request.headers.get("accept-language") ?? undefined;

  let backendResponse: Response;
  try {
    backendResponse = await callBackend({
      method: "POST",
      path: "/api/v1/auth/login",
      acceptLanguage,
      contentType: "application/json",
      body: JSON.stringify(parsedBody.data),
    });
  } catch {
    return jsonBffError(request, 502, "UPSTREAM_UNAVAILABLE");
  }

  let data: z.infer<typeof loginResponseSchema>;
  try {
    data = await parseApiEnvelope(backendResponse, loginResponseSchema);
  } catch (cause) {
    if (cause instanceof ApiError) {
      return jsonApiError(cause);
    }
    if (cause instanceof MalformedApiResponseError) {
      return jsonBffError(request, 502, "UPSTREAM_INVALID_RESPONSE");
    }
    throw cause;
  }

  const response = NextResponse.json({
    data: {
      user: {
        id: data.user.id,
        email: data.user.email,
        displayName: data.user.display_name,
        locale: data.user.locale,
        roles: data.user.roles,
      },
      expiresAt: data.expires_at,
    },
  });

  // response.cookies.set() must run BEFORE the raw header appends below: it
  // rebuilds the Set-Cookie header from its own internal cookie jar, which
  // silently discards any Set-Cookie entries added beforehand via
  // response.headers.append() that it never parsed into that jar. Setting
  // the CSRF cookie first (the only cookie this route mints itself), then
  // appending the backend's raw session Set-Cookie afterward, is what keeps
  // both present on the final response — the reverse order was found to
  // drop the session cookie entirely while still returning 200, which
  // silently broke every login (caught by proxy.ts on the very next
  // protected navigation, which correctly saw no session cookie and bounced
  // back to /login).
  response.cookies.set(CSRF_COOKIE_NAME, data.csrf_token, {
    httpOnly: true,
    secure: process.env.NODE_ENV === "production",
    sameSite: "lax",
    path: "/",
    maxAge: SESSION_MAX_AGE_FALLBACK_SECONDS,
  });

  for (const cookie of backendResponse.headers.getSetCookie()) {
    response.headers.append("Set-Cookie", cookie);
  }

  return response;
}
