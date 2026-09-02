/**
 * Route table for the generic BFF passthrough handler
 * (src/app/api/bff/[...path]/route.ts). This is what prevents the BFF from
 * becoming an open proxy: only a request whose method AND path pattern
 * match an entry here — with every `:param` segment validated as a UUID —
 * is ever forwarded to the Go API. Anything else (path traversal, an
 * unknown backend route, a disallowed method on an otherwise-known path, a
 * non-UUID value where a UUID is required) is rejected before any outbound
 * fetch is attempted.
 *
 * Auth (login/logout) is handled by dedicated route files, not this table,
 * because those routes need bespoke cookie-issuing logic beyond a plain
 * passthrough.
 */
export type HttpMethod = "GET" | "POST" | "PATCH" | "DELETE" | "PUT";

export interface RouteMethodConfig {
  /** Whether this method requires the caller to hold a valid session. */
  readonly requiresAuth: boolean;
  /** Whether X-CSRF-Token must be attached (mirrors requireCSRF in server.go). */
  readonly requiresCsrf: boolean;
  /** Exact query-parameter names this method may forward. Anything else is dropped. */
  readonly queryParams?: readonly string[];
  /** Whether the request body is read and forwarded verbatim (JSON routes only — see phase 6 for multipart). */
  readonly forwardsBody?: boolean;
  /** Whether an Idempotency-Key header, if present, is forwarded. */
  readonly forwardsIdempotencyKey?: boolean;
}

export interface BffRoute {
  /** Segments after /api/bff/, with ":name" denoting a captured path parameter (always validated as a UUID). */
  readonly pattern: readonly string[];
  readonly backendPath: (params: Readonly<Record<string, string>>) => string;
  readonly methods: Readonly<Partial<Record<HttpMethod, RouteMethodConfig>>>;
}

const UUID_PATTERN = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

export function isUuid(value: string): boolean {
  return UUID_PATTERN.test(value);
}

const readOnly = (queryParams?: readonly string[]): RouteMethodConfig => ({
  requiresAuth: true,
  requiresCsrf: false,
  ...(queryParams ? { queryParams } : {}),
});

const mutating = (overrides: Partial<RouteMethodConfig> = {}): RouteMethodConfig => ({
  requiresAuth: true,
  requiresCsrf: true,
  forwardsBody: true,
  ...overrides,
});

