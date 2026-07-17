import { test, expect } from '../../fixtures/test.js';
import { MOCK_URL } from '../../helpers/api.js';

test.describe("Settings — Integration Settings", { tag: '@needs-mock' }, () => {

  test.describe("Jira Integration", () => {
    test("saves configuration", async ({ page, settingsPage }) => {
      await test.step("Open the Jira Integration settings panel", async () => {
        await settingsPage.open();
        await settingsPage.openTab("Jira");
      });

      await test.step("Enable Jira and fill in the connection fields", async () => {
        await settingsPage.enableIntegration(/Enable Jira integration/i);
        await settingsPage.fillIntegrationForm({ baseUrl: MOCK_URL, email: "test@example.com", token: "mock-token" });
      });

      await test.step("Save the configuration and verify the success message", async () => {
        await settingsPage.saveConfiguration();
        await expect(page.getByText("Jira configuration saved.")).toBeVisible();
      });
    });

    test("test connection fetches a ticket", async ({ page, api, settingsPage }) => {
      let ticketInput;

      await test.step("Pre-configure Jira via API", async () => {
        await api.configureJira();
      });

      await test.step("Open the Jira Integration panel and confirm the ticket input is visible", async () => {
        await settingsPage.open();
        await settingsPage.openTab("Jira");

        // The test connection section shows only once config exists and is enabled.
        ticketInput = settingsPage.jiraTestTicketInput;
        await expect(ticketInput).toBeVisible();
      });

      await test.step("Fetch a ticket and verify its summary appears", async () => {
        await ticketInput.fill("PROJ-101");
        await page.getByRole("button", { name: "Fetch Ticket" }).click();

        await expect(page.getByText("PROJ-101")).toBeVisible();
      });
    });
  });

  test.describe("Confluence Integration", () => {
    test("saves configuration", async ({ page, settingsPage }) => {
      await test.step("Open the Confluence Integration settings panel", async () => {
        await settingsPage.open();
        await settingsPage.openTab("Confluence");
      });

      await test.step("Enable Confluence and fill in the connection fields", async () => {
        await settingsPage.enableIntegration(/Enable Confluence integration/i);
        await settingsPage.fillIntegrationForm({ baseUrl: MOCK_URL, email: "test@example.com", token: "mock-token" });
      });

      await test.step("Save the configuration and verify the success message", async () => {
        await settingsPage.saveConfiguration();
        await expect(page.getByText("Confluence configuration saved.")).toBeVisible();
      });
    });

    test("test connection lists spaces", async ({ page, api, settingsPage }) => {
      await test.step("Pre-configure Confluence via API", async () => {
        await api.configureConfluence();
      });

      await test.step("Open the Confluence panel and run Test Connection", async () => {
        await settingsPage.open();
        await settingsPage.openTab("Confluence");

        await page.getByRole("button", { name: "Test Connection" }).click();
      });

      await test.step("Verify the connection success message appears", async () => {
        await expect(page.getByText("Connected successfully")).toBeVisible();
      });
    });
  });
});
