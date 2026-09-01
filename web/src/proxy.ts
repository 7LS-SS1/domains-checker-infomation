import { NextResponse, type NextRequest } from "next/server";
import { callBackend } from "@/lib/api/backend-client";
import { SESSION_COOKIE_NAME } from "@/lib/auth/cookies";

/**
 * Extend this list as more authenticated pages are built in later phases.
 * Every prefix here gets a real session-validity check against the
 * backend before the page is ever rendered.
 */
const PROTECTED_PREFIXES = [
  "/dashboard",
  "/domains",
  "/incidents",
  "/finance",
  "/recommendations",
  "/reports",
  "/sync",
  "/probes",
  "/settings",
];

function isProtectedPath(pathname: string): boolean {
  return PROTECTED_PREFIXES.some(
    (prefix) => pathname === prefix || pathname.startsWith(`${prefix}/`),
  );
}

function redirectToLogin(request: NextRequest, pathname: string, search: string): NextResponse {
  const loginUrl = new URL("/login", request.url);
  loginUrl.searchParams.set("returnTo", `${pathname}${search}`);
  return NextResponse.redirect(loginUrl);
}

/**
 * Route guard for protected pages (Next.js 16 "Proxy" convention, formerly
 * "Middleware" — see https://nextjs.org/docs/messages/middleware-to-proxy).
 * Validates the session cookie against the real backend (not just cookie
 * presence) so a revoked/expired session is caught before any authenticated
 * UI renders, and always redirects to /login with a returnTo built from the
 * actual attempted path — see Master Prompt Phase Frontend 2 task 8 and
 * web/FRONTEND_ARCHITECTURE.md section 1.3. On a network failure talking to
 * the backend, the request is allowed through so the destination layout's
 * own getSession() call can classify it precisely as "system unavailable"
 * rather than this proxy guessing "logged out" from a transient blip.
 */
export async function proxy(request: NextRequest): Promise<NextResponse> {
  const { pathname, search } = request.nextUrl;
  if (!isProtectedPath(pathname)) {
    return NextResponse.next();
  }

  const sessionCookie = request.cookies.get(SESSION_COOKIE_NAME);
  if (!sessionCookie) {
    return redirectToLogin(request, pathname, search);
  }

  let response: Response;
  try {
    response = await callBackend({
      method: "GET",
      path: "/api/v1/auth/me",
      sessionCookie: `${SESSION_COOKIE_NAME}=${sessionCookie.value}`,
      acceptLanguage: request.headers.get("accept-language") ?? undefined,
    });
  } catch {
    return NextResponse.next();
  }

  if (response.status === 401) {
    return redirectToLogin(request, pathname, search);
  }

  return NextResponse.next();
}

export const config = {
  matcher: [
    "/dashboard/:path*",
    "/domains/:path*",
    "/incidents/:path*",
    "/finance/:path*",
    "/recommendations/:path*",
    "/reports/:path*",
    "/sync/:path*",
    "/probes/:path*",
    "/settings/:path*",
  ],
};
