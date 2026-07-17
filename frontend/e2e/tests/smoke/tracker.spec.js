import { test, expect } from '../../fixtures/test.js';
import { TIMEOUTS } from '../../config.js';

test.describe('Test Tracking System E2E', () => {

    test.beforeEach(async ({ libraryPage }) => {
        await libraryPage.open();
    });

    test('should create a root folder and subfolder', async ({ libraryPage }) => {
        const timestamp = Date.now();
        const rootFolderName = `Root Folder ${timestamp}`;
        const subFolderName = `Sub Folder ${timestamp}`;

        await test.step('Create a root folder', async () => {
            const rootFolder = await libraryPage.createRootFolder(rootFolderName);
            await expect(rootFolder).toBeVisible();
        });

        await test.step('Create a subfolder via the context menu', async () => {
            const subFolder = await libraryPage.createSubfolder(rootFolderName, subFolderName);
            await expect(subFolder).toBeVisible();
        });
    });

    test('should create a test case in a folder', async ({ libraryPage }) => {
        const timestamp = Date.now();
        const rootFolderName = `Root Folder ${timestamp}`;
        const testName = `User Login Test ${timestamp}`;

        await test.step('Create a folder and select it', async () => {
            await libraryPage.createRootFolder(rootFolderName);
            await libraryPage.selectFolder(rootFolderName);
        });

        await test.step('Create a test case in the folder', async () => {
            await libraryPage.createTestCase(testName);
        });

        await test.step('Verify the test case appears in the grid', async () => {
            await expect(libraryPage.testRow(testName)).toBeVisible();
        });
    });

    test('should manage suites and assign to test', async ({ page, libraryPage, categoriesPage }) => {
        const timestamp = Date.now();
        const rootFolderName = `Root Folder ${timestamp}`;
        const testName = `User Login Test ${timestamp}`;
        const categoryName = `Regression ${timestamp}`;

        await test.step('Set up a folder and a test case', async () => {
            await libraryPage.createRootFolder(rootFolderName);
            await libraryPage.selectFolder(rootFolderName);
            await libraryPage.createTestCase(testName);
        });

        await test.step('Create a category in the Category Manager', async () => {
            await libraryPage.nav('Quality');
            await libraryPage.nav('Categories');
            await page.waitForURL(/\/categories/);
            await categoriesPage.create(categoryName);
            await expect(page.getByText(categoryName)).toBeVisible();
        });

        await test.step('Return to the library and re-select the folder', async () => {
            await libraryPage.nav('Tests');
            await libraryPage.selectFolder(rootFolderName);
        });

        await test.step('Assign the category to the test via the detail view', async () => {
            // Click the test name (not the row) to navigate without hitting the first-column checkbox.
            await libraryPage.testRow(testName).getByText(testName).click();
            await expect(page.getByTestId('test-case-name-input')).toHaveValue(testName);
            // Selecting from the "+ Category" dropdown adds the category immediately.
            await page.getByTestId('category-select').selectOption({ label: categoryName });
            await expect(page.locator('.detail-pane-chip').filter({ hasText: categoryName })).toBeVisible();
        });

        await test.step('Save and verify the category tag appears in the grid row', async () => {
            await page.getByRole('button', { name: 'Save changes' }).click();
            await expect(libraryPage.testRow(testName).locator('.category-tag')).toContainText(categoryName);
        });
    });

    test('should record execution results', async ({ api, runDetailPage }) => {
        const timestamp = Date.now();
        const rootFolderName = `Root Folder ${timestamp}`;
        const testName = `User Login Test ${timestamp}`;
        const runName = `Smoke Run ${timestamp}`;

        let testCase;
        let runId;
        let statusSelect;

        // Execution results are recorded in the run detail view, not the library
        // grid — seed the run + result via the API, then drive the status select.
        await test.step('Seed a folder, test, run and result via the API', async () => {
            const folder = await api.createFolder(rootFolderName);
            testCase = await api.createTest(testName, folder.id);
            const run = await api.createRun(runName);
            await api.addRunResult(run.id, testCase.id);
            runId = run.id;
        });

        await test.step('Open the run detail view', async () => {
            await runDetailPage.open(runId);
            statusSelect = runDetailPage.statusSelect(testCase.id);
            await expect(statusSelect).toBeVisible({ timeout: TIMEOUTS.APP_RENDER });
        });

        await test.step('Record a passing result via the status select', async () => {
            await statusSelect.selectOption('PASS');
            await expect(statusSelect).toHaveValue('PASS');
            await expect(runDetailPage.stat('passed')).toContainText('1');
        });

        await test.step('Record a failing result via the status select', async () => {
            await statusSelect.selectOption('FAIL');
            await expect(statusSelect).toHaveValue('FAIL');
            await expect(runDetailPage.stat('failed')).toContainText('1');
        });
    });

    test('should support multiple folder selection', async ({ page, libraryPage }) => {
        const timestamp = Date.now();
        const f1 = `F1 ${timestamp}`;
        const f2 = `F2 ${timestamp}`;

        let node1;

        await test.step('Create two root folders', async () => {
            await libraryPage.createRootFolder(f1);
            await libraryPage.createRootFolder(f2);
        });

        await test.step('Select the first folder', async () => {
            node1 = libraryPage.folderNode(f1);
            await node1.click();
            await expect(node1).toHaveClass(/selected/);
        });

        await test.step('Ctrl-select the second folder and verify both are selected', async () => {
            const node2 = libraryPage.folderNode(f2);
            await node2.click({ modifiers: ['ControlOrMeta'] });
            await expect(node1).toHaveClass(/selected/);
            await expect(node2).toHaveClass(/selected/);
        });

        await test.step('Verify the bulk action UI appears', async () => {
            await expect(page.getByText('Delete (2)')).toBeVisible();
        });
    });
});
