import { test, expect } from '../../fixtures/test.js';

test.describe("Resync & Unlink", { tag: '@needs-mock' }, () => {
    test.beforeEach(async ({ api, requirementsPage }) => {
        await api.deleteAllRequirements();
        await api.configureJira();
        await requirementsPage.open();
    });

    test("resync auto-updates when no local edits", async ({ page, requirementsPage }) => {
        await test.step("Import the Jira ticket PROJ-101", async () => {
            await requirementsPage.importJiraTicket("PROJ-101");
        });

        await test.step("Re-sync the row from its Actions menu", async () => {
            await requirementsPage.resyncRow("PROJ-101");
        });

        await test.step("Verify the auto-update success toast appears", async () => {
            await expect(page.getByText("Requirement auto-updated from source.")).toBeVisible();
        });
    });

    test("resync shows conflict when local edits exist", async ({ page, requirementsPage }) => {
        await test.step("Import the Jira ticket PROJ-101", async () => {
            await requirementsPage.importJiraTicket("PROJ-101");
        });

        await test.step("Edit the title locally to create a conflict", async () => {
            await requirementsPage.editTitle("User login should validate email format", "Locally edited title");
        });

        await test.step("Re-sync the row from its Actions menu", async () => {
            await requirementsPage.resyncRow("PROJ-101");
        });

        await test.step("Verify the side-by-side conflict view is shown", async () => {
            await expect(page.getByText("Local (current)")).toBeVisible();
            await expect(page.getByText("Remote (Jira)")).toBeVisible();
        });
    });

    test("accept remote resolves conflict with remote data", async ({ page, requirementsPage }) => {
        const originalTitle = "Dashboard should show recent activity";

        await test.step("Import the Jira ticket PROJ-102", async () => {
            await requirementsPage.importJiraTicket("PROJ-102");
        });

        await test.step("Edit the title locally to create a conflict", async () => {
            await requirementsPage.editTitle(originalTitle, "Locally modified dashboard title");
        });

        await test.step("Re-sync from the Actions menu and wait for the conflict view", async () => {
            await requirementsPage.resyncRow("PROJ-102");
            await expect(page.getByText("Local (current)")).toBeVisible();
        });

        await test.step("Accept Remote and verify the title reverts to the remote value", async () => {
            await page.getByRole("button", { name: "Accept Remote" }).click();
            await expect(page.getByText(originalTitle)).toBeVisible();
        });
    });

    test("unlink removes source association", async ({ page, requirementsPage }) => {
        await test.step("Import the Jira ticket PROJ-103", async () => {
            await requirementsPage.importJiraTicket("PROJ-103");
        });

        await test.step("Open the row menu and verify Re-sync and Unlink actions exist", async () => {
            await requirementsPage.openRowActions("PROJ-103");
            await expect(requirementsPage.menuItem("Re-sync")).toBeVisible();
            await expect(requirementsPage.menuItem("Unlink")).toBeVisible();
        });

        await test.step("Accept the confirm dialog and click Unlink", async () => {
            page.on("dialog", (dialog) => dialog.accept());
            await requirementsPage.menuItem("Unlink").click();
        });

        await test.step("Verify Re-sync is no longer offered after unlinking", async () => {
            await requirementsPage.openRowActions("PROJ-103");
            await expect(requirementsPage.menuItem("Re-sync")).toBeHidden();
        });
    });
});
