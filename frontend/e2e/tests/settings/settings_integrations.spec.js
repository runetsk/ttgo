import { test, expect } from '@playwright/test';
import { API_URL, MOCK_URL, configureJiraAPI, configureConfluenceAPI } from '../../helpers/api.js';

test.describe("Settings — Integration Settings", () => {

  // ── Jira Integration ────────────────────────────────────────────────────────

  test.describe("Jira Integration", () => {
    test("saves configuration", async ({ page }) => {
      await test.step("Open the Jira Integration settings panel", async () => {
        await page.goto("/settings");

        await page.getByRole("button", { name: "Jira Integration" }).click();
      });

      await test.step("Enable Jira and fill in the connection fields", async () => {
        await page.getByLabel(/Enable Jira integration/i).check();
        await page.getByPlaceholder("https://yourcompany.atlassian.net").fill(MOCK_URL);
        await page.getByPlaceholder("you@yourcompany.com").fill("test@example.com");
        await page.locator('input[type="password"]').fill("mock-token");
      });

      await test.step("Save the configuration and verify the success message", async () => {
        await page.getByRole("button", { name: "Save Configuration" }).click();

        await expect(page.getByText("Jira configuration saved.")).toBeVisible();
      });
    });

    test("test connection fetches a ticket", async ({ page, request }) => {
      let ticketInput;

      await test.step("Pre-configure Jira via API", async () => {
        // Pre-configure Jira via API so the form is already saved and enabled
        await configureJiraAPI(request);
      });

      await test.step("Open the Jira Integration panel and confirm the ticket input is visible", async () => {
        await page.goto("/settings");
        await page.getByRole("button", { name: "Jira Integration" }).click();

        // The test connection section should be visible since config exists and is enabled
        ticketInput = page.getByPlaceholder("e.g. PROJ-123");
        await expect(ticketInput).toBeVisible();
      });

      await test.step("Fetch a ticket and verify its summary appears", async () => {
        await ticketInput.fill("PROJ-101");
        await page.getByRole("button", { name: "Fetch Ticket" }).click();

        // Verify the fetched ticket summary appears on the page
        await expect(page.getByText("PROJ-101")).toBeVisible();
      });
    });
  });

  // ── Confluence Integration ──────────────────────────────────────────────────

  test.describe("Confluence Integration", () => {
    test("saves configuration", async ({ page }) => {
      await test.step("Open the Confluence Integration settings panel", async () => {
        await page.goto("/settings");

        await page.getByRole("button", { name: "Confluence Integration" }).click();
      });

      await test.step("Enable Confluence and fill in the connection fields", async () => {
        await page.getByLabel(/Enable Confluence integration/i).check();
        await page.getByPlaceholder("https://yourcompany.atlassian.net").fill(MOCK_URL);
        await page.getByPlaceholder("you@yourcompany.com").fill("test@example.com");
        await page.locator('input[type="password"]').fill("mock-token");
      });

      await test.step("Save the configuration and verify the success message", async () => {
        await page.getByRole("button", { name: "Save Configuration" }).click();

        await expect(page.getByText("Confluence configuration saved.")).toBeVisible();
      });
    });

    test("test connection lists spaces", async ({ page, request }) => {
      await test.step("Pre-configure Confluence via API", async () => {
        // Pre-configure Confluence via API so the form is already saved and enabled
        await configureConfluenceAPI(request);
      });

      await test.step("Open the Confluence panel and run Test Connection", async () => {
        await page.goto("/settings");
        await page.getByRole("button", { name: "Confluence Integration" }).click();

        await page.getByRole("button", { name: "Test Connection" }).click();
      });

      await test.step("Verify the connection success message appears", async () => {
        await expect(page.getByText("Connected successfully")).toBeVisible();
      });
    });
  });
});
