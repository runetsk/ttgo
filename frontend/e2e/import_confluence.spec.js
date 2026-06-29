import { test, expect } from '@playwright/test';
import { configureConfluenceAPI, deleteAllRequirements } from './helpers.js';

test.describe("Confluence Import", () => {
  test.beforeEach(async ({ page, request }) => {
    await deleteAllRequirements(request);
    await configureConfluenceAPI(request);
    await page.goto("/requirements");
  });

  test.describe("Single Import", () => {
    test("navigates through steps and imports a page", async ({ page }) => {
      await test.step("Open the Confluence import dialog and select a space", async () => {
        await page.getByRole("button", { name: "⬇ Confluence" }).click();

        // Step 1: select space and browse pages
        await expect(page.getByText("Step 1 of 3")).toBeVisible();
        await page.locator("select").selectOption({ label: "Requirements (REQ)" });
        await page.getByRole("button", { name: "Browse Pages" }).click();
      });

      await test.step("Select a page from the list", async () => {
        // Step 2: select a page from the list
        await expect(page.getByText("Step 2 of 3")).toBeVisible();
        await expect(page.getByText("Login Requirements")).toBeVisible();
        await page.getByText("Login Requirements").click();
      });

      await test.step("Preview and import the requirement", async () => {
        // Step 3: preview and import
        await expect(page.getByText("Step 3 of 3")).toBeVisible();
        await expect(page.getByText("Login Requirements")).toBeVisible();

        await page.getByRole("button", { name: "Import Requirement" }).click();

        // Verify the requirement appears on the page after import
        await expect(page.getByText("Login Requirements")).toBeVisible();
      });
    });

    test("back button navigates between steps", async ({ page }) => {
      await test.step("Open the Confluence import dialog and advance to step 2", async () => {
        await page.getByRole("button", { name: "⬇ Confluence" }).click();

        // Step 1: select space and go to step 2
        await expect(page.getByText("Step 1 of 3")).toBeVisible();
        await page.locator("select").selectOption({ label: "Requirements (REQ)" });
        await page.getByRole("button", { name: "Browse Pages" }).click();

        // Step 2: verify we are on step 2
        await expect(page.getByText("Step 2 of 3")).toBeVisible();
      });

      await test.step("Click back and verify return to step 1", async () => {
        // Click back to return to step 1
        await page.getByRole("button", { name: "← Back" }).click();
        await expect(page.getByText("Step 1 of 3")).toBeVisible();
      });
    });

    test("shows not-configured message when Confluence is disabled", async ({ page, request }) => {
      await test.step("Disable Confluence and verify the not-configured message", async () => {
        await configureConfluenceAPI(request, { enabled: false });
        await page.reload();

        await page.getByRole("button", { name: "⬇ Confluence" }).click();

        await expect(page.getByText("Confluence integration is not configured")).toBeVisible();
      });
    });
  });

  test.describe("Bulk Import", () => {
    test("selects space, loads pages, and imports selected", async ({ page }) => {
      await test.step("Open bulk Confluence import, select a space, and load pages", async () => {
        await page.getByRole("button", { name: "⬇⬇ Bulk Confluence" }).click();

        // Focus dropdown to trigger space loading, then select
        await page.locator("select").focus();
        await page.waitForTimeout(500);
        await page.locator("select").selectOption({ label: "Requirements (REQ)" });

        // Load pages
        await page.getByRole("button", { name: "Load Pages" }).click();

        // Verify pages are listed
        await expect(page.getByText("Login Requirements")).toBeVisible();
        await expect(page.getByText("Dashboard Requirements")).toBeVisible();
      });

      await test.step("Select all pages and import them", async () => {
        // Select all and import
        await page.getByRole("button", { name: "Select All", exact: true }).click();
        await page.getByRole("button", { name: /Import Selected \(2\)/ }).click();

        // Verify import results
        await expect(page.getByText("Imported")).toBeVisible();
      });
    });
  });
});
