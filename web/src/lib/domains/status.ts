import type { StatusTone } from "@/lib/status/tokens";

/**
 * Per-dimension status → tone mapping, following the Master Prompt's UX
 * rule (ONLINE=emerald, DEGRADED/REVIEW=amber, OFFLINE/error=rose,
 * UNKNOWN=slate, ISP risk=violet). Each dimension has its own enum
 * semantics (see web/API_CONTRACT_MATRIX.md section 9), so each gets its
 * own mapping rather than one generic function guessing at meaning.
 */
export function availabilityTone(status: string): StatusTone {
  switch (status) {
    case "ACTIVE":
      return "emerald";
    case "DEGRADED":
      return "amber";
    case "UNAVAILABLE":
      return "rose";
    default:
      return "slate";
  }
}

export function dnsTone(status: string): StatusTone {
  switch (status) {
    case "OK":
      return "emerald";
    case "DISCREPANCY":
      return "amber";
    case "NXDOMAIN":
    case "SERVFAIL":
    case "REFUSED":
    case "TIMEOUT":
    case "NETWORK_ERROR":
      return "rose";
    default:
      return "slate";
  }
}

export function httpTone(status: string): StatusTone {
  switch (status) {
    case "OK":
      return "emerald";
    case "REDIRECT":
      return "amber";
    case "CLIENT_ERROR":
    case "SERVER_ERROR":
    case "TIMEOUT":
    case "CONNECTION_ERROR":
      return "rose";
    default:
      return "slate";
  }
}

export function redirectTone(status: string): StatusTone {
  switch (status) {
    case "NONE":
      return "emerald";
    case "TEMPORARY":
      return "amber";
    case "PERMANENT":
      return "slate";
    case "LOOP":
    case "INVALID":
    case "HTTPS_DOWNGRADE":
      return "rose";
    default:
      return "slate";
  }
}

/** ISP risk always renders violet per the Master Prompt's explicit palette rule. */
export function ispTone(status: string): StatusTone {
  switch (status) {
    case "NOT_DETECTED":
      return "emerald";
    case "SUSPECTED":
    case "HIGH_CONFIDENCE_BLOCK":
      return "violet";
    default:
      return "slate";
  }
}

export function tlsTone(status: string): StatusTone {
  switch (status) {
    case "VALID":
      return "emerald";
    case "EXPIRING":
      return "amber";
    case "EXPIRED":
    case "HOSTNAME_MISMATCH":
    case "INVALID":
    case "ERROR":
      return "rose";
    default:
      return "slate";
  }
}

export function lifecycleTone(status: string): StatusTone {
  switch (status) {
    case "active":
      return "emerald";
    case "inactive":
      return "slate";
    case "archived":
      return "slate";
    default:
      return "slate";
  }
}

export function sourceStatusTone(status: string): StatusTone {
  switch (status) {
    case "present":
      return "emerald";
    case "missing_from_source":
      return "amber";
    default:
      return "slate";
  }
}
