import { test, expect } from "@playwright/test";

/**
 * Probe Nodes page. The list view is read-only against the real backend;
 * creating a registration token is a real write (a registration_tokens
 * row) of a security-sensitive artifact, so — like domains.spec.ts — it is
 * additionally gated on E2E_ALLOW_MUTATION=true. Revoking a probe cannot be
 * exercised here without a live probe agent actually consuming a
 * registration token to register itself first, which is outside this
 * frontend's control; that gap is called out explicitly rather than faked.
 */
const ADMIN_EMAIL = process.env.E2E_ADMIN_EMAIL;
const ADMIN_PASSWORD = process.env.E2E_ADMIN_PASSWORD;
const ALLOW_MUTATION = process.env.E2E_ALLOW_MUTATION === "true";
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

test("lists probe nodes from the real backend", async ({ page }) => {
  await page.goto("/probes");
  await expect(page.getByRole("heading", { name: "Probe Nodes" })).toBeVisible();
});

test("creates a registration token, shows it exactly once, and never re-shows it after closing", async ({
  page,
}) => {
  test.skip(
    !ALLOW_MUTATION,
    "requires E2E_ALLOW_MUTATION=true — writes a real registration_tokens row",
  );

  await page.goto("/probes");
  await page.getByRole("button", { name: "Create registration token" }).click();

  const tokenName = `e2e-probe-token-${Date.now()}`;
  await page.getByLabel("Name", { exact: true }).fill(tokenName);
  await page.getByLabel("Region code").fill("apac");
  await page.getByLabel("Country code").fill("th");
  await page.getByLabel("Valid for (hours)").fill("1");
  await page.getByRole("button", { name: "Create token" }).click();

  await expect(page.getByText("Registration token created")).toBeVisible({ timeout: 15_000 });
  const tokenField = page.getByLabel("Registration token", { exact: true });
  await expect(tokenField).toBeVisible();
  const revealedToken = await tokenField.inputValue();
  expect(revealedToken.length).toBeGreaterThan(10);

  await page.getByRole("button", { name: "Done" }).click();
  await expect(page.getByText("Registration token created")).not.toBeVisible();

  // Reopening the dialog must show a fresh create form, never the
  // already-consumed token from the previous creation.
  await page.getByRole("button", { name: "Create registration token" }).click();
  await expect(page.getByLabel("Name", { exact: true })).toHaveValue("");
  await expect(page.getByText(revealedToken)).toHaveCount(0);
});
