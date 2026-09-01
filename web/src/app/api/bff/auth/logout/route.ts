import { NextResponse, type NextRequest } from "next/server";
import { cookies } from "next/headers";
import { callBackend } from "@/lib/api/backend-client";
import { CSRF_COOKIE_NAME, SESSION_COOKIE_NAME } from "@/lib/auth/cookies";

export const runtime = "nodejs";

/**
 * Always clears both BFF cookies regardless of whether the upstream revoke
 * call succeeds — a network failure here must never leave the browser
 * holding cookies for a session the user believes is closed. See
 * web/FRONTEND_ARCHITECTURE.md section 1.4.
 */
export async function POST(request: NextRequest) {
  const cookieStore = await cookies();
  const sessionCookie = cookieStore.get(SESSION_COOKIE_NAME);
  const csrfCookie = cookieStore.get(CSRF_COOKIE_NAME);
  const acceptLanguage = request.headers.get("accept-language") ?? undefined;

  if (sessionCookie && csrfCookie) {
    try {
      await callBackend({
        method: "POST",
        path: "/api/v1/auth/logout",
        acceptLanguage,
        sessionCookie: `${SESSION_COOKIE_NAME}=${sessionCookie.value}`,
        csrfToken: csrfCookie.value,
      });
    } catch {
      // Deliberately swallowed — see docstring above.
    }
  }

  const response = NextResponse.json({ data: { loggedOut: true } });
  response.cookies.delete(SESSION_COOKIE_NAME);
  response.cookies.delete(CSRF_COOKIE_NAME);
  return response;
}
