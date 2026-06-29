import { test, expect } from '@playwright/test';

test.describe('Test Tracking System E2E', () => {

    test.beforeEach(async ({ page }) => {
        // Ensure we are on the home page
        await page.goto('/');
    });

    test('should create a root folder and subfolder', async ({ page }) => {
        const timestamp = Date.now();
        const rootFolderName = `Root Folder ${timestamp}`;
        const subFolderName = `Sub Folder ${timestamp}`;

        let rootFolder;

        await test.step('Create a root folder', async () => {
            await page.getByTestId('create-root-folder-button').click();
            await page.getByTestId('modal-input').fill(rootFolderName);
            await page.getByTestId('modal-confirm-button').click();

            rootFolder = page.getByTestId('folder-name').filter({ hasText: rootFolderName });
            await expect(rootFolder).toBeVisible();
        });

        await test.step('Create a subfolder via the context menu', async () => {
            await rootFolder.click({ button: 'right' });
            await page.getByTestId('context-menu-create-subfolder').click();
            await page.getByTestId('modal-input').fill(subFolderName);
            await page.getByTestId('modal-confirm-button').click();

            const subFolder = page.getByTestId('folder-name').filter({ hasText: subFolderName });
            await expect(subFolder).toBeVisible();
        });
    });

    test('should create a test case in a folder', async ({ page }) => {
        const timestamp = Date.now();
        const rootFolderName = `Root Folder ${timestamp}`;
        const testName = `User Login Test ${timestamp}`;

        await test.step('Create a folder and select it', async () => {
            // Pre-requisite: Create a folder to put the test in
            await page.getByTestId('create-root-folder-button').click();
            await page.getByTestId('modal-input').fill(rootFolderName);
            await page.getByTestId('modal-confirm-button').click();

            const folder = page.getByTestId('folder-name').filter({ hasText: rootFolderName });
            await folder.click();
        });

        await test.step('Create a test case in the folder', async () => {
            await page.getByTestId('create-test-button').click();
            await page.getByTestId('modal-input').fill(testName);
            await page.getByTestId('modal-confirm-button').click();
        });

        await test.step('Verify the test case appears in the grid', async () => {
            const testRow = page.getByTestId('test-row').filter({ hasText: testName });
            await expect(testRow).toBeVisible();
        });
    });

    test('should manage suites and assign to test', async ({ page }) => {
        const timestamp = Date.now();
        const rootFolderName = `Root Folder ${timestamp}`;
        const testName = `User Login Test ${timestamp}`;
        const suiteName = `Regression ${timestamp}`;

        await test.step('Set up a folder and a test case', async () => {
            await page.getByTestId('create-root-folder-button').click();
            await page.getByTestId('modal-input').fill(rootFolderName);
            await page.getByTestId('modal-confirm-button').click();

            const folder = page.getByTestId('folder-name').filter({ hasText: rootFolderName }).first();
            await folder.click();

            await page.getByTestId('create-test-button').click();
            await page.getByTestId('modal-input').fill(testName);
            await page.getByTestId('modal-confirm-button').click();
        });

        await test.step('Create a suite in the Suite Manager', async () => {
            // Navigate to Suite Manager
            await page.getByRole('button', { name: 'Suites' }).click();
            await page.waitForURL(/\/suites/);

            await page.getByTestId('open-create-suite-modal').click();
            await page.getByTestId('suite-name-input').fill(suiteName);
            await page.getByTestId('create-suite-button').click();
            await expect(page.getByText(suiteName)).toBeVisible();
        });

        await test.step('Return to the library and re-select the folder', async () => {
            await page.getByRole('button', { name: 'Tests' }).click();

            // Re-select folder to see tests
            const refolder = page.getByTestId('folder-name').filter({ hasText: rootFolderName }).first();
            await refolder.click();
        });

        await test.step('Assign the suite to the test via the detail view', async () => {
            // Click test name to navigate (avoid checkbox in first column)
            await page.getByTestId('test-row').filter({ hasText: testName }).getByText(testName).click();

            // In Details View — verify test name input is focused
            await expect(page.getByTestId('test-case-name-input')).toHaveValue(testName);

            // Select suite from the "+ Suite" dropdown, then click Add
            await page.getByTestId('suite-select').selectOption({ label: suiteName });
            await page.getByTestId('add-suite-button').click();

            // Verify suite chip appears in the meta bar before saving
            await expect(page.locator('.meta-chip').filter({ hasText: suiteName })).toBeVisible();
        });

        await test.step('Save and verify the suite tag appears in the grid row', async () => {
            // Save — navigates back to folder view
            await page.getByRole('button', { name: 'Save Changes' }).click();

            // Verify suite tag appears in the grid row
            await expect(page.getByTestId('test-row').filter({ hasText: testName }).locator('.suite-tag')).toContainText(suiteName);
        });
    });

    test('should record execution results', async ({ page }) => {
        const timestamp = Date.now();
        const rootFolderName = `Root Folder ${timestamp}`;
        const testName = `User Login Test ${timestamp}`;

        let testRow;

        await test.step('Set up a folder and a test case', async () => {
            await page.getByTestId('create-root-folder-button').click();
            await page.getByTestId('modal-input').fill(rootFolderName);
            await page.getByTestId('modal-confirm-button').click();

            const folder = page.getByTestId('folder-name').filter({ hasText: rootFolderName }).first();
            await folder.click();

            await page.getByTestId('create-test-button').click();
            await page.getByTestId('modal-input').fill(testName);
            await page.getByTestId('modal-confirm-button').click();

            testRow = page.getByTestId('test-row').filter({ hasText: testName });
        });

        await test.step('Record a passing result via the bulk action', async () => {
            // Select row and use bulk actions
            await testRow.locator('input[type="checkbox"]').check();
            await page.getByTestId('bulk-pass-button').click();

            // Wait for selection to clear (action bar goes away)
            await expect(page.getByTestId('bulk-pass-button')).not.toBeVisible();
        });

        await test.step('Record a failing result via the bulk action', async () => {
            // Re-select for next action (bulk action clears selection)
            await testRow.locator('input[type="checkbox"]').check();

            // Wait for action bar to reappear
            await expect(page.getByTestId('bulk-fail-button')).toBeVisible({ timeout: 10000 });
            await page.getByTestId('bulk-fail-button').click();
            await expect(page.getByTestId('bulk-fail-button')).not.toBeVisible();
        });
    });

    test('should support multiple folder selection', async ({ page }) => {
        test.setTimeout(90000);
        const timestamp = Date.now();
        const f1 = `F1 ${timestamp}`;
        const f2 = `F2 ${timestamp}`;

        let node1;
        let node2;

        await test.step('Create two root folders', async () => {
            await page.getByTestId('create-root-folder-button').click();
            await page.getByTestId('modal-input').fill(f1);
            await page.getByTestId('modal-confirm-button').click();

            await page.getByTestId('create-root-folder-button').click();
            await page.getByTestId('modal-input').fill(f2);
            await page.getByTestId('modal-confirm-button').click();
        });

        await test.step('Select the first folder', async () => {
            node1 = page.getByTestId('folder-name').filter({ hasText: f1 }).first();
            await node1.click();
            await expect(node1).toHaveClass(/selected/); // Link itself has class
        });

        await test.step('Ctrl-select the second folder and verify both are selected', async () => {
            node2 = page.getByTestId('folder-name').filter({ hasText: f2 }).first();
            await node2.click({ modifiers: ['ControlOrMeta'] });

            // Verify both selected
            await expect(node1).toHaveClass(/selected/);
            await expect(node2).toHaveClass(/selected/);
        });

        await test.step('Verify the bulk action UI appears', async () => {
            await expect(page.getByText('Delete (2)')).toBeVisible();
        });
    });
});
