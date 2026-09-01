import { test, expect } from "@playwright/test";

/**
 * Settings page smoke — entirely read-only against the real backend (no
 * mutation, so gated the same as finance-recommendations-reports.spec.ts:
 * just an authenticated admin session, no E2E_ALLOW_MUTATION needed).
 */
const ADMIN_EMAIL = process.env.E2E_ADMIN_EMAIL;
const ADMIN_PASSWORD = process.env.E2E_ADMIN_PASSWORD;
const canRun = Boolean(ADMIN_EMAIL && ADMIN_PASSWORD);

test.beforeEach(async ({ context, baseURL, page }) => {
  const url = new URL(baseURL ?? "http://127.0.0.1:3100");
  await context.addCookies([{ name: "NEXT_LOCALE", value: "en", domain: url.hostname, path: "/" }]);
  test.skip(!canRun, "requires E2E_ADMIN_EMAIL/E2E_ADMIN_PASSWORD against a real backend");
  await page.goto("/login");
  await page.getByLabel("Email").fill(ADMIN_EMAIL!);
  await page.getByLabel("Password", { exact: true }).fill(ADMIN_PASSWORD!);
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page).toHaveURL(/\/dashboard/);
});

test("shows profile, system status, and a Drive connection summary from real endpoints", async ({
  page,
}) => {
  await page.goto("/settings");
  await expect(page.getByRole("heading", { name: "Settings" })).toBeVisible();

  await expect(page.getByText(ADMIN_EMAIL!)).toBeVisible();
  await expect(page.getByText("Profile fields are read-only")).toBeVisible();

  await expect(page.getByText("domain-monitor-api")).toBeVisible({ timeout: 10_000 });

  await expect(page.getByRole("link", { name: "Manage in Import Center" })).toBeVisible();
});
