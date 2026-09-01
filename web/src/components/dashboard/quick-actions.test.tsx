import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { QuickActions } from "@/components/dashboard/quick-actions";
import enMessages from "@/messages/en.json";

function renderQuickActions(roles: readonly string[]) {
  return render(
    <NextIntlClientProvider locale="en" messages={enMessages}>
      <QuickActions roles={roles} />
    </NextIntlClientProvider>,
  );
}

describe("QuickActions (role-based rendering)", () => {
  it("shows every action for ADMIN", () => {
    renderQuickActions(["ADMIN"]);
    expect(screen.getByRole("link", { name: /add domain/i })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /run recommendations/i })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /import excel/i })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /connect google drive/i })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /create report/i })).toBeInTheDocument();
  });

  it("hides ADMIN-only actions for STAFF but keeps the shared ones", () => {
    renderQuickActions(["STAFF"]);
    expect(screen.getByRole("link", { name: /add domain/i })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /run recommendations/i })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /import excel/i })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /create report/i })).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /connect google drive/i })).not.toBeInTheDocument();
  });

  it("renders nothing at all for VIEWER — a read-only role gets no mutating shortcuts", () => {
    const { container } = renderQuickActions(["VIEWER"]);
    expect(container).toBeEmptyDOMElement();
  });

  it("renders nothing for an empty/unknown role list rather than erroring", () => {
    const { container } = renderQuickActions([]);
    expect(container).toBeEmptyDOMElement();
  });
});
