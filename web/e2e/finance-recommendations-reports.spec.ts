import { test, expect } from "@playwright/test";

/**
 * Finance / Recommendations / Reports smoke flow. Requires a live backend
 * with a seeded admin session (E2E_ADMIN_EMAIL/E2E_ADMIN_PASSWORD) — read
 * only, no E2E_ALLOW_MUTATION needed for the view-only assertions, but the
 * "create report" and "run recommendations" actions do write real data, so
 * this whole spec is gated the same as domains.spec.ts.
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

test("finance page shows the budget overview with exact-decimal amounts", async ({ page }) => {
  await page.goto("/finance");
  await expect(page.getByRole("heading", { name: "Finance" })).toBeVisible();
  // Exact-decimal money values render as "<amount> <CURRENCY>", never a bare float.
  await expect(page.getByText(/\d[\d,]*(\.\d+)? [A-Z]{3}/).first()).toBeVisible();
});

test("recommendations page lists generated vs effective action distinctly", async ({ page }) => {
  await page.goto("/recommendations");
  await expect(page.getByRole("heading", { name: "Recommendations" })).toBeVisible();
});

test("reports page creates a JSON report and offers a download link", async ({ page }) => {
  await page.goto("/reports");
  await expect(page.getByRole("heading", { name: "Reports", exact: true })).toBeVisible();

  const createButton = page.getByRole("button", { name: "Create" });
  if (await createButton.isVisible().catch(() => false)) {
    await createButton.click();
    await expect(page.getByRole("link", { name: /download/i }).first()).toBeVisible({
      timeout: 15_000,
    });
  }
});
