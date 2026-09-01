import { test, expect } from "@playwright/test";

/**
 * Domains E2E flow: list -> create -> detail -> manual check -> runs/history.
 * Requires a live backend (E2E_ADMIN_EMAIL/E2E_ADMIN_PASSWORD) AND explicit
 * opt-in to create/mutate real data (E2E_ALLOW_MUTATION=true), per the
 * Master Prompt: never create or delete a test domain against real data
 * without that explicit flag. Skips cleanly otherwise.
 */
const ADMIN_EMAIL = process.env.E2E_ADMIN_EMAIL;
const ADMIN_PASSWORD = process.env.E2E_ADMIN_PASSWORD;
const ALLOW_MUTATION = process.env.E2E_ALLOW_MUTATION === "true";
const canRun = Boolean(ADMIN_EMAIL && ADMIN_PASSWORD && ALLOW_MUTATION);

test.beforeEach(async ({ context, baseURL }) => {
  const url = new URL(baseURL ?? "http://127.0.0.1:3100");
  await context.addCookies([{ name: "NEXT_LOCALE", value: "en", domain: url.hostname, path: "/" }]);
});

test.describe("domains list -> create -> detail -> manual check", () => {
  test.skip(
    !canRun,
    "requires E2E_ADMIN_EMAIL/E2E_ADMIN_PASSWORD and E2E_ALLOW_MUTATION=true against a real backend",
  );

  test("creates a domain, opens its detail page, and triggers a manual check", async ({ page }) => {
    await page.goto("/login");
    await page.getByLabel("Email").fill(ADMIN_EMAIL!);
    await page.getByLabel("Password", { exact: true }).fill(ADMIN_PASSWORD!);
    await page.getByRole("button", { name: "Sign in" }).click();
    await expect(page).toHaveURL(/\/dashboard/);

    await page.goto("/domains");
    await expect(page.getByRole("heading", { name: "Domains" })).toBeVisible();

    // The .example TLD (RFC 2606) is rejected by internal/domain.Normalizer
    // unless ALLOW_UNKNOWN_TLD=true — the default backend config has it
    // false, since Normalize() checks the real ICANN public suffix list.
    // Using a namespaced .com-style name keeps this collision-safe and
    // structurally valid without requiring a non-default backend config.
    const testDomain = `e2e-test-${Date.now()}.com`;
    await page.getByRole("button", { name: "Add domain" }).click();
    await page.getByLabel("Domain", { exact: true }).fill(testDomain);
    await page.getByRole("button", { name: "Add domain" }).last().click();

    // A generous timeout here: on a cold `next dev` process, this is the
    // first request that ever compiles the /domains/[domainId] route, and
    // that on-demand Turbopack compile can outlast the default 5s and
    // interrupt the in-flight client-side navigation. A production build
    // (Phase 8) precompiles every route and doesn't hit this at all.
    await expect(page).toHaveURL(/\/domains\/[0-9a-f-]+/, { timeout: 20_000 });
    await expect(page.getByRole("heading", { level: 1 })).toContainText(testDomain);

    await page.getByRole("button", { name: /run check now/i }).click();

    await page.getByRole("tab", { name: "Monitoring" }).click();
    await expect(page.getByText("Monitoring runs")).toBeVisible();
  });
});
