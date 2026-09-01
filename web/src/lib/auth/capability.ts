/**
 * RBAC capability matrix. Sourced directly from the `requireRoles(...)`
 * call sites in internal/api/server.go, as documented and cross-referenced
 * in web/API_CONTRACT_MATRIX.md and web/FRONTEND_ARCHITECTURE.md section 5
 * — NOT from ARCHITECTURE.md's aspirational RBAC table, which does not
 * match the running code (see web/API_GAPS.md GAP-05). The backend is
 * always the final authority; this matrix only controls what the UI shows
 * or hides so users are not offered actions the backend will reject.
 */
export type Role = "ADMIN" | "STAFF" | "VIEWER";

export function isKnownRole(value: string): value is Role {
  return value === "ADMIN" || value === "STAFF" || value === "VIEWER";
}

/**
 * The backend's roles array is untyped JSON on the wire; this narrows it to
 * only the roles this frontend understands so a future/unexpected role
 * string can never silently widen a capability check.
 */
export function toKnownRoles(roles: readonly string[]): Role[] {
  return roles.filter(isKnownRole);
}

export type Capability =
  | "viewDomains"
  | "editDomains"
  | "manageOverrides"
  | "manageExchangeRates"
  | "previewImport"
  | "applyImport"
  | "manageSheetConfig"
  | "manageDriveConnection"
  | "viewDriveConnection"
  | "generateRecommendation"
  | "viewRecommendations"
  | "createReport"
  | "viewReports"
  | "viewProbes"
  | "manageProbes"
  | "viewIncidents";

const CAPABILITY_ROLES: Readonly<Record<Capability, readonly Role[]>> = {
  viewDomains: ["ADMIN", "STAFF", "VIEWER"],
  editDomains: ["ADMIN", "STAFF"],
  manageOverrides: ["ADMIN"],
  manageExchangeRates: ["ADMIN"],
  previewImport: ["ADMIN", "STAFF"],
  applyImport: ["ADMIN"],
  manageSheetConfig: ["ADMIN"],
  manageDriveConnection: ["ADMIN"],
  viewDriveConnection: ["ADMIN", "STAFF", "VIEWER"],
  generateRecommendation: ["ADMIN", "STAFF"],
  viewRecommendations: ["ADMIN", "STAFF", "VIEWER"],
  createReport: ["ADMIN", "STAFF"],
  viewReports: ["ADMIN", "STAFF", "VIEWER"],
  viewProbes: ["ADMIN", "STAFF", "VIEWER"],
  manageProbes: ["ADMIN"],
  viewIncidents: ["ADMIN", "STAFF", "VIEWER"],
};

export function hasCapability(roles: readonly Role[], capability: Capability): boolean {
  const allowedRoles = CAPABILITY_ROLES[capability];
  return roles.some((role) => allowedRoles.includes(role));
}

export function hasAnyCapability(
  roles: readonly Role[],
  capabilities: readonly Capability[],
): boolean {
  return capabilities.some((capability) => hasCapability(roles, capability));
}

export function capabilitiesFor(roles: readonly Role[]): ReadonlySet<Capability> {
  const capabilities = Object.keys(CAPABILITY_ROLES) as Capability[];
  return new Set(capabilities.filter((capability) => hasCapability(roles, capability)));
}
