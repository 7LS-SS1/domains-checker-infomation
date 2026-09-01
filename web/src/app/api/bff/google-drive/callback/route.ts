import { NextResponse, type NextRequest } from "next/server";
import { cookies } from "next/headers";
import { callBackend } from "@/lib/api/backend-client";
import { SESSION_COOKIE_NAME } from "@/lib/auth/cookies";

export const runtime = "nodejs";

/**
 * Dedicated handler for the Google OAuth redirect target. Unlike every
 * other BFF route, this one is never fetched by app JavaScript — Google
 * itself redirects the browser's top-level navigation here after consent,
 * carrying `state`/`code` as query params. It must run as a full-page
 * redirect (not a JSON XHR) both because that's how OAuth redirects work
 * and because GET /api/v1/google-drive/callback requires the admin's
 * session cookie to already be present (see web/API_GAPS.md GAP-07) — a
 * cross-origin popup straight to Google would lose that cookie context on
 * return, so this app never opens Google's consent screen in a popup.
 */
export async function GET(request: NextRequest): Promise<NextResponse> {
  const state = request.nextUrl.searchParams.get("state");
  const code = request.nextUrl.searchParams.get("code");
  const syncUrl = new URL("/sync", request.url);
  syncUrl.searchParams.set("tab", "drive");

  const cookieStore = await cookies();
  const sessionCookie = cookieStore.get(SESSION_COOKIE_NAME);
  if (!sessionCookie || !state || !code) {
    syncUrl.searchParams.set("driveConnected", "false");
    return NextResponse.redirect(syncUrl);
  }

  try {
    const backendResponse = await callBackend({
      method: "GET",
      path: `/api/v1/google-drive/callback?state=${encodeURIComponent(state)}&code=${encodeURIComponent(code)}`,
      acceptLanguage: request.headers.get("accept-language") ?? undefined,
      sessionCookie: `${SESSION_COOKIE_NAME}=${sessionCookie.value}`,
    });
    syncUrl.searchParams.set("driveConnected", backendResponse.ok ? "true" : "false");
  } catch {
    syncUrl.searchParams.set("driveConnected", "false");
  }

  return NextResponse.redirect(syncUrl);
}
