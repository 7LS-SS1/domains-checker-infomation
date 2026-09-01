import { describe, expect, it } from "vitest";
import {
  availabilityTone,
  dnsTone,
  httpTone,
  ispTone,
  lifecycleTone,
  redirectTone,
  sourceStatusTone,
  tlsTone,
} from "@/lib/domains/status";
import {
  AVAILABILITY_STATUSES,
  DNS_STATUSES,
  HTTP_STATUSES,
  ISP_STATUSES,
  LIFECYCLE_STATUSES,
  REDIRECT_STATUSES,
  SOURCE_STATUSES,
  TLS_STATUSES,
} from "@/lib/domains/types";

describe("domain status tone mappings", () => {
  it.each(AVAILABILITY_STATUSES)("maps every availability_status value %s", (status) => {
    expect(availabilityTone(status)).toBeTruthy();
  });
  it.each(DNS_STATUSES)("maps every dns_status value %s", (status) => {
    expect(dnsTone(status)).toBeTruthy();
  });
  it.each(HTTP_STATUSES)("maps every http_status value %s", (status) => {
    expect(httpTone(status)).toBeTruthy();
  });
  it.each(REDIRECT_STATUSES)("maps every redirect_status value %s", (status) => {
    expect(redirectTone(status)).toBeTruthy();
  });
  it.each(ISP_STATUSES)("maps every isp_status value %s", (status) => {
    expect(ispTone(status)).toBeTruthy();
  });
  it.each(TLS_STATUSES)("maps every tls_status value %s", (status) => {
    expect(tlsTone(status)).toBeTruthy();
  });
  it.each(LIFECYCLE_STATUSES)("maps every lifecycle_status value %s", (status) => {
    expect(lifecycleTone(status)).toBeTruthy();
  });
  it.each(SOURCE_STATUSES)("maps every source_status value %s", (status) => {
    expect(sourceStatusTone(status)).toBeTruthy();
  });

  it("always renders ISP risk as violet, per the fixed palette rule", () => {
    expect(ispTone("SUSPECTED")).toBe("violet");
    expect(ispTone("HIGH_CONFIDENCE_BLOCK")).toBe("violet");
  });

  it("every UNKNOWN value across dimensions maps to slate, never a healthy/zero-looking color", () => {
    expect(availabilityTone("UNKNOWN")).toBe("slate");
    expect(dnsTone("UNKNOWN")).toBe("slate");
    expect(httpTone("UNKNOWN")).toBe("slate");
    expect(redirectTone("UNKNOWN")).toBe("slate");
    expect(ispTone("UNKNOWN")).toBe("slate");
    expect(tlsTone("UNKNOWN")).toBe("slate");
  });
});
