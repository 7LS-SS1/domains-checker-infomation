import { render, screen } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, expect, it } from "vitest";
import {
  ConfidenceRing,
  DomainLifecycleDot,
  DomainListIspBadge,
} from "@/components/domains/domain-list-indicators";
import thMessages from "@/messages/th.json";
import type { Domain } from "@/lib/domains/types";

function renderWithMessages(component: React.ReactNode) {
  return render(
    <NextIntlClientProvider locale="th" messages={thMessages}>
      {component}
    </NextIntlClientProvider>,
  );
}

describe("domain list indicators", () => {
  const baseDomain = {
    lifecycle_status: "active",
    current_failure_stage: null,
    current_error_code: null,
  } as Domain;

  it("renders confidence as an accessible circular progress indicator", () => {
    renderWithMessages(<ConfidenceRing value={85} />);

    expect(screen.getByRole("progressbar", { name: "ความเชื่อมั่น: 85%" })).toHaveAttribute(
      "aria-valuenow",
      "85",
    );
    expect(screen.getByText("85%")).toBeInTheDocument();
  });

  it("renders reachable ISP checks as normal", () => {
    renderWithMessages(<DomainListIspBadge status="NOT_DETECTED" />);
    expect(screen.getByText("ปกติ")).toBeInTheDocument();
  });

  it.each(["SUSPECTED", "HIGH_CONFIDENCE_BLOCK"])("renders %s ISP checks as VPN", (status) => {
    renderWithMessages(<DomainListIspBadge status={status} />);
    expect(screen.getByText("VPN")).toBeInTheDocument();
  });

  it("keeps unknown ISP checks neutral", () => {
    renderWithMessages(<DomainListIspBadge status="UNKNOWN" />);
    expect(screen.getByText("ไม่ทราบ")).toBeInTheDocument();
  });

  it.each([
    ["active", baseDomain, "ใช้งาน"],
    ["cancelled", { ...baseDomain, lifecycle_status: "archived" }, "ยกเลิก"],
    ["not used", { ...baseDomain, lifecycle_status: "inactive" }, "ยังไม่ได้ใช้งาน"],
    ["check problem", { ...baseDomain, current_error_code: "TIMEOUT" }, "มีปัญหาตรวจสอบไม่ได้"],
  ])("renders the %s domain state dot", (_name, domain, label) => {
    renderWithMessages(<DomainLifecycleDot domain={domain} />);
    expect(screen.getByRole("img", { name: label })).toBeInTheDocument();
  });
});
