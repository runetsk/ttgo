import { test, expect } from '../../fixtures/test.js';
import { TIMEOUTS } from '../../config.js';

test.describe('Deep Linking', () => {

    test('should navigate to test detail via URL and persist', async ({ page, libraryPage }) => {
        const folderName = `DeepLink Folder ${Date.now()}`;
        const testName = `DeepLink Test ${Date.now()}`;
        let testId;

        await test.step('Create a folder and a test case via the UI', async () => {
            await libraryPage.open();
            await libraryPage.createRootFolder(folderName);
            await libraryPage.selectFolder(folderName);
            await libraryPage.createTestCase(testName);
        });

        await test.step('Click the test to navigate and verify the detail page URL', async () => {
            // From a selected folder this opens the three-pane route
            // /library/folders/<folderId>/tests/<testId> — match the stable /tests/<id> tail.
            await page.getByText(testName).click();
            await expect(page).toHaveURL(/\/tests\/[^/]+$/);
            testId = page.url().split('/tests/')[1];
            await expect(page.getByTestId('test-case-name-input')).toHaveValue(testName);
        });

        await test.step('Reload the page and verify the test detail persists', async () => {
            await page.reload();
            await expect(page.getByTestId('test-case-name-input')).toHaveValue(testName);
            expect(page.url()).toContain(testId);
        });

        await test.step('Close the detail and verify navigation away from the test URL', async () => {
            await page.getByTestId('close-modal-button').click();
            await expect(page.url()).not.toContain('/tests/');
        });
    });

    // FIXME: folder deletion now cascade-deletes its test cases (backend F-015), so a test
    // can no longer be "orphaned" (folder gone, test surviving). This asserts a removed feature
    // (the folder-missing warning banner). Restore an orphaned-test path or delete this test.
    test.fixme('should show warning banner and folder picker when test folder has been deleted', async ({ page, api }) => {
        const folderName = 'Deleted Folder ' + Date.now();
        const testName = 'Orphaned Test ' + Date.now();
        let testCase;

        await test.step('Create a folder and a test via API', async () => {
            const folder = await api.createFolder(folderName);
            testCase = await api.createTest(testName, folder.id);
            const deleteRes = await api.delete(`/folders/${folder.id}`);
            expect(deleteRes.status()).toBe(204);
        });

        await test.step('Navigate to the orphaned test and verify the warning banner', async () => {
            await page.goto(`/library/tests/${testCase.id}`);
            await page.waitForLoadState('domcontentloaded');
            await expect(page.getByText("This test's folder has been deleted")).toBeVisible({ timeout: TIMEOUTS.ELEMENT });
        });

        await test.step('Verify the folder picker and a disabled Move button are present', async () => {
            const moveBtn = page.getByRole('button', { name: 'Move' });
            await expect(moveBtn).toBeVisible();
            await expect(moveBtn).toBeDisabled();
        });

        await test.step('Verify the test still loads with its name', async () => {
            await expect(page.getByTestId('test-case-name-input')).toHaveValue(testName);
        });

        await test.step('Close the modal and verify navigation to /library', async () => {
            await page.getByTestId('close-modal-button').click();
            await expect(page).toHaveURL(/\/library$/);
        });
    });

    // FIXME: same removed feature as above — folder deletion cascades to its tests (F-015),
    // so an orphaned test (and its warning-banner folder picker) can't be set up anymore.
    test.fixme('should move orphaned test to a new folder via the warning banner picker', async ({ page, api }) => {
        const folderName = 'ToDelete Folder ' + Date.now();
        const targetFolderName = 'Target Folder ' + Date.now();
        const testName = 'Move-Me Test ' + Date.now();
        let testCase;

        await test.step('Create two folders and a test via API', async () => {
            const delFolder = await api.createFolder(folderName);
            await api.createFolder(targetFolderName);
            testCase = await api.createTest(testName, delFolder.id);
            await api.delete(`/folders/${delFolder.id}`);
        });

        await test.step('Navigate to the orphaned test and verify the warning banner', async () => {
            await page.goto(`/library/tests/${testCase.id}`);
            await page.waitForLoadState('domcontentloaded');
            await expect(page.getByText("This test's folder has been deleted")).toBeVisible({ timeout: TIMEOUTS.ELEMENT });
        });

        await test.step('Select the target folder and click Move', async () => {
            const folderPicker = page.locator('select.meta-select').last();
            await folderPicker.selectOption({ label: targetFolderName });
            const moveBtn = page.getByRole('button', { name: 'Move' });
            await expect(moveBtn).toBeEnabled();
            await moveBtn.click();
        });

        await test.step('Verify the warning banner disappears after the move', async () => {
            await expect(page.getByText("This test's folder has been deleted")).not.toBeVisible({ timeout: TIMEOUTS.ELEMENT });
        });

        await test.step('Verify the sidebar highlights the target folder', async () => {
            await expect(page.locator('.folder-header.selected')).toContainText(targetFolderName);
        });
    });
});
