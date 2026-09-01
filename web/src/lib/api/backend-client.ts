/**
 * Server-only client for talking to the Go API directly. Must never be
 * imported from a Client Component — it reads a server-side-only env var
 * and forwards the raw session cookie, neither of which may reach the
 * browser bundle. All Next.js Route Handlers under src/app/api/bff/** are
 * the only permitted callers (see web/FRONTEND_ARCHITECTURE.md section 1.1).
 */

const MUTATING_METHODS = new Set(["POST", "PUT", "PATCH", "DELETE"]);

export function isMutatingMethod(method: string): boolean {
  return MUTATING_METHODS.has(method.toUpperCase());
}

export function getBackendBaseUrl(): string {
  const url = process.env.API_INTERNAL_URL;
  if (!url) {
    throw new Error("API_INTERNAL_URL is not configured. Copy .env.local.example to .env.local.");
  }
  return url;
}

export interface BackendRequestHeaderInput {
  method: string;
  sessionCookie?: string | undefined;
  acceptLanguage?: string | undefined;
  /**
   * The BFF's own server-side CSRF token (read from the HttpOnly bff_csrf
   * cookie, never from client JS). Only attached to the outbound backend
   * request when the method is mutating — GET requests never carry it, even
   * if a caller passes one by mistake. This is the enforcement point for
   * the Master Prompt rule "mutating request แนบ CSRF แต่ GET ไม่แนบ".
   */
  csrfToken?: string | undefined;
  contentType?: string | undefined;
  /**
   * A small, explicit set of additional headers to forward (currently only
   * ever `Idempotency-Key`, set by the BFF route after validating it
   * itself — never a blind passthrough of arbitrary client headers).
   */
  extraHeaders?: Record<string, string | undefined> | undefined;
}

export function buildBackendRequestHeaders(input: BackendRequestHeaderInput): Headers {
  const headers = new Headers();
  if (input.contentType) {
    headers.set("Content-Type", input.contentType);
  }
  if (input.acceptLanguage) {
    headers.set("Accept-Language", input.acceptLanguage);
  }
  if (input.sessionCookie) {
    headers.set("Cookie", input.sessionCookie);
  }
  if (isMutatingMethod(input.method) && input.csrfToken) {
    headers.set("X-CSRF-Token", input.csrfToken);
  }
  for (const [name, value] of Object.entries(input.extraHeaders ?? {})) {
    if (value !== undefined) {
      headers.set(name, value);
    }
  }
  return headers;
}

export interface CallBackendOptions extends BackendRequestHeaderInput {
  path: string;
  body?: BodyInit | undefined;
}

/**
 * Issues the actual outbound request to the Go API. redirect:"manual" and
 * cache:"no-store" are deliberate: the backend never issues redirects on
 * these routes, and every response here is either session-bearing or
 * safety-critical (auth, readiness) — nothing here may be cached.
 */
export async function callBackend(options: CallBackendOptions): Promise<Response> {
  const url = new URL(options.path, getBackendBaseUrl());
  const headers = buildBackendRequestHeaders(options);
  return fetch(url, {
    method: options.method,
    headers,
    body: options.body,
    redirect: "manual",
    cache: "no-store",
  });
}
