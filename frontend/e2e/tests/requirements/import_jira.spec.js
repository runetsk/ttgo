import { test, expect } from '@playwright/test';
import { API_URL, MOCK_URL, configureJiraAPI, deleteAllRequirements } from '../../helpers/api.js';

test.describe("Jira Import", () => {
  test.beforeEach(async ({ page, request }) => {
    await deleteAllRequirements(request);
    await configureJiraAPI(request);
    await page.goto("/requirements");
  });

  test.describe("Single Import", () => {
    test("fetches preview and imports a Jira ticket", async ({ page }) => {
      await test.step("Open the Jira import dialog and fetch a ticket preview", async () => {
        await page.getByRole("button", { name: "⬇ Jira" }).click();

        const input = page.getByPlaceholder("e.g. PROJ-123");
        await input.fill("PROJ-101");
        await page.getByRole("button", { name: "Fetch Preview" }).click();

        await expect(page.getByText("PROJ-101")).toBeVisible();
      });

      await test.step("Import the requirement and verify it appears", async () => {
        await page.getByRole("button", { name: "Import Requirement" }).click();

        await expect(page.getByText("PROJ-101")).toBeVisible();
      });
    });

    test("shows not-configured message when Jira is disabled", async ({ page, request }) => {
      await test.step("Disable Jira and verify the not-configured message", async () => {
        await configureJiraAPI(request, { enabled: false });
        await page.reload();

        await page.getByRole("button", { name: "⬇ Jira" }).click();

        await expect(page.getByText("Jira integration is not configured")).toBeVisible();
      });
    });

    test("shows already-imported warning on duplicate import", async ({ page }) => {
      let input;

      await test.step("Import a Jira ticket once", async () => {
        await page.getByRole("button", { name: "⬇ Jira" }).click();

        input = page.getByPlaceholder("e.g. PROJ-123");
        await input.fill("PROJ-101");
        await page.getByRole("button", { name: "Fetch Preview" }).click();
        await page.getByRole("button", { name: "Import Requirement" }).click();
      });

      await test.step("Fetch the same ticket again and verify the already-imported warning", async () => {
        await page.getByRole("button", { name: "⬇ Jira" }).click();

        await input.fill("PROJ-101");
        await page.getByRole("button", { name: "Fetch Preview" }).click();

        await expect(page.getByText(/already been imported/)).toBeVisible();
        await expect(page.getByRole("button", { name: "Import Requirement" })).toBeHidden();
      });
    });
  });

  test.describe("Bulk Import", () => {
    test("searches JQL, selects tickets, and imports", async ({ page }) => {
      await test.step("Open bulk Jira import and search by JQL", async () => {
        await page.getByRole("button", { name: "⬇⬇ Bulk Jira" }).click();

        const input = page.getByPlaceholder(/project = PROJ/);
        await input.fill("project = PROJ");
        await page.getByRole("button", { name: "Search" }).click();

        await expect(page.getByText("PROJ-101")).toBeVisible();
        await expect(page.getByText("PROJ-102")).toBeVisible();
        await expect(page.getByText("PROJ-103")).toBeVisible();
      });

      await test.step("Select all tickets and import them", async () => {
        await page.getByRole("button", { name: "Select All", exact: true }).click();
        await page.getByRole("button", { name: /Import Selected \(3\)/ }).click();

        await expect(page.getByText("Imported")).toBeVisible();
      });
    });

    test("select all and deselect all work correctly", async ({ page }) => {
      await test.step("Open bulk Jira import and search by JQL", async () => {
        await page.getByRole("button", { name: "⬇⬇ Bulk Jira" }).click();

        const input = page.getByPlaceholder(/project = PROJ/);
        await input.fill("project = PROJ");
        await page.getByRole("button", { name: "Search" }).click();

        await expect(page.getByText("PROJ-101")).toBeVisible();
      });

      await test.step("Select all tickets and verify the selected count", async () => {
        await page.getByRole("button", { name: "Select All", exact: true }).click();
        await expect(page.getByText("3 selected")).toBeVisible();
      });

      await test.step("Deselect all tickets and verify the selected count", async () => {
        await page.getByRole("button", { name: "Deselect All" }).click();
        await expect(page.getByText("0 selected")).toBeVisible();
      });
    });
  });
});
