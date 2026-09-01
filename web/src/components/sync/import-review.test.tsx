import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { ImportReview } from "@/components/sync/import-review";
import type { SheetImport } from "@/lib/sheets/types";
import enMessages from "@/messages/en.json";

const baseImport: SheetImport = {
  id: "11111111-1111-4111-8111-111111111111",
  config_id: null,
  spreadsheet_id: "sheet-1",
  sheet_name: "Domains",
  status: "preview",
  trigger_type: "manual",
  source_kind: "google_sheets",
  source_metadata: {},
  source_revision: "1",
  source_hash: "abc",
  column_mapping: {},
  total_rows: 4,
  added_count: 1,
  modified_count: 1,
  unchanged_count: 1,
  missing_count: 0,
  invalid_count: 1,
  valid_rows_applied: 0,
  requested_by: "22222222-2222-4222-8222-222222222222",
  previewed_at: "2026-08-29T00:00:00Z",
  applied_at: null,
  rejected_at: null,
  rows: [
    {
      id: "a",
      row_number: 2,
      matched_domain_id: null,
      action: "ADD",
      valid: true,
      raw_values: { domain: "new.example" },
      normalized_values: null,
      validation_errors: [],
      diff: {},
    },
    {
      id: "b",
      row_number: 3,
      matched_domain_id: "33333333-3333-4333-8333-333333333333",
      action: "MODIFY",
      valid: true,
      raw_values: { domain: "changed.example" },
      normalized_values: null,
      validation_errors: [],
      diff: {},
    },
    {
      id: "c",
      row_number: 4,
      matched_domain_id: "44444444-4444-4444-8444-444444444444",
      action: "UNCHANGED",
      valid: true,
      raw_values: { domain: "same.example" },
      normalized_values: null,
      validation_errors: [],
      diff: {},
    },
    {
      id: "d",
      row_number: 5,
      matched_domain_id: null,
      action: "INVALID",
      valid: false,
      raw_values: { domain: "not a domain" },
      normalized_values: null,
      validation_errors: ["INVALID_DOMAIN"],
      diff: {},
    },
  ],
};

function renderReview(roles: readonly string[]) {
  render(
    <NextIntlClientProvider locale="en" messages={enMessages}>
      <ImportReview
        importRecord={baseImport}
        roles={roles}
        onApply={vi.fn()}
        onReject={vi.fn()}
        isApplying={false}
        isRejecting={false}
      />
    </NextIntlClientProvider>,
  );
}

describe("ImportReview", () => {
  it("renders every staged row with its action label, including INVALID rows and their errors", () => {
    renderReview(["ADMIN"]);
    expect(screen.getByText("new.example")).toBeInTheDocument();
    expect(screen.getByText("changed.example")).toBeInTheDocument();
    expect(screen.getByText("same.example")).toBeInTheDocument();
    expect(screen.getByText("not a domain")).toBeInTheDocument();
    expect(screen.getByText("INVALID_DOMAIN")).toBeInTheDocument();
  });

  it("shows Apply/Reject actions for ADMIN (applyImport capability)", () => {
    renderReview(["ADMIN"]);
    expect(screen.getByRole("button", { name: /apply/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /reject/i })).toBeInTheDocument();
  });

  it("hides Apply/Reject actions for STAFF — apply is ADMIN-only per web/API_GAPS.md GAP-05", () => {
    renderReview(["STAFF"]);
    expect(screen.queryByRole("button", { name: /^apply$/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /^reject$/i })).not.toBeInTheDocument();
  });

  it("hides Apply/Reject actions for VIEWER", () => {
    renderReview(["VIEWER"]);
    expect(screen.queryByRole("button", { name: /^apply$/i })).not.toBeInTheDocument();
  });
});