export const BFF_ROUTES: readonly BffRoute[] = [
  {
    pattern: ["auth", "me"],
    backendPath: () => "/api/v1/auth/me",
    methods: { GET: readOnly() },
  },
  {
    pattern: ["meta", "locales"],
    backendPath: () => "/api/v1/meta/locales",
    methods: { GET: { requiresAuth: false, requiresCsrf: false } },
  },
  {
    pattern: ["system", "health"],
    backendPath: () => "/health",
    methods: { GET: { requiresAuth: false, requiresCsrf: false } },
  },
  {
    pattern: ["system", "ready"],
    backendPath: () => "/ready",
    methods: { GET: { requiresAuth: false, requiresCsrf: false } },
  },
  {
    pattern: ["reports", "summary"],
    backendPath: () => "/api/v1/reports/summary",
    methods: { GET: readOnly(["reporting_currency"]) },
  },
  {
    pattern: ["reports", "dashboard"],
    backendPath: () => "/api/v1/reports/dashboard",
    methods: { GET: readOnly(["reporting_currency"]) },
  },
  {
    pattern: ["finance", "summary"],
    backendPath: () => "/api/v1/finance/summary",
    methods: { GET: readOnly(["reporting_currency"]) },
  },
  {
    pattern: ["finance", "exchange-rates"],
    backendPath: () => "/api/v1/finance/exchange-rates",
    methods: { POST: mutating() },
  },
  {
    pattern: ["incidents"],
    backendPath: () => "/api/v1/incidents",
    methods: { GET: readOnly(["status", "page", "page_size"]) },
  },
  {
    pattern: ["recommendations"],
    backendPath: () => "/api/v1/recommendations",
    methods: { GET: readOnly(["action", "limit"]) },
  },
  {
    pattern: ["recommendations", "run"],
    backendPath: () => "/api/v1/recommendations/run",
    methods: { POST: mutating() },
  },
  {
    pattern: ["reports"],
    backendPath: () => "/api/v1/reports",
    methods: { POST: mutating({ forwardsIdempotencyKey: true }) },
  },
  {
    pattern: ["reports", ":reportId"],
    backendPath: (p) => `/api/v1/reports/${p.reportId}`,
    methods: { GET: readOnly() },
  },
  {
    pattern: ["google-sheets", "config"],
    backendPath: () => "/api/v1/google-sheets/config",
    methods: {
      GET: readOnly(),
      PUT: mutating(),
    },
  },
  {
    pattern: ["google-sheets", "previews"],
    backendPath: () => "/api/v1/google-sheets/previews",
    methods: { POST: mutating({ forwardsIdempotencyKey: true, forwardsBody: false }) },
  },
  {
    pattern: ["google-sheets", "imports"],
    backendPath: () => "/api/v1/google-sheets/imports",
    methods: { GET: readOnly(["limit"]) },
  },
  {
    pattern: ["google-sheets", "imports", ":importId"],
    backendPath: (p) => `/api/v1/google-sheets/imports/${p.importId}`,
    methods: { GET: readOnly() },
  },
  {
    pattern: ["google-sheets", "imports", ":importId", "apply"],
    backendPath: (p) => `/api/v1/google-sheets/imports/${p.importId}/apply`,
    methods: { POST: mutating({ forwardsIdempotencyKey: true }) },
  },
  {
    pattern: ["google-sheets", "imports", ":importId", "reject"],
    backendPath: (p) => `/api/v1/google-sheets/imports/${p.importId}/reject`,
    methods: { POST: mutating() },
  },
  {
    pattern: ["google-drive", "connection"],
    backendPath: () => "/api/v1/google-drive/connection",
    methods: {
      GET: readOnly(),
      DELETE: mutating(),
    },
  },
  {
    pattern: ["google-drive", "connect"],
    backendPath: () => "/api/v1/google-drive/connect",
    methods: { POST: mutating({ forwardsBody: false }) },
  },
  {
    pattern: ["google-drive", "files"],
    backendPath: () => "/api/v1/google-drive/files",
    methods: { GET: readOnly(["page_token"]) },
  },
  {
    pattern: ["probes"],
    backendPath: () => "/api/v1/probes",
    methods: { GET: readOnly() },
  },
  {
    pattern: ["probes", "registration-tokens"],
    backendPath: () => "/api/v1/probes/registration-tokens",
    methods: { POST: mutating() },
  },
  {
    pattern: ["probes", ":probeId", "revoke"],
    backendPath: (p) => `/api/v1/probes/${p.probeId}/revoke`,
    methods: { POST: mutating() },
  },
  {
    pattern: ["domains"],
    backendPath: () => "/api/v1/domains",
    methods: {
      GET: readOnly([
        "query",
        "lifecycle_status",
        "source_status",
        "page",
        "page_size",
        "sort",
        "direction",
      ]),
      POST: mutating(),
    },
  },
  {
    pattern: ["domains", ":domainId"],
    backendPath: (p) => `/api/v1/domains/${p.domainId}`,
    methods: {
      GET: readOnly(),
      PATCH: mutating(),
    },
  },
  {
    pattern: ["domains", ":domainId", "archive"],
    backendPath: (p) => `/api/v1/domains/${p.domainId}/archive`,
    methods: { POST: mutating() },
  },
  {
    pattern: ["domains", ":domainId", "restore"],
    backendPath: (p) => `/api/v1/domains/${p.domainId}/restore`,
    methods: { POST: mutating() },
  },
  {
    pattern: ["domains", ":domainId", "check"],
    backendPath: (p) => `/api/v1/domains/${p.domainId}/check`,
    methods: { POST: mutating({ forwardsIdempotencyKey: true, forwardsBody: false }) },
  },
  {
    pattern: ["domains", ":domainId", "isp-check"],
    backendPath: (p) => `/api/v1/domains/${p.domainId}/isp-check`,
    methods: { POST: mutating({ forwardsIdempotencyKey: true, forwardsBody: false }) },
  },
  {
    pattern: ["domains", ":domainId", "monitoring-runs"],
    backendPath: (p) => `/api/v1/domains/${p.domainId}/monitoring-runs`,
    methods: { GET: readOnly(["page", "page_size"]) },
  },
  {
    pattern: ["domains", ":domainId", "monitoring-history"],
    backendPath: (p) => `/api/v1/domains/${p.domainId}/monitoring-history`,
    methods: { GET: readOnly(["window"]) },
  },
  {
    pattern: ["monitoring-runs", ":runId"],
    backendPath: (p) => `/api/v1/monitoring-runs/${p.runId}`,
    methods: { GET: readOnly() },
  },
  {
    pattern: ["domains", ":domainId", "rdap"],
    backendPath: (p) => `/api/v1/domains/${p.domainId}/rdap`,
    methods: { GET: readOnly() },
  },
  {
    pattern: ["domains", ":domainId", "rdap-check"],
    backendPath: (p) => `/api/v1/domains/${p.domainId}/rdap-check`,
    methods: { POST: mutating() },
  },
  {
    pattern: ["domains", ":domainId", "costs"],
    backendPath: (p) => `/api/v1/domains/${p.domainId}/costs`,
    methods: {
      GET: readOnly(),
      POST: mutating(),
    },
  },
  {
    pattern: ["domains", ":domainId", "overrides"],
    backendPath: (p) => `/api/v1/domains/${p.domainId}/overrides`,
    methods: {
      GET: readOnly(),
      POST: mutating(),
    },
  },
  {
    pattern: ["domains", ":domainId", "overrides", ":overrideId"],
    backendPath: (p) => `/api/v1/domains/${p.domainId}/overrides/${p.overrideId}`,
    methods: { DELETE: mutating() },
  },
  {
    pattern: ["domains", ":domainId", "provenance"],
    backendPath: (p) => `/api/v1/domains/${p.domainId}/provenance`,
    methods: { GET: readOnly() },
  },
  {
    pattern: ["domains", ":domainId", "recommendation"],
    backendPath: (p) => `/api/v1/domains/${p.domainId}/recommendation`,
    methods: {
      GET: readOnly(),
      POST: mutating(),
    },
  },
];

