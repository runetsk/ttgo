import { test, expect } from '../../fixtures/test.js';

test.describe("Confluence Import", { tag: '@needs-mock' }, () => {
  test.beforeEach(async ({ api, requirementsPage }) => {
    await api.deleteAllRequirements();
    await api.configureConfluence();
    await requirementsPage.open();
  });

  test.describe("Single Import", () => {
    test("navigates through steps and imports a page", async ({ page, requirementsPage }) => {
      // The modal steps are Space → Page → Preview (a labelled indicator, not
      // "Step N of 3"), so each step is verified by its content.
      await test.step("Open the Confluence import dialog and select a space", async () => {
        await requirementsPage.openImportConfluence();
        await requirementsPage.selectConfluenceSpace("Requirements");
        await requirementsPage.browseConfluencePages();
      });

      await test.step("Select a page from the list", async () => {
        await expect(page.getByText("Login Requirements")).toBeVisible();
        await page.getByText("Login Requirements").click();
      });

      await test.step("Preview and import the requirement", async () => {
        await expect(page.getByRole("button", { name: "Import Requirement" })).toBeVisible();
        await expect(page.getByText("Login Requirements")).toBeVisible();

        await requirementsPage.importRequirement();

        await expect(page.getByText("Login Requirements")).toBeVisible();
      });
    });

    test("back button navigates between steps", async ({ page, requirementsPage }) => {
      await test.step("Open the dialog, select a space, and browse pages", async () => {
        await requirementsPage.openImportConfluence();
        await requirementsPage.selectConfluenceSpace("Requirements");
        await requirementsPage.browseConfluencePages();
        await expect(page.getByText("Login Requirements")).toBeVisible();
      });

      await test.step("Click back and verify return to the space step", async () => {
        await page.getByRole("button", { name: "← Back" }).click();
        // "Browse Pages" only renders on the space step.
        await expect(page.getByRole("button", { name: "Browse Pages" })).toBeVisible();
      });
    });

    test("shows not-configured message when Confluence is disabled", async ({ page, api, requirementsPage }) => {
      await test.step("Disable Confluence and verify the not-configured message", async () => {
        await api.configureConfluence({ enabled: false });
        await page.reload();

        await requirementsPage.openImportConfluence();

        await expect(page.getByText("Confluence integration is not configured")).toBeVisible();
      });
    });
  });

  test.describe("Bulk Import", () => {
    test("selects space, loads pages, and imports selected", async ({ page, requirementsPage }) => {
      await test.step("Open bulk Confluence import, select a space, and load pages", async () => {
        await requirementsPage.openBulkImportConfluence();
        await requirementsPage.loadBulkConfluenceSpace("Requirements (REQ)");

        await expect(page.getByText("Login Requirements")).toBeVisible();
        await expect(page.getByText("Dashboard Requirements")).toBeVisible();
      });

      await test.step("Select all pages and import them", async () => {
        await requirementsPage.selectAllBulk();
        await requirementsPage.importSelected(2);

        await expect(page.getByText("Imported")).toBeVisible();
      });
    });
  });
});
