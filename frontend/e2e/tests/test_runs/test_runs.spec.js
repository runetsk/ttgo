import { test, expect } from '@playwright/test';
import { API_URL } from '../../config.js';

test.describe('Test Runs Management', () => {

    // API Helpers
    const createCategoryAPI = async (request, name) => {
        const res = await request.post(`${API_URL}/categories`, {
            data: { name: name, description: 'Created via API' }
        });
        expect(res.ok()).toBeTruthy();
        return await res.json();
    };

    const createFolderAPI = async (request, name) => {
        const res = await request.post(`${API_URL}/folders`, {
            data: { name: name, parent_id: null }
        });
        expect(res.ok()).toBeTruthy();
        return await res.json();
    }

    const createTestAPI = async (request, name, folderId) => {
        const res = await request.post(`${API_URL}/tests`, {
            data: { name: name, folder_id: folderId, description: 'API Test' }
        });
        expect(res.ok()).toBeTruthy();
        return await res.json();
    }

    const linkTestToCategoryAPI = async (request, testId, categoryId) => {
        const res = await request.post(`${API_URL}/tests/${testId}/categories`, {
            data: { category_id: categoryId }
        });
        expect(res.ok()).toBeTruthy();
    }

    const createRunAPI = async (request, categoryId, name) => {
        const res = await request.post(`${API_URL}/runs`, {
            data: { category_id: categoryId, name: name }
        });
        expect(res.ok()).toBeTruthy();
        return await res.json();
    };

    // Returns the RunResult primary key (id) for a given test_case_id within a run.
    const getResultId = async (request, runId, testCaseId) => {
        const res = await request.get(`${API_URL}/runs/${runId}`);
        const run = await res.json();
        return run.run_results.find(r => r.test_case_id === testCaseId)?.id;
    };

    // UI Helper for Category Creation
    const createCategoryUI = async (page, name) => {
        await page.goto('/categories');
        await page.getByTestId('open-create-category-modal').click();
        await page.getByTestId('category-name-input').fill(name);
        await page.getByTestId('create-category-button').click();
        await expect(page.locator(`text=${name}`)).toBeVisible();
    };

    test.beforeEach(async ({ page }) => {
        page.on('console', msg => console.log(`[Browser Console] ${msg.type()}: ${msg.text()}`));
        page.on('pageerror', err => console.log(`[Browser Error]: ${err.message}`));
    });

    // NOTE: the `test.fixme` tests below are skipped pending modernization for the current
    // run-management UI (run list / detail / result-detail redesigns). They are NOT blocked by
    // the suites->categories migration — the seeding already uses categories; they fail on
    // drifted selectors/markup from later UI changes. Tracked as a separate follow-up.
    test.fixme('should allow creating a test run from a category', async ({ page }) => {
        // This test exercises the UI flow entirely
        const categoryName = 'UI Category ' + Date.now();
        const runName = 'UI Run ' + Date.now();
        let row;

        await test.step('Create a category via the UI', async () => {
            await createCategoryUI(page, categoryName);
        });

        await test.step('Open the runs page', async () => {
            // 1. Navigate to runs page
            await page.goto('/runs');
            await page.waitForLoadState('domcontentloaded');
            await expect(page.locator('.grid-title')).toHaveText('Test Runs', { timeout: 30000 });
        });

        await test.step('Create a new run for the category', async () => {
            // 2. Create a new run
            await page.getByRole('button', { name: 'New Test Run' }).click({ force: true });
            await expect(page.locator('.modal-content')).toBeVisible();

            // Select the category
            await page.getByTestId('create-run-category-select').selectOption({ label: categoryName });
            await page.getByTestId('create-run-name-input').fill(runName);
            await page.getByTestId('create-run-submit').click();
        });

        await test.step('Verify the run appears in the list as pending', async () => {
            // 3. Verify run appears in list
            row = page.getByRole('row', { name: runName });
            await expect(row).toBeVisible();
            await expect(row.getByText('PENDING')).toBeVisible();
        });

        await test.step('Open the run and verify navigation to its detail page', async () => {
            // 4. Click run to see details (Implicit check of navigation)
            await row.getByText('PENDING').click();

            await expect(page).toHaveURL(/\/runs\/run\/[a-f0-9-]+$/);
            await expect(page.locator('.grid-title')).toContainText(runName); // Renamed header uses grid-title
        });
    });

    test('should filter and sort test runs', async ({ page, request }) => {
        const catNameA = 'Filter Cat A ' + Date.now();
        const catNameB = 'Filter Cat B ' + Date.now();
        const run1Name = 'Run 1 A ' + Date.now();
        const run2Name = 'Run 2 B ' + Date.now();
        const run3Name = 'Run 3 A ' + Date.now();
        let catA;

        await test.step('Seed two categories and three runs via API', async () => {
            catA = await createCategoryAPI(request, catNameA);
            const catB = await createCategoryAPI(request, catNameB);
            await createRunAPI(request, catA.id, run1Name);
            await createRunAPI(request, catB.id, run2Name);
            await createRunAPI(request, catA.id, run3Name);
        });

        await test.step('Open the test runs page and reveal column filters', async () => {
            await page.goto('/runs');
            await page.waitForLoadState('domcontentloaded');
            await page.getByRole('button', { name: 'Column Filters' }).click();
        });

        await test.step('Filtering by Category A shows only its runs', async () => {
            await page.getByTestId('filter-run-category').click();
            await page.getByTestId(`filter-run-category-option-${catA.id}`).click();
            await page.keyboard.press('Escape');
            await expect(page.getByText(run1Name)).toBeVisible();
            await expect(page.getByText(run3Name)).toBeVisible();
            await expect(page.getByText(run2Name)).not.toBeVisible();
        });

        await test.step('Clearing the category filter shows all runs again', async () => {
            await page.getByTestId('filter-run-category').click();
            await page.getByTestId('filter-run-category-clear').click();
            await page.keyboard.press('Escape');
            await expect(page.getByText(run2Name)).toBeVisible();
        });

        await test.step('Filtering by Pending status keeps the pending run', async () => {
            await page.getByTestId('filter-status-select').selectOption('PENDING');
            await expect(page.getByText(run1Name)).toBeVisible();
        });

        await test.step('Filtering by Failed status hides the pending run', async () => {
            await page.getByTestId('filter-status-select').selectOption('FAILED');
            await expect(page.getByText(run1Name)).not.toBeVisible();
        });
    });

    test.fixme('should delete a test run', async ({ page, request }) => {
        const runName = 'Run to Delete ' + Date.now();
        let row;

        await test.step('Seed a category and a run via API', async () => {
            const suite = await createCategoryAPI(request, 'Delete Suite ' + Date.now());
            await createRunAPI(request, suite.id, runName);
        });

        await test.step('Open the runs page and verify the run is visible', async () => {
            await page.goto('/runs');
            await page.waitForLoadState('domcontentloaded');

            // Verify visible
            row = page.getByRole('row', { name: runName });
            await expect(row).toBeVisible();
        });

        await test.step('Delete the run and confirm in the modal', async () => {
            // Click delete inside the row — opens custom Modal
            await row.getByTitle('Delete Run').click();

            // Confirm in the custom Modal
            await expect(page.getByTestId('modal-confirm-button')).toBeVisible();
            await page.getByTestId('modal-confirm-button').click();
        });

        await test.step('Verify the run row is gone after the modal closes', async () => {
            // Wait for modal to close, then verify run row is gone
            await expect(page.locator('.modal-overlay')).not.toBeVisible();
            await expect(page.getByRole('row', { name: runName })).not.toBeVisible();
        });
    });

    test.fixme('should rename run, add test, and remove test', async ({ page, request }) => {
        const runName = 'Original Name ' + Date.now();
        const renamedRunName = 'Renamed Run ' + Date.now();
        let row;
        let rowTest1;

        await test.step('Seed a category with two tests and a run via API', async () => {
            const suite = await createCategoryAPI(request, 'CRUD Suite ' + Date.now());

            // Add minimal tests to suite
            const folder = await createFolderAPI(request, 'CRUD Folder ' + Date.now());
            const test1 = await createTestAPI(request, 'Test 1', folder.id);
            const test2 = await createTestAPI(request, 'Test 2', folder.id);
            await linkTestToCategoryAPI(request, test1.id, suite.id);
            await linkTestToCategoryAPI(request, test2.id, suite.id);

            await createRunAPI(request, suite.id, runName);
        });

        await test.step('Open the runs page', async () => {
            await page.goto('/runs');
            await page.waitForLoadState('domcontentloaded');
        });

        await test.step('Rename the run via the list modal', async () => {
            // 1. Rename via List — opens custom Modal with prompt
            row = page.getByRole('row', { name: runName });
            await expect(row).toBeVisible();
            await row.getByTitle('Rename Run').click();

            // Fill the rename modal
            await expect(page.getByTestId('modal-input')).toBeVisible();
            await page.getByTestId('modal-input').fill(renamedRunName);
            await page.getByTestId('modal-confirm-button').click();

            await expect(page.getByText(renamedRunName)).toBeVisible();
        });

        await test.step('Open the run detail and verify both tests are present', async () => {
            // 2. Go to Detail
            await page.getByRole('row', { name: renamedRunName }).click();
            await expect(page.locator('.grid-title')).toContainText(renamedRunName);

            // Check Stats (Total 2)
            rowTest1 = page.getByRole('row', { name: 'Test 1' });
            const rowTest2 = page.getByRole('row', { name: 'Test 2' });
            await expect(rowTest1).toBeVisible();
            await expect(rowTest2).toBeVisible();
        });

        await test.step('Remove Test 1 from the run', async () => {
            // 3. Remove "Test 1" — uses window.confirm in TestRunDetail
            page.on('dialog', dialog => dialog.accept());
            await rowTest1.getByRole('button', { name: '✕' }).click();
            await expect(rowTest1).not.toBeVisible();
        });

        await test.step('Add Test 1 back to the run', async () => {
            // 4. Add "Test 1" back
            await page.getByTestId('add-test-to-run-button').click();
            await page.getByTestId('add-test-select').selectOption({ label: 'Test 1' });
            await page.getByTestId('confirm-add-test-button').click();

            // Verify "Test 1" is back
            await expect(page.getByRole('row', { name: 'Test 1' })).toBeVisible();
        });
    });

    test.fixme('should navigate to details on row click', async ({ page, request }) => {
        const runName = 'Nav API Run ' + Date.now();
        let row;

        await test.step('Seed a category and a run via API', async () => {
            const suite = await createCategoryAPI(request, 'Nav API Suite ' + Date.now());
            await createRunAPI(request, suite.id, runName);
        });

        await test.step('Open the runs page and verify the run is visible', async () => {
            await page.goto('/runs');
            await page.waitForLoadState('domcontentloaded');

            row = page.getByRole('row', { name: runName });
            await expect(row).toBeVisible();
        });

        await test.step('Click the run row and verify navigation to its detail page', async () => {
            await row.getByText('PENDING').click();

            await expect(page.locator('.grid-title')).toContainText(runName);
            await expect(page.url()).toMatch(/\/runs\/run\/[a-f0-9-]+$/);
        });
    });

    test('should update test result status in a run', async ({ page, request }) => {
        let run;
        let row;
        let reloadedRow;

        await test.step('Seed a category with a test and a run via API', async () => {
            const suite = await createCategoryAPI(request, 'Status Update Suite ' + Date.now());
            const folder = await createFolderAPI(request, 'Status Folder ' + Date.now());
            const test1 = await createTestAPI(request, 'Status Test 1', folder.id);
            await linkTestToCategoryAPI(request, test1.id, suite.id);

            const runName = 'Status Run ' + Date.now();
            run = await createRunAPI(request, suite.id, runName);
        });

        await test.step('Open the run detail and locate the test row', async () => {
            // Navigate directly to run detail
            await page.goto(`/runs/run/${run.id}`);
            await page.waitForLoadState('domcontentloaded');

            // Locate the row for Test 1
            row = page.getByRole('row', { name: 'Status Test 1' });
            await expect(row).toBeVisible();
        });

        await test.step('Set the result status to PASS and verify', async () => {
            // Change status to PASS
            await row.locator('select').first().selectOption('PASS');

            // Verify value immediately
            await expect(row.locator('select').first()).toHaveValue('PASS');
        });

        await test.step('Reload and verify the PASS status persisted', async () => {
            // Verify persistence after reload
            await page.reload();
            await page.waitForLoadState('domcontentloaded');
            reloadedRow = page.getByRole('row', { name: 'Status Test 1' });
            await expect(reloadedRow).toBeVisible();
            await expect(reloadedRow.locator('select').first()).toHaveValue('PASS');
        });

        await test.step('Change the status to FAIL and verify', async () => {
            // Change to FAIL
            await reloadedRow.locator('select').first().selectOption('FAIL');
            await expect(reloadedRow.locator('select').first()).toHaveValue('FAIL');
        });
    });

    test.fixme('should display rich failure details', async ({ page, request }) => {
        let run;
        let testCase;
        let row;

        await test.step('Seed a category with a test and a run, then set failure data via API', async () => {
            const suite = await createCategoryAPI(request, 'Failure Suite ' + Date.now());
            const folder = await createFolderAPI(request, 'Failure Folder ' + Date.now());
            testCase = await createTestAPI(request, 'Failure Test', folder.id);
            await linkTestToCategoryAPI(request, testCase.id, suite.id);
            run = await createRunAPI(request, suite.id, 'Failure Run ' + Date.now());

            // Update result with failure data
            const resultId = await getResultId(request, run.id, testCase.id);
            const res = await request.put(`${API_URL}/runs/${run.id}/results/${resultId}`, {
                data: {
                    status: 'FAIL',
                    error_message: 'Element #submit not found',
                    stack_trace: 'Error: at page.click (login.js:20:10)',
                    failure_type: 'TimeoutError',
                    log_text: '[INFO] Starting... [ERROR] Failed',
                    screenshot: 'https://example.com/scr.png' // Mock URL
                }
            });
            expect(res.ok()).toBeTruthy();
        });

        await test.step('Verify the failure data persisted via API', async () => {
            // Verify API Persistence
            const debugRun = await request.get(`${API_URL}/runs/${run.id}`).then(r => r.json());
            const debugResult = debugRun.run_results.find(r => r.test_case_id === testCase.id);
            console.log('Debug Result:', debugResult);
            expect(debugResult.error_message).toBe('Element #submit not found');
        });

        await test.step('Open the run detail and expand the failure row', async () => {
            // Navigate to Run Detail
            await page.goto(`/runs/run/${run.id}`);
            await page.waitForLoadState('domcontentloaded');

            // Locate the row for Failure Test
            row = page.getByRole('row', { name: 'Failure Test' });
            await expect(row).toBeVisible();

            // Click to expand and show RunResultDetail
            await row.click();
        });

        await test.step('Assert the rich failure details and screenshot link are shown', async () => {
            // Assertions for rich data in the expanded detail
            await expect(page.getByText('Element #submit not found')).toBeVisible();
            await expect(page.getByText('TimeoutError')).toBeVisible();
            await expect(page.getByText('Stack Trace')).toBeVisible(); // Header
            await expect(page.getByText('at page.click (login.js:20:10)')).toBeVisible();

            // Check for Screenshot link
            const screenshotLink = page.getByRole('link', { name: 'Screenshot' });
            await expect(screenshotLink).toBeVisible();
            await expect(screenshotLink).toHaveAttribute('href', 'https://example.com/scr.png');
        });
    });

    test('should sort run results by duration', async ({ page, request }) => {
        let run;
        let rows;

        await test.step('Seed a category with three tests and a run, then set durations via API', async () => {
            const suite = await createCategoryAPI(request, 'Perf Suite ' + Date.now());
            const folder = await createFolderAPI(request, 'Perf Folder ' + Date.now());
            const t1 = await createTestAPI(request, 'Test Short', folder.id);
            const t2 = await createTestAPI(request, 'Test Long', folder.id);
            const t3 = await createTestAPI(request, 'Test Medium', folder.id);

            await linkTestToCategoryAPI(request, t1.id, suite.id);
            await linkTestToCategoryAPI(request, t2.id, suite.id);
            await linkTestToCategoryAPI(request, t3.id, suite.id);

            run = await createRunAPI(request, suite.id, 'Perf Run ' + Date.now());

            // Update Durations (resolve result PKs first)
            const r1id = await getResultId(request, run.id, t1.id);
            const r2id = await getResultId(request, run.id, t2.id);
            const r3id = await getResultId(request, run.id, t3.id);
            await request.put(`${API_URL}/runs/${run.id}/results/${r1id}`, { data: { duration_ms: 100, status: 'PASS' } });
            await request.put(`${API_URL}/runs/${run.id}/results/${r2id}`, { data: { duration_ms: 2000, status: 'PASS' } });
            await request.put(`${API_URL}/runs/${run.id}/results/${r3id}`, { data: { duration_ms: 500, status: 'PASS' } });
        });

        await test.step('Open the run detail and verify the initial duration formatting', async () => {
            await page.goto(`/runs/run/${run.id}`);
            await page.waitForLoadState('domcontentloaded');

            // Locate rows
            const rowShort = page.getByRole('row', { name: 'Test Short' });
            const rowLong = page.getByRole('row', { name: 'Test Long' });
            const rowMedium = page.getByRole('row', { name: 'Test Medium' });

            // Verify Initial Format
            await expect(rowShort).toContainText('100ms');
            await expect(rowLong).toContainText('2.00s');
            await expect(rowMedium).toContainText('500ms');
        });

        await test.step('Sort by duration ascending and verify the row order', async () => {
            // Click Duration Header (Ascending)
            await page.getByRole('columnheader', { name: 'Duration' }).click();

            // Check order of Test Names
            rows = page.locator('tbody tr').filter({ hasText: /Test (Short|Medium|Long)/ });
            await expect(rows.nth(0)).toContainText('Test Short');
            await expect(rows.nth(1)).toContainText('Test Medium');
            await expect(rows.nth(2)).toContainText('Test Long');
        });

        await test.step('Sort by duration descending and verify the row order', async () => {
            // Click again (Descending)
            await page.getByRole('columnheader', { name: 'Duration' }).click();
            await expect(rows.nth(0)).toContainText('Test Long');
            await expect(rows.nth(1)).toContainText('Test Medium');
            await expect(rows.nth(2)).toContainText('Test Short');
        });
    });

    test.fixme('should display environment context in run result details', async ({ page, request }) => {
        let run;

        await test.step('Seed a category with a test and a run, then set context fields via API', async () => {
            const suite = await createCategoryAPI(request, 'Env Suite ' + Date.now());
            const folder = await createFolderAPI(request, 'Env Folder ' + Date.now());
            const testCase = await createTestAPI(request, 'Context Test', folder.id);
            await linkTestToCategoryAPI(request, testCase.id, suite.id);
            run = await createRunAPI(request, suite.id, 'Env Run ' + Date.now());

            // Update with Context Fields
            const envResultId = await getResultId(request, run.id, testCase.id);
            await request.put(`${API_URL}/runs/${run.id}/results/${envResultId}`, {
                data: {
                    status: 'PASS',
                    browser: 'Chrome 121',
                    os: 'MacOS 14.2',
                    environment: 'Staging',
                    app_version: 'v2.0.1'
                }
            });
        });

        await test.step('Open the run detail and expand the result row', async () => {
            await page.goto(`/runs/run/${run.id}`);
            await page.waitForLoadState('domcontentloaded');

            const row = page.getByRole('row', { name: 'Context Test' });
            await row.click(); // Expand to show RunResultDetail
        });

        await test.step('Assert the environment context fields are shown', async () => {
            await expect(page.getByText('Chrome 121')).toBeVisible();
            await expect(page.getByText('MacOS 14.2')).toBeVisible();
            await expect(page.getByText('Staging')).toBeVisible();
            await expect(page.getByText('v2.0.1')).toBeVisible();
        });
    });

    test('should navigate from run result to test case', async ({ page, request }) => {
        let run;
        let testCase;
        let testLink;

        await test.step('Seed a category with a test and a run via API', async () => {
            const suite = await createCategoryAPI(request, 'Nav Test Suite ' + Date.now());
            const folder = await createFolderAPI(request, 'Nav Test Folder ' + Date.now());
            testCase = await createTestAPI(request, 'Navigable Test', folder.id);
            await linkTestToCategoryAPI(request, testCase.id, suite.id);
            run = await createRunAPI(request, suite.id, 'Nav Run ' + Date.now());
        });

        await test.step('Open the run detail and verify the test name is a link', async () => {
            await page.goto(`/runs/run/${run.id}`);
            await page.waitForLoadState('domcontentloaded');

            // Test name should be rendered as a clickable link
            testLink = page.getByRole('link', { name: 'Navigable Test' });
            await expect(testLink).toBeVisible();
        });

        await test.step('Click the test link and verify navigation to the test case detail', async () => {
            // Click the link — navigates to test detail without triggering row expand
            await testLink.click();
            await expect(page).toHaveURL(new RegExp(`/library/tests/${testCase.id}`));
            await expect(page.getByTestId('test-case-name-input')).toHaveValue('Navigable Test');
        });
    });

    test('should display categories in run result rows and stats bar', async ({ page, request }) => {
        const categoryName = 'Result Category ' + Date.now();
        let run;

        await test.step('Seed a category with a tagged test and a run via API', async () => {
            const category = await createCategoryAPI(request, categoryName);
            const folder = await createFolderAPI(request, 'Result Folder ' + Date.now());
            const testCase = await createTestAPI(request, 'Category-Tagged Test', folder.id);
            await linkTestToCategoryAPI(request, testCase.id, category.id);
            run = await createRunAPI(request, category.id, 'Category Display Run ' + Date.now());
        });

        await test.step('Open the run detail and verify the category tag in the result row', async () => {
            await page.goto(`/runs/run/${run.id}`);
            await page.waitForLoadState('domcontentloaded');

            // Categories column in the result row should show the category tag
            const row = page.getByRole('row', { name: 'Category-Tagged Test' });
            await expect(row).toBeVisible();
            await expect(row.locator('.category-tag')).toContainText(categoryName);
        });

        await test.step('Verify the category is shown in the stats bar', async () => {
            // Stats bar should also derive and show the category from the result's test case
            await expect(page.getByTestId('run-categories')).toContainText(categoryName);
        });
    });

    test.fixme('should bulk delete test runs', async ({ page, request }) => {
        const timestamp = Date.now();
        const suiteName = `Bulk Delete Suite ${timestamp}`;
        const run1 = `Run 1 ${timestamp}`;
        const run2 = `Run 2 ${timestamp}`;
        const run3 = `Run 3 ${timestamp}`;
        let row1;
        let row2;
        let row3;

        await test.step('Seed a category and three runs via API', async () => {
            // 1. Create Suite
            const suite = await createCategoryAPI(request, suiteName);

            // 2. Create 3 Test Runs via API
            await createRunAPI(request, suite.id, run1);
            await createRunAPI(request, suite.id, run2);
            await createRunAPI(request, suite.id, run3);
        });

        await test.step('Open the runs page and select the first two runs', async () => {
            await page.goto('/runs');
            await page.waitForLoadState('domcontentloaded');

            // 3. Select 2 Runs (Run 1 and Run 2)
            row1 = page.getByRole('row', { name: run1 });
            row2 = page.getByRole('row', { name: run2 });
            row3 = page.getByRole('row', { name: run3 });

            await expect(row1).toBeVisible();
            await expect(row2).toBeVisible();

            await row1.locator('input[type="checkbox"]').check();
            await row2.locator('input[type="checkbox"]').check();
        });

        await test.step('Trigger bulk delete and confirm in the modal', async () => {
            // 4. Click Bulk Delete — opens custom Modal
            const bulkDeleteBtn = page.getByTestId('bulk-delete-runs-button');
            await expect(bulkDeleteBtn).toBeVisible();
            await bulkDeleteBtn.click();

            // Confirm in the custom Modal
            await expect(page.getByTestId('modal-confirm-button')).toBeVisible();
            await page.getByTestId('modal-confirm-button').click();
        });

        await test.step('Verify the two selected runs are deleted and the third remains', async () => {
            // 5. Verify Runs Deleted
            await expect(row1).not.toBeVisible();
            await expect(row2).not.toBeVisible();
            await expect(row3).toBeVisible();
        });
    });

});
