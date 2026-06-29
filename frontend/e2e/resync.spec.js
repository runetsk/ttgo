import { test, expect } from '@playwright/test';
import { API_URL, configureJiraAPI, deleteAllRequirements } from './helpers.js';

/**
 * Helper: import a single Jira ticket via the UI import flow.
 */
async function importJiraTicket(page, ticketKey) {
    await page.getByRole("button", { name: "\u2B07 Jira" }).click();

    const input = page.getByPlaceholder("e.g. PROJ-123");
    await input.fill(ticketKey);
    await page.getByRole("button", { name: "Fetch Preview" }).click();

    await expect(page.getByText(ticketKey)).toBeVisible();

    await page.getByRole("button", { name: "Import Requirement" }).click();

    // Wait for the requirement to appear in the list
    await expect(page.getByText(ticketKey)).toBeVisible();
}

/**
 * Helper: edit a requirement's title inline via the Edit button.
 */
async function editRequirementTitle(page, currentTitle, newTitle) {
    const row = page.locator("tr", { hasText: currentTitle });
    await row.hover();
    await row.getByRole("button", { name: "Edit" }).click();

    const titleInput = page.getByPlaceholder('Short description of the requirement');
    await titleInput.clear();
    await titleInput.fill(newTitle);

    await page.getByRole("button", { name: "Save Changes" }).click();

    // Wait for save to complete
    await expect(page.getByText(newTitle)).toBeVisible();
}

test.describe("Resync & Unlink", () => {
    test.beforeEach(async ({ page, request }) => {
        await deleteAllRequirements(request);
        await configureJiraAPI(request);
        await page.goto("/requirements");
    });

    test("resync auto-updates when no local edits", async ({ page }) => {
        await test.step("Import the Jira ticket PROJ-101", async () => {
            await importJiraTicket(page, "PROJ-101");
        });

        await test.step("Hover the row and click Sync", async () => {
            // Hover to reveal action buttons and click Sync
            const row = page.locator("tr", { hasText: "PROJ-101" });
            await row.hover();
            await row.getByRole("button", { name: "\u21BB Sync" }).click();
        });

        await test.step("Verify the auto-update success toast appears", async () => {
            // Verify success toast appears (modal auto-closes after update)
            await expect(page.getByText("Requirement auto-updated from source.")).toBeVisible();
        });
    });

    test("resync shows conflict when local edits exist", async ({ page }) => {
        await test.step("Import the Jira ticket PROJ-101", async () => {
            await importJiraTicket(page, "PROJ-101");
        });

        await test.step("Edit the title locally to create a conflict", async () => {
            // Edit the title locally to create a conflict
            await editRequirementTitle(page, "User login should validate email format", "Locally edited title");
        });

        await test.step("Hover the row and click Sync", async () => {
            // Hover and click Sync
            const row = page.locator("tr", { hasText: "PROJ-101" });
            await row.hover();
            await row.getByRole("button", { name: "\u21BB Sync" }).click();
        });

        await test.step("Verify the side-by-side conflict view is shown", async () => {
            // Verify conflict side-by-side is shown
            await expect(page.getByText("Local (current)")).toBeVisible();
            await expect(page.getByText("Remote (Jira)")).toBeVisible();
        });
    });

    test("accept remote resolves conflict with remote data", async ({ page }) => {
        const originalTitle = "Dashboard should show recent activity";

        await test.step("Import the Jira ticket PROJ-102", async () => {
            await importJiraTicket(page, "PROJ-102");
        });

        await test.step("Edit the title locally to create a conflict", async () => {
            // Edit the title locally to create a conflict
            await editRequirementTitle(page, originalTitle, "Locally modified dashboard title");
        });

        await test.step("Hover the row, click Sync, and wait for the conflict view", async () => {
            // Hover and click Sync
            const row = page.locator("tr", { hasText: "PROJ-102" });
            await row.hover();
            await row.getByRole("button", { name: "\u21BB Sync" }).click();

            // Wait for the conflict view to appear
            await expect(page.getByText("Local (current)")).toBeVisible();
        });

        await test.step("Accept Remote and verify the title reverts to the remote value", async () => {
            // Click Accept Remote to resolve with remote data
            await page.getByRole("button", { name: "Accept Remote" }).click();

            // Verify the title reverts to the original remote title
            await expect(page.getByText(originalTitle)).toBeVisible();
        });
    });

    test("unlink removes source association", async ({ page }) => {
        let row;

        await test.step("Import the Jira ticket PROJ-103", async () => {
            await importJiraTicket(page, "PROJ-103");
        });

        await test.step("Hover the row and verify Sync and Unlink buttons are visible", async () => {
            // Verify Sync and Unlink buttons are visible on hover
            row = page.locator("tr", { hasText: "PROJ-103" });
            await row.hover();
            await expect(row.getByRole("button", { name: "\u21BB Sync" })).toBeVisible();
            await expect(row.getByRole("button", { name: "\u2298 Unlink" })).toBeVisible();
        });

        await test.step("Accept the confirm dialog and click Unlink", async () => {
            // Accept the confirm dialog and click Unlink
            page.on("dialog", (dialog) => dialog.accept());
            await row.getByRole("button", { name: "\u2298 Unlink" }).click();
        });

        await test.step("Verify the Sync button is no longer visible after unlinking", async () => {
            // Verify Sync button is no longer visible (source association removed)
            await row.hover();
            await expect(row.getByRole("button", { name: "\u21BB Sync" })).toBeHidden();
        });
    });
});