export interface ResolvedRoute {
  readonly route: BffRoute;
  readonly methodConfig: RouteMethodConfig;
  readonly backendPath: string;
}

/**
 * Matches an incoming BFF request's method + path segments against
 * BFF_ROUTES. A ":param" pattern segment captures the corresponding actual
 * segment, but only if that value is a syntactically valid UUID — a
 * non-UUID value (including any attempt to smuggle extra path segments
 * through a captured slot) fails the match entirely rather than being
 * forwarded to the backend.
 */
export function resolveRoute(method: string, segments: readonly string[]): ResolvedRoute | null {
  const normalizedMethod = method.toUpperCase() as HttpMethod;

  for (const route of BFF_ROUTES) {
    if (route.pattern.length !== segments.length) {
      continue;
    }
    const params: Record<string, string> = {};
    let matched = true;
    for (let index = 0; index < route.pattern.length; index += 1) {
      const patternSegment = route.pattern[index]!;
      const actualSegment = segments[index]!;
      if (patternSegment.startsWith(":")) {
        if (!isUuid(actualSegment)) {
          matched = false;
          break;
        }
        params[patternSegment.slice(1)] = actualSegment;
      } else if (patternSegment !== actualSegment) {
        matched = false;
        break;
      }
    }
    if (!matched) {
      continue;
    }
    const methodConfig = route.methods[normalizedMethod];
    if (!methodConfig) {
      continue;
    }
    return { route, methodConfig, backendPath: route.backendPath(params) };
  }
  return null;
}

/**
 * Builds the query string to forward to the backend for a resolved route,
 * keeping only params named in methodConfig.queryParams and dropping
 * everything else.
 */
export function buildAllowlistedQuery(
  methodConfig: RouteMethodConfig,
  searchParams: URLSearchParams,
): string {
  const allowed = methodConfig.queryParams ?? [];
  const filtered = new URLSearchParams();
  for (const name of allowed) {
    const value = searchParams.get(name);
    if (value !== null) {
      filtered.set(name, value);
    }
  }
  const query = filtered.toString();
  return query ? `?${query}` : "";
}
