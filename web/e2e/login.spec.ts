import { test, expect } from "@playwright/test";

/**
 * Login smoke suite. Two tiers:
 *
 * 1. Always runs, no backend required: unauthenticated access to a
 *    protected route redirects to /login with a validated returnTo. This
 *    only exercises src/proxy.ts's no-cookie branch, which never calls the
 *    backend.
 * 2. Only runs when E2E_ADMIN_EMAIL / E2E_ADMIN_PASSWORD are set in the
 *    environment (never committed — see .env.local.example, which
 *    intentionally does not define these) AND a real backend is reachable
 *    at API_INTERNAL_URL. Skips cleanly otherwise, per the Master Prompt:
 *    "Playwright login smoke กับ backend จริงผ่าน environment credential
 *    ที่ไม่ commit ถ้ามี".
 *
 * The app defaults to Thai; every test forces the NEXT_LOCALE=en cookie
 * first so assertions can target stable English copy regardless of which
 * locale a real user last picked.
 */
const ADMIN_EMAIL = process.env.E2E_ADMIN_EMAIL;
const ADMIN_PASSWORD = process.env.E2E_ADMIN_PASSWORD;
const hasCredentials = Boolean(ADMIN_EMAIL && ADMIN_PASSWORD);

test.beforeEach(async ({ context, baseURL }) => {
  const url = new URL(baseURL ?? "http://127.0.0.1:3100");
  await context.addCookies([{ name: "NEXT_LOCALE", value: "en", domain: url.hostname, path: "/" }]);
});

test.describe("unauthenticated access", () => {
  test("redirects /dashboard to /login with a validated returnTo", async ({ page }) => {
    await page.goto("/dashboard?tab=active");

    await expect(page).toHaveURL(/\/login\?returnTo=%2Fdashboard/);
    await expect(page.getByRole("heading", { name: "Sign in" })).toBeVisible();
  });
});

test.describe("login against a real backend", () => {
  test.skip(
    !hasCredentials,
    "E2E_ADMIN_EMAIL / E2E_ADMIN_PASSWORD not set — skipping live login smoke",
  );

  test("signs in, reaches the dashboard, and the session cookie is not readable from document.cookie", async ({
    page,
    context,
  }) => {
    await page.goto("/login");

    await page.getByLabel("Email").fill(ADMIN_EMAIL!);
    await page.getByLabel("Password", { exact: true }).fill(ADMIN_PASSWORD!);
    await page.getByRole("button", { name: "Sign in" }).click();

    await expect(page).toHaveURL(/\/dashboard/);
    await expect(page.getByRole("heading", { level: 1 })).toBeVisible();

    const cookies = await context.cookies();
    const sessionCookie = cookies.find((cookie) => cookie.name === "domainintel_session");
    const csrfCookie = cookies.find((cookie) => cookie.name === "bff_csrf");
    expect(sessionCookie?.httpOnly).toBe(true);
    expect(csrfCookie?.httpOnly).toBe(true);

    const readableFromJs = await page.evaluate(() => document.cookie);
    expect(readableFromJs).not.toContain("domainintel_session");
    expect(readableFromJs).not.toContain("bff_csrf");
  });

  test("logs out and returns to /login, and /dashboard is protected again", async ({ page }) => {
    await page.goto("/login");
    await page.getByLabel("Email").fill(ADMIN_EMAIL!);
    await page.getByLabel("Password", { exact: true }).fill(ADMIN_PASSWORD!);
    await page.getByRole("button", { name: "Sign in" }).click();
    await expect(page).toHaveURL(/\/dashboard/);

    await page.getByRole("button", { name: /^User menu:/ }).click();
    await page.getByRole("menuitem", { name: "Sign out" }).click();

    await expect(page).toHaveURL(/\/login/);
    await page.goto("/dashboard");
    await expect(page).toHaveURL(/\/login\?returnTo=%2Fdashboard/);
  });

  test("shows a localized error for wrong credentials without redirecting", async ({ page }) => {
    await page.goto("/login");
    await page.getByLabel("Email").fill(ADMIN_EMAIL!);
    await page.getByLabel("Password", { exact: true }).fill("definitely-the-wrong-password");
    await page.getByRole("button", { name: "Sign in" }).click();

    await expect(page.getByRole("alert")).toBeVisible();
    await expect(page).toHaveURL(/\/login/);
  });
});
