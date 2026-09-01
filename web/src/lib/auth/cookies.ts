/**
 * Cookie name constants shared between BFF route handlers and server-side
 * session reads. Both cookies set by this app are HttpOnly and therefore
 * unreadable from client JavaScript — see web/FRONTEND_ARCHITECTURE.md
 * section 1.1. Only LOCALE_COOKIE_NAME is intentionally non-HttpOnly since
 * it carries no secret and must be readable for client-side language
 * switching.
 */
export const SESSION_COOKIE_NAME = "domainintel_session";
export const CSRF_COOKIE_NAME = "bff_csrf";
export const LOCALE_COOKIE_NAME = "NEXT_LOCALE";

export const SESSION_MAX_AGE_FALLBACK_SECONDS = 8 * 60 * 60;
