import { fileURLToPath } from "node:url";
import { test, expect, type Page, type Route } from "@playwright/test";

/**
 * Import Center (Phase Frontend 6) E2E coverage, split per the Master
 * Prompt's explicit requirement for BOTH a Google fixture/mocked-contract
 * test AND a real-backend Excel fixture test — and the rule that real
 * Google OAuth consent must never be claimed as tested without actually
 * exercising it (it isn't, here; see the "mocked Google Drive contract"
 * describe block below).
 */
const ADMIN_EMAIL = process.env.E2E_ADMIN_EMAIL;
const ADMIN_PASSWORD = process.env.E2E_ADMIN_PASSWORD;
const ALLOW_MUTATION = process.env.E2E_ALLOW_MUTATION === "true";
const hasCredentials = Boolean(ADMIN_EMAIL && ADMIN_PASSWORD);

const FIXTURE_XLSX = fileURLToPath(new URL("./fixtures/import-fixture.xlsx", import.meta.url));

async function jsonRoute(route: Route, status: number, body: unknown) {
  await route.fulfill({ status, contentType: "application/json", body: JSON.stringify(body) });
}

function apiError(code: string) {
  return {
    error: {
      code,
      message: code,
      messages: { th: code, en: code },
      locale: "en",
      request_id: "e2e-fixture-request-id",
    },
  };
}

test.beforeEach(async ({ context, baseURL }) => {
  const url = new URL(baseURL ?? "http://127.0.0.1:3100");
  await context.addCookies([{ name: "NEXT_LOCALE", value: "en", domain: url.hostname, path: "/" }]);
});

async function loginAsAdmin(page: Page) {
  await page.goto("/login");
  await page.getByLabel("Email").fill(ADMIN_EMAIL!);
  await page.getByLabel("Password", { exact: true }).fill(ADMIN_PASSWORD!);
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page).toHaveURL(/\/dashboard/);
}

/**
 * "Google fixture/mocked contract" test: requires a real backend/session
 * (the app's proxy validates the session server-side, so login must be
 * real), but every Google Drive boundary — the BFF's connection/connect/
 * files responses, and the OAuth popup's destination page — is intercepted
 * at the browser network layer via context.route(). No real Google account,
 * consent screen, or credential is used or required. This deliberately does
 * NOT claim real Google OAuth was verified.
 */
test.describe("Google Drive — mocked contract", () => {
  test.skip(!hasCredentials, "requires E2E_ADMIN_EMAIL/E2E_ADMIN_PASSWORD against a real backend");

  test("connect flow: popup opens, authorization_url is honored, and a completed connection closes the popup", async ({
    page,
    context,
  }) => {
    let connectionPollCount = 0;

    await context.route("**/api/bff/google-drive/connection", async (route) => {
      if (route.request().method() === "GET") {
        connectionPollCount += 1;
        if (connectionPollCount === 1) {
          await jsonRoute(route, 404, apiError("DRIVE_NOT_CONNECTED"));
          return;
        }
        await jsonRoute(route, 200, {
          data: {
            id: "e2e-fixture-connection-id",
            user_id: "e2e-fixture-user-id",
            google_email: "e2e-fixture@example.com",
            scopes: ["https://www.googleapis.com/auth/drive.readonly"],
            status: "active",
            token_expires_at: null,
            connected_at: new Date().toISOString(),
            updated_at: new Date().toISOString(),
          },
        });
        return;
      }
      await route.fallback();
    });

    await context.route("**/api/bff/google-drive/connect", async (route) => {
      await jsonRoute(route, 200, {
        data: {
          authorization_url: "https://accounts.google.com/o/oauth2/e2e-fixture-consent",
          expires_at: new Date(Date.now() + 5 * 60_000).toISOString(),
        },
      });
    });

    await context.route("**/api/bff/google-drive/files**", async (route) => {
      await jsonRoute(route, 200, { data: { items: [] } });
    });

    // Never let the popup make a real network request to Google — fulfill
    // the fixture consent URL locally so the popup navigation completes
    // deterministically and offline.
    await context.route("https://accounts.google.com/**", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "text/html",
        body: "<html><body>e2e fixture consent page</body></html>",
      });
    });

    await loginAsAdmin(page);
    await page.goto("/sync?tab=drive");
    await expect(page.getByText("Not connected")).toBeVisible();

    const popupPromise = page.waitForEvent("popup");
    await page.getByRole("button", { name: "Connect Google Drive" }).click();
    const popup = await popupPromise;
    await popup.waitForURL("https://accounts.google.com/o/oauth2/e2e-fixture-consent", {
      timeout: 10_000,
    });
    expect(popup.url()).toBe("https://accounts.google.com/o/oauth2/e2e-fixture-consent");

    // The bounded poll (POLL_INTERVAL_MS=2000 in use-drive-oauth-popup.ts)
    // picks up the "active" fixture response and closes the popup itself.
    await popup.waitForEvent("close", { timeout: 10_000 });
    await expect(page.getByText("Connected as e2e-fixture@example.com")).toBeVisible();
    await expect(page.getByText("No Google Sheets found")).toBeVisible();
  });

  test("DRIVE_NOT_CONFIGURED renders an honest empty state with no connect action", async ({
    page,
    context,
  }) => {
    await context.route("**/api/bff/google-drive/connection", async (route) => {
      await jsonRoute(route, 404, apiError("DRIVE_NOT_CONFIGURED"));
    });

    await loginAsAdmin(page);
    await page.goto("/sync?tab=drive");

    await expect(
      page.getByText("Google Drive OAuth is not configured on this backend"),
    ).toBeVisible();
    await expect(page.getByRole("button", { name: "Connect Google Drive" })).toHaveCount(0);
  });
});

/**
 * Real-backend Excel fixture flow. The .xlsx fixture (web/e2e/fixtures/
 * import-fixture.xlsx) is generated from internal/sheets' own excelize
 * usage — header row domain/renewal_price/currency, one data row — and
 * contains no secret or real business data. Preview-only assertions need
 * just an authenticated admin session (the same bar as report creation in
 * finance-recommendations-reports.spec.ts); the apply step is additionally
 * gated on E2E_ALLOW_MUTATION=true because it writes a real domain row.
 */
test.describe("Excel import — real backend fixture", () => {
  test.skip(!hasCredentials, "requires E2E_ADMIN_EMAIL/E2E_ADMIN_PASSWORD against a real backend");

  test("uploads the fixture workbook and renders a staged-rows review", async ({ page }) => {
    await loginAsAdmin(page);
    await page.goto("/sync?tab=excel");

    await page.getByLabel("File (.xlsx)").setInputFiles(FIXTURE_XLSX);
    await page.getByLabel("Source name").fill("e2e-fixture-source");
    await page.getByLabel("Sheet name", { exact: true }).fill("Domains");
    await page.getByRole("button", { name: "Upload and preview" }).click();

    await expect(page.getByText("Staged rows")).toBeVisible({ timeout: 15_000 });
    await expect(page.getByText("e2e-excel-fixture.com")).toBeVisible();

    if (ALLOW_MUTATION) {
      await page.getByRole("button", { name: "Apply" }).click();
      await page.getByLabel("Reason").fill("e2e sync.spec.ts fixture apply");
      await page.getByRole("button", { name: "Apply" }).last().click();
      await expect(page.getByText("This import has already been applied.")).toBeVisible({
        timeout: 15_000,
      });
    }
  });
});
