import { test, expect } from '../../fixtures/test.js';

test.describe("Jira Import", { tag: '@needs-mock' }, () => {
  test.beforeEach(async ({ api, requirementsPage }) => {
    await api.deleteAllRequirements();
    await api.configureJira();
    await requirementsPage.open();
  });

  test.describe("Single Import", () => {
    test("fetches preview and imports a Jira ticket", async ({ page, requirementsPage }) => {
      await test.step("Open the Jira import dialog and fetch a ticket preview", async () => {
        await requirementsPage.openImportJira();
        await requirementsPage.fetchJiraPreview("PROJ-101");

        await expect(page.getByText("PROJ-101")).toBeVisible();
      });

      await test.step("Import the requirement and verify it appears", async () => {
        await requirementsPage.importRequirement();

        await expect(page.getByText("PROJ-101")).toBeVisible();
      });
    });

    test("shows not-configured message when Jira is disabled", async ({ page, api, requirementsPage }) => {
      await test.step("Disable Jira and verify the not-configured message", async () => {
        await api.configureJira({ enabled: false });
        await page.reload();

        await requirementsPage.openImportJira();

        await expect(page.getByText("Jira integration is not configured")).toBeVisible();
      });
    });

    test("shows already-imported warning on duplicate import", async ({ page, requirementsPage }) => {
      await test.step("Import a Jira ticket once", async () => {
        await requirementsPage.openImportJira();
        await requirementsPage.fetchJiraPreview("PROJ-101");
        await requirementsPage.importRequirement();
      });

      await test.step("Fetch the same ticket again and verify the already-imported warning", async () => {
        await requirementsPage.openImportJira();
        await requirementsPage.fetchJiraPreview("PROJ-101");

        await expect(page.getByText(/already been imported/)).toBeVisible();
        await expect(page.getByRole("button", { name: "Import Requirement" })).toBeHidden();
      });
    });
  });

  test.describe("Bulk Import", () => {
    test("searches JQL, selects tickets, and imports", async ({ page, requirementsPage }) => {
      await test.step("Open bulk Jira import and search by JQL", async () => {
        await requirementsPage.openBulkImportJira();
        await requirementsPage.searchBulkJira("project = PROJ");

        await expect(page.getByText("PROJ-101")).toBeVisible();
        await expect(page.getByText("PROJ-102")).toBeVisible();
        await expect(page.getByText("PROJ-103")).toBeVisible();
      });

      await test.step("Select all tickets and import them", async () => {
        await requirementsPage.selectAllBulk();
        await requirementsPage.importSelected(3);

        await expect(page.getByText("Imported")).toBeVisible();
      });
    });

    test("select all and deselect all work correctly", async ({ page, requirementsPage }) => {
      await test.step("Open bulk Jira import and search by JQL", async () => {
        await requirementsPage.openBulkImportJira();
        await requirementsPage.searchBulkJira("project = PROJ");

        await expect(page.getByText("PROJ-101")).toBeVisible();
      });

      await test.step("Select all tickets and verify the selected count", async () => {
        await requirementsPage.selectAllBulk();
        await expect(page.getByText("3 selected")).toBeVisible();
      });

      await test.step("Deselect all tickets and verify the selected count", async () => {
        await requirementsPage.deselectAllBulk();
        await expect(page.getByText("0 selected")).toBeVisible();
      });
    });
  });
});
