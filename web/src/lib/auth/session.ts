import "server-only";
import { cookies, headers } from "next/headers";
import { z } from "zod";
import { callBackend } from "@/lib/api/backend-client";
import { ApiError, parseApiEnvelope } from "@/lib/api/envelope";
import { SESSION_COOKIE_NAME } from "@/lib/auth/cookies";
import type { AuthUser } from "@/lib/auth/types";

const meResponseSchema = z.object({
  id: z.string(),
  email: z.string(),
  display_name: z.string(),
  locale: z.string(),
  roles: z.array(z.string()),
});

export type SessionUser = AuthUser;

/**
 * Thrown when the auth/me call could not be completed at all (network
 * failure, non-401 error, malformed response) — distinct from "not
 * authenticated". Callers must render a system-unavailable state for this,
 * never silently redirect to /login, per
 * web/FRONTEND_ARCHITECTURE.md section 1.3.
 */
export class SessionUnavailableError extends Error {
  constructor(cause?: unknown) {
    super("Could not reach the authentication service.");
    this.name = "SessionUnavailableError";
    this.cause = cause;
  }
}

/**
 * Reads the current session by forwarding the incoming request's session
 * cookie straight to GET /api/v1/auth/me. Returns null when there is no
 * cookie or the backend reports 401 (not authenticated) — both are
 * legitimate "logged out" states, not errors. Throws SessionUnavailableError
 * for anything else, so route guards can tell "log in" apart from "system
 * is down" (Master Prompt task 8, 401 handling; also directly required by
 * ARCHITECTURE.md's "never treat a network failure as logged-out" posture
 * carried over from web/FRONTEND_ARCHITECTURE.md section 1.3).
 */
export async function getSession(): Promise<SessionUser | null> {
  const cookieStore = await cookies();
  const sessionCookie = cookieStore.get(SESSION_COOKIE_NAME);
  if (!sessionCookie || sessionCookie.value === "") {
    return null;
  }

  const headerStore = await headers();
  const acceptLanguage = headerStore.get("accept-language") ?? undefined;

  let response: Response;
  try {
    response = await callBackend({
      method: "GET",
      path: "/api/v1/auth/me",
      sessionCookie: `${SESSION_COOKIE_NAME}=${sessionCookie.value}`,
      acceptLanguage,
    });
  } catch (cause) {
    throw new SessionUnavailableError(cause);
  }

  if (response.status === 401) {
    return null;
  }

  try {
    const data = await parseApiEnvelope(response, meResponseSchema);
    return {
      id: data.id,
      email: data.email,
      displayName: data.display_name,
      locale: data.locale,
      roles: data.roles,
    };
  } catch (cause) {
    if (cause instanceof ApiError && cause.status === 401) {
      return null;
    }
    throw new SessionUnavailableError(cause);
  }
}
