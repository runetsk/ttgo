import { test, expect } from '@playwright/test';
import { API_URL } from '../../config.js';

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
        const categoryName = `Regression ${timestamp}`;

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

        await test.step('Create a category in the Category Manager', async () => {
            // Navigate to Category Manager (Suites was renamed to Categories)
            await page.getByRole('button', { name: 'Categories' }).click();
            await page.waitForURL(/\/categories/);

            await page.getByTestId('open-create-category-modal').click();
            await page.getByTestId('category-name-input').fill(categoryName);
            await page.getByTestId('create-category-button').click();
            await expect(page.getByText(categoryName)).toBeVisible();
        });

        await test.step('Return to the library and re-select the folder', async () => {
            await page.getByRole('button', { name: 'Tests' }).click();

            // Re-select folder to see tests
            const refolder = page.getByTestId('folder-name').filter({ hasText: rootFolderName }).first();
            await refolder.click();
        });

        await test.step('Assign the category to the test via the detail view', async () => {
            // Click test name to navigate (avoid checkbox in first column)
            await page.getByTestId('test-row').filter({ hasText: testName }).getByText(testName).click();

            // In Details View — verify test name input is focused
            await expect(page.getByTestId('test-case-name-input')).toHaveValue(testName);

            // Selecting from the "+ Category" dropdown adds the category immediately
            await page.getByTestId('category-select').selectOption({ label: categoryName });

            // Verify category chip appears in the detail pane before saving
            await expect(page.locator('.detail-pane-chip').filter({ hasText: categoryName })).toBeVisible();
        });

        await test.step('Save and verify the category tag appears in the grid row', async () => {
            // Save — navigates back to folder view
            await page.getByRole('button', { name: 'Save changes' }).click();

            // Verify category tag appears in the grid row
            await expect(page.getByTestId('test-row').filter({ hasText: testName }).locator('.category-tag')).toContainText(categoryName);
        });
    });

    test('should record execution results', async ({ page, request }) => {
        const timestamp = Date.now();
        const rootFolderName = `Root Folder ${timestamp}`;
        const testName = `User Login Test ${timestamp}`;
        const runName = `Smoke Run ${timestamp}`;

        let testCase;
        let runId;
        let statusSelect;

        await test.step('Seed a folder, test, run and result via the API', async () => {
            // Execution results are now recorded in the run detail view, not the
            // library grid. Seed the run + result via the API, then drive the
            // per-result status select in the UI.
            const folderRes = await request.post(`${API_URL}/folders`, { data: { name: rootFolderName, parent_id: null } });
            expect(folderRes.ok()).toBeTruthy();
            const folder = await folderRes.json();

            const testRes = await request.post(`${API_URL}/tests`, { data: { name: testName, folder_id: folder.id } });
            expect(testRes.ok()).toBeTruthy();
            testCase = await testRes.json();

            const runRes = await request.post(`${API_URL}/runs`, { data: { name: runName } });
            expect(runRes.ok()).toBeTruthy();
            runId = (await runRes.json()).id;

            const resultRes = await request.post(`${API_URL}/runs/${runId}/results`, { data: { test_case_id: testCase.id } });
            expect(resultRes.ok()).toBeTruthy();
        });

        await test.step('Open the run detail view', async () => {
            await page.goto(`/runs/run/${runId}`);
            statusSelect = page.getByTestId(`test-status-select-${testCase.id}`);
            await expect(statusSelect).toBeVisible({ timeout: 15000 });
        });

        await test.step('Record a passing result via the status select', async () => {
            await statusSelect.selectOption('PASS');
            await expect(statusSelect).toHaveValue('PASS');
            await expect(page.getByTestId('stats-passed')).toContainText('1');
        });

        await test.step('Record a failing result via the status select', async () => {
            await statusSelect.selectOption('FAIL');
            await expect(statusSelect).toHaveValue('FAIL');
            await expect(page.getByTestId('stats-failed')).toContainText('1');
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
