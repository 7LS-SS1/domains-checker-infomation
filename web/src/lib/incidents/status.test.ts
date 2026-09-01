import { describe, expect, it } from "vitest";
import { incidentStatusIcon, incidentStatusTone, INCIDENT_STATUSES } from "@/lib/incidents/status";

describe("incident status → tone/icon mapping", () => {
  it.each(INCIDENT_STATUSES)("maps every known status %s to a tone and an icon", (status) => {
    expect(incidentStatusTone(status)).toBeTruthy();
    expect(incidentStatusIcon(status)).toBeTruthy();
  });

  it("maps open to rose (attention required)", () => {
    expect(incidentStatusTone("open")).toBe("rose");
  });

  it("maps closed to emerald (resolved)", () => {
    expect(incidentStatusTone("closed")).toBe("emerald");
  });

  it("falls back to slate for an unrecognized status rather than crashing", () => {
    expect(incidentStatusTone("some_future_status")).toBe("slate");
    expect(incidentStatusIcon("some_future_status")).toBeTruthy();
  });
});
