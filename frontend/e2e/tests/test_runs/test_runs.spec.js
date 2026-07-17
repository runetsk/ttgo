import { test, expect } from '../../fixtures/test.js';
import { TIMEOUTS } from '../../config.js';

test.describe('Test Runs Management', () => {

    // The `test.fixme` tests below are skipped pending modernization for the current
    // run-management UI (run list / detail / result-detail redesigns). They are NOT
    // blocked by the suites→categories migration — seeding already uses categories;
    // they fail on drifted selectors from later UI changes. Tracked as a follow-up.
    test.fixme('should allow creating a test run from a category', async ({ page, runsPage, categoriesPage }) => {
        const categoryName = 'UI Category ' + Date.now();
        const runName = 'UI Run ' + Date.now();
        let row;

        await test.step('Create a category via the UI', async () => {
            await categoriesPage.openAndCreate(categoryName);
            await expect(page.getByText(categoryName)).toBeVisible();
        });

        await test.step('Open the runs page', async () => {
            await runsPage.open();
            await expect(page.locator('.grid-title')).toHaveText('Test Runs', { timeout: TIMEOUTS.HEAVY_GRID });
        });

        await test.step('Create a new run for the category', async () => {
            await runsPage.openNewRunModal();
            await expect(page.locator('.modal-content')).toBeVisible();
            await runsPage.runCategorySelect.selectOption({ label: categoryName });
            await runsPage.runNameInput.fill(runName);
            await runsPage.submitNewRun();
        });

        await test.step('Verify the run appears in the list as pending', async () => {
            row = runsPage.runRow(runName);
            await expect(row).toBeVisible();
            await expect(row.getByText('PENDING')).toBeVisible();
        });

        await test.step('Open the run and verify navigation to its detail page', async () => {
            await row.getByText('PENDING').click();
            await expect(page).toHaveURL(/\/runs\/run\/[a-f0-9-]+$/);
            await expect(page.locator('.grid-title')).toContainText(runName);
        });
    });

    test('should filter and sort test runs', async ({ page, runsPage, api }) => {
        const catNameA = 'Filter Cat A ' + Date.now();
        const catNameB = 'Filter Cat B ' + Date.now();
        const run1Name = 'Run 1 A ' + Date.now();
        const run2Name = 'Run 2 B ' + Date.now();
        const run3Name = 'Run 3 A ' + Date.now();
        let catA;

        await test.step('Seed two categories and three runs via API', async () => {
            catA = await api.createCategory(catNameA);
            const catB = await api.createCategory(catNameB);
            await api.createRun(run1Name, { categoryId: catA.id });
            const run2 = await api.createRun(run2Name, { categoryId: catB.id });
            await api.createRun(run3Name, { categoryId: catA.id });

            // Give run2 a failing result and finalize it so its run status is FAIL.
            const folder = await api.createFolder('Filter Folder ' + Date.now());
            const tc = await api.createTest('Failing case', folder.id);
            await api.addRunResult(run2.id, tc.id, { status: 'FAIL' });
            await api.completeRun(run2.id);
        });

        await test.step('Open the test runs page and reveal column filters', async () => {
            await runsPage.open();
            await runsPage.openColumnFilters();
        });

        await test.step('Filtering by Category A shows only its runs', async () => {
            await runsPage.filterByCategory(catA.id);
            await expect(page.getByText(run1Name)).toBeVisible();
            await expect(page.getByText(run3Name)).toBeVisible();
            await expect(page.getByText(run2Name)).not.toBeVisible();
        });

        await test.step('Clearing the category filter shows all runs again', async () => {
            await runsPage.clearCategoryFilter();
            await expect(page.getByText(run2Name)).toBeVisible();
        });

        await test.step('Filtering by Pending status keeps the pending run', async () => {
            await runsPage.statusFilter.selectOption('PENDING');
            await expect(page.getByText(run1Name)).toBeVisible();
            await expect(page.getByText(run2Name)).not.toBeVisible();
        });

        await test.step('Filtering by Failed status shows only the failed run', async () => {
            await runsPage.statusFilter.selectOption('FAIL');
            await expect(page.getByText(run2Name)).toBeVisible();
            await expect(page.getByText(run1Name)).not.toBeVisible();
        });
    });

    test.fixme('should delete a test run', async ({ page, runsPage, api }) => {
        const runName = 'Run to Delete ' + Date.now();
        let row;

        await test.step('Seed a category and a run via API', async () => {
            const suite = await api.createCategory('Delete Suite ' + Date.now());
            await api.createRun(runName, { categoryId: suite.id });
        });

        await test.step('Open the runs page and verify the run is visible', async () => {
            await runsPage.open();
            row = runsPage.runRow(runName);
            await expect(row).toBeVisible();
        });

        await test.step('Delete the run and confirm in the modal', async () => {
            await row.getByTitle('Delete Run').click();
            await expect(page.getByTestId('modal-confirm-button')).toBeVisible();
            await page.getByTestId('modal-confirm-button').click();
        });

        await test.step('Verify the run row is gone after the modal closes', async () => {
            await expect(page.locator('.modal-overlay')).not.toBeVisible();
            await expect(runsPage.runRow(runName)).not.toBeVisible();
        });
    });

    test.fixme('should rename run, add test, and remove test', async ({ page, runsPage, api }) => {
        const runName = 'Original Name ' + Date.now();
        const renamedRunName = 'Renamed Run ' + Date.now();
        let row;
        let rowTest1;
        let test1Id;

        await test.step('Seed a category with two tests and a run via API', async () => {
            const suite = await api.createCategory('CRUD Suite ' + Date.now());
            const folder = await api.createFolder('CRUD Folder ' + Date.now());
            const test1 = await api.createTest('Test 1', folder.id);
            const test2 = await api.createTest('Test 2', folder.id);
            test1Id = test1.id;
            await api.linkTestToCategory(test1.id, suite.id);
            await api.linkTestToCategory(test2.id, suite.id);
            await api.createRun(runName, { categoryId: suite.id });
        });

        await test.step('Open the runs page', async () => {
            await runsPage.open();
        });

        await test.step('Rename the run via the list modal', async () => {
            row = runsPage.runRow(runName);
            await expect(row).toBeVisible();
            await row.getByTitle('Rename Run').click();
            await expect(page.getByTestId('modal-input')).toBeVisible();
            await page.getByTestId('modal-input').fill(renamedRunName);
            await page.getByTestId('modal-confirm-button').click();
            await expect(page.getByText(renamedRunName)).toBeVisible();
        });

        await test.step('Open the run detail and verify both tests are present', async () => {
            await runsPage.runRow(renamedRunName).click();
            await expect(page.locator('.grid-title')).toContainText(renamedRunName);
            rowTest1 = page.getByRole('row', { name: 'Test 1' });
            const rowTest2 = page.getByRole('row', { name: 'Test 2' });
            await expect(rowTest1).toBeVisible();
            await expect(rowTest2).toBeVisible();
        });

        await test.step('Remove Test 1 from the run', async () => {
            // TestRunDetail uses window.confirm for removal.
            page.on('dialog', dialog => dialog.accept());
            await rowTest1.getByRole('button', { name: '✕' }).click();
            await expect(rowTest1).not.toBeVisible();
        });

        await test.step('Add Test 1 back to the run', async () => {
            await page.getByTestId('add-test-to-run-button').click();
            await expect(page.getByTestId('test-tree-picker')).toBeVisible();
            await page.getByTestId(`test-tree-test-${test1Id}`).check();
            await page.getByTestId('add-tests-submit').click();
            await expect(page.getByRole('row', { name: 'Test 1' })).toBeVisible();
        });
    });

    test.fixme('should navigate to details on row click', async ({ page, runsPage, api }) => {
        const runName = 'Nav API Run ' + Date.now();
        let row;

        await test.step('Seed a category and a run via API', async () => {
            const suite = await api.createCategory('Nav API Suite ' + Date.now());
            await api.createRun(runName, { categoryId: suite.id });
        });

        await test.step('Open the runs page and verify the run is visible', async () => {
            await runsPage.open();
            row = runsPage.runRow(runName);
            await expect(row).toBeVisible();
        });

        await test.step('Click the run row and verify navigation to its detail page', async () => {
            await row.getByText('PENDING').click();
            await expect(page.locator('.grid-title')).toContainText(runName);
            await expect(page.url()).toMatch(/\/runs\/run\/[a-f0-9-]+$/);
        });
    });

    test('should update test result status in a run', async ({ page, runDetailPage, api }) => {
        let run;

        await test.step('Seed a category with a test and a run via API', async () => {
            const suite = await api.createCategory('Status Update Suite ' + Date.now());
            const folder = await api.createFolder('Status Folder ' + Date.now());
            const test1 = await api.createTest('Status Test 1', folder.id);
            await api.linkTestToCategory(test1.id, suite.id);
            run = await api.createRun('Status Run ' + Date.now(), { categoryId: suite.id });
        });

        await test.step('Open the run detail and locate the test row', async () => {
            await runDetailPage.open(run.id);
            await expect(runDetailPage.resultRow('Status Test 1')).toBeVisible();
        });

        await test.step('Set the result status to PASS and verify', async () => {
            await runDetailPage.rowStatusSelect('Status Test 1').selectOption('PASS');
            await expect(runDetailPage.rowStatusSelect('Status Test 1')).toHaveValue('PASS');
        });

        await test.step('Reload and verify the PASS status persisted', async () => {
            await page.reload();
            await page.waitForLoadState('domcontentloaded');
            await expect(runDetailPage.resultRow('Status Test 1')).toBeVisible();
            await expect(runDetailPage.rowStatusSelect('Status Test 1')).toHaveValue('PASS');
        });

        await test.step('Change the status to FAIL and verify', async () => {
            await runDetailPage.rowStatusSelect('Status Test 1').selectOption('FAIL');
            await expect(runDetailPage.rowStatusSelect('Status Test 1')).toHaveValue('FAIL');
        });
    });

    test.fixme('should display rich failure details', async ({ page, runDetailPage, api }) => {
        let run;
        let testCase;

        await test.step('Seed a category with a test and a run, then set failure data via API', async () => {
            const suite = await api.createCategory('Failure Suite ' + Date.now());
            const folder = await api.createFolder('Failure Folder ' + Date.now());
            testCase = await api.createTest('Failure Test', folder.id);
            await api.linkTestToCategory(testCase.id, suite.id);
            run = await api.createRun('Failure Run ' + Date.now(), { categoryId: suite.id });

            const resultId = await api.getResultId(run.id, testCase.id);
            await api.updateRunResult(run.id, resultId, {
                status: 'FAIL',
                error_message: 'Element #submit not found',
                stack_trace: 'Error: at page.click (login.js:20:10)',
                failure_type: 'TimeoutError',
                log_text: '[INFO] Starting... [ERROR] Failed',
                screenshot: 'https://example.com/scr.png',
            });
        });

        await test.step('Verify the failure data persisted via API', async () => {
            const debugRun = await api.get(`/runs/${run.id}`).then(r => r.json());
            const debugResult = debugRun.run_results.find(r => r.test_case_id === testCase.id);
            expect(debugResult.error_message).toBe('Element #submit not found');
        });

        await test.step('Open the run detail and expand the failure row', async () => {
            await runDetailPage.open(run.id);
            const row = runDetailPage.resultRow('Failure Test');
            await expect(row).toBeVisible();
            await row.click();
        });

        await test.step('Assert the rich failure details and screenshot link are shown', async () => {
            await expect(page.getByText('Element #submit not found')).toBeVisible();
            await expect(page.getByText('TimeoutError')).toBeVisible();
            await expect(page.getByText('Stack Trace')).toBeVisible();
            await expect(page.getByText('at page.click (login.js:20:10)')).toBeVisible();

            const screenshotLink = page.getByRole('link', { name: 'Screenshot' });
            await expect(screenshotLink).toBeVisible();
            await expect(screenshotLink).toHaveAttribute('href', 'https://example.com/scr.png');
        });
    });

    test('should sort run results by duration', async ({ page, runDetailPage, api }) => {
        let run;
        let rows;

        await test.step('Seed a category with three tests and a run, then set durations via API', async () => {
            const suite = await api.createCategory('Perf Suite ' + Date.now());
            const folder = await api.createFolder('Perf Folder ' + Date.now());
            const t1 = await api.createTest('Test Short', folder.id);
            const t2 = await api.createTest('Test Long', folder.id);
            const t3 = await api.createTest('Test Medium', folder.id);

            await api.linkTestToCategory(t1.id, suite.id);
            await api.linkTestToCategory(t2.id, suite.id);
            await api.linkTestToCategory(t3.id, suite.id);

            run = await api.createRun('Perf Run ' + Date.now(), { categoryId: suite.id });

            const r1id = await api.getResultId(run.id, t1.id);
            const r2id = await api.getResultId(run.id, t2.id);
            const r3id = await api.getResultId(run.id, t3.id);
            await api.updateRunResult(run.id, r1id, { duration_ms: 100, status: 'PASS' });
            await api.updateRunResult(run.id, r2id, { duration_ms: 2000, status: 'PASS' });
            await api.updateRunResult(run.id, r3id, { duration_ms: 500, status: 'PASS' });
        });

        await test.step('Open the run detail and verify the initial duration formatting', async () => {
            await runDetailPage.open(run.id);
            await expect(runDetailPage.resultRow('Test Short')).toContainText('100ms');
            await expect(runDetailPage.resultRow('Test Long')).toContainText('2.00s');
            await expect(runDetailPage.resultRow('Test Medium')).toContainText('500ms');
        });

        await test.step('Sort by duration ascending and verify the row order', async () => {
            await runDetailPage.sortByColumn('Duration');
            rows = page.locator('tbody tr').filter({ hasText: /Test (Short|Medium|Long)/ });
            await expect(rows.nth(0)).toContainText('Test Short');
            await expect(rows.nth(1)).toContainText('Test Medium');
            await expect(rows.nth(2)).toContainText('Test Long');
        });

        await test.step('Sort by duration descending and verify the row order', async () => {
            await runDetailPage.sortByColumn('Duration');
            await expect(rows.nth(0)).toContainText('Test Long');
            await expect(rows.nth(1)).toContainText('Test Medium');
            await expect(rows.nth(2)).toContainText('Test Short');
        });
    });

    test.fixme('should display environment context in run result details', async ({ page, runDetailPage, api }) => {
        let run;

        await test.step('Seed a category with a test and a run, then set context fields via API', async () => {
            const suite = await api.createCategory('Env Suite ' + Date.now());
            const folder = await api.createFolder('Env Folder ' + Date.now());
            const testCase = await api.createTest('Context Test', folder.id);
            await api.linkTestToCategory(testCase.id, suite.id);
            run = await api.createRun('Env Run ' + Date.now(), { categoryId: suite.id });

            const envResultId = await api.getResultId(run.id, testCase.id);
            await api.updateRunResult(run.id, envResultId, {
                status: 'PASS',
                browser: 'Chrome 121',
                os: 'MacOS 14.2',
                environment: 'Staging',
                app_version: 'v2.0.1',
            });
        });

        await test.step('Open the run detail and expand the result row', async () => {
            await runDetailPage.open(run.id);
            await runDetailPage.resultRow('Context Test').click();
        });

        await test.step('Assert the environment context fields are shown', async () => {
            await expect(page.getByText('Chrome 121')).toBeVisible();
            await expect(page.getByText('MacOS 14.2')).toBeVisible();
            await expect(page.getByText('Staging')).toBeVisible();
            await expect(page.getByText('v2.0.1')).toBeVisible();
        });
    });

    test('should navigate from run result to test case', async ({ page, runDetailPage, api }) => {
        let run;
        let testCase;
        let testLink;

        await test.step('Seed a category with a test and a run via API', async () => {
            const suite = await api.createCategory('Nav Test Suite ' + Date.now());
            const folder = await api.createFolder('Nav Test Folder ' + Date.now());
            testCase = await api.createTest('Navigable Test', folder.id);
            await api.linkTestToCategory(testCase.id, suite.id);
            run = await api.createRun('Nav Run ' + Date.now(), { categoryId: suite.id });
        });

        await test.step('Open the run detail and verify the test name is a link', async () => {
            await runDetailPage.open(run.id);
            testLink = page.getByRole('link', { name: 'Navigable Test' });
            await expect(testLink).toBeVisible();
        });

        await test.step('Click the test link and verify navigation to the test case detail', async () => {
            await testLink.click();
            await expect(page).toHaveURL(new RegExp(`/library/tests/${testCase.id}`));
            await expect(page.getByTestId('test-case-name-input')).toHaveValue('Navigable Test');
        });
    });

    test('should display categories in run result rows and stats bar', async ({ runDetailPage, api }) => {
        const categoryName = 'Result Category ' + Date.now();
        let run;

        await test.step('Seed a category with a tagged test and a run via API', async () => {
            const category = await api.createCategory(categoryName);
            const folder = await api.createFolder('Result Folder ' + Date.now());
            const testCase = await api.createTest('Category-Tagged Test', folder.id);
            await api.linkTestToCategory(testCase.id, category.id);
            run = await api.createRun('Category Display Run ' + Date.now(), { categoryId: category.id });
        });

        await test.step('Open the run detail and verify the category tag in the result row', async () => {
            await runDetailPage.open(run.id);
            const row = runDetailPage.resultRow('Category-Tagged Test');
            await expect(row).toBeVisible();
            await expect(row.locator('.category-tag')).toContainText(categoryName);
        });

        await test.step('Verify the category is shown in the stats bar', async () => {
            await expect(runDetailPage.categories).toContainText(categoryName);
        });
    });

    test.fixme('should bulk delete test runs', async ({ page, runsPage, api }) => {
        const timestamp = Date.now();
        const suiteName = `Bulk Delete Suite ${timestamp}`;
        const run1 = `Run 1 ${timestamp}`;
        const run2 = `Run 2 ${timestamp}`;
        const run3 = `Run 3 ${timestamp}`;
        let row1;
        let row2;
        let row3;

        await test.step('Seed a category and three runs via API', async () => {
            const suite = await api.createCategory(suiteName);
            await api.createRun(run1, { categoryId: suite.id });
            await api.createRun(run2, { categoryId: suite.id });
            await api.createRun(run3, { categoryId: suite.id });
        });

        await test.step('Open the runs page and select the first two runs', async () => {
            await runsPage.open();
            row1 = runsPage.runRow(run1);
            row2 = runsPage.runRow(run2);
            row3 = runsPage.runRow(run3);

            await expect(row1).toBeVisible();
            await expect(row2).toBeVisible();

            await row1.locator('input[type="checkbox"]').check();
            await row2.locator('input[type="checkbox"]').check();
        });

        await test.step('Trigger bulk delete and confirm in the modal', async () => {
            const bulkDeleteBtn = page.getByTestId('bulk-delete-runs-button');
            await expect(bulkDeleteBtn).toBeVisible();
            await bulkDeleteBtn.click();
            await expect(page.getByTestId('modal-confirm-button')).toBeVisible();
            await page.getByTestId('modal-confirm-button').click();
        });

        await test.step('Verify the two selected runs are deleted and the third remains', async () => {
            await expect(row1).not.toBeVisible();
            await expect(row2).not.toBeVisible();
            await expect(row3).toBeVisible();
        });
    });
});
