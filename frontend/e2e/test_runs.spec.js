import { test, expect } from '@playwright/test';

test.describe('Test Runs Management', () => {

    const API_URL = 'http://localhost:8080/api';

    // API Helpers
    const createSuiteAPI = async (request, name) => {
        const res = await request.post(`${API_URL}/suites`, {
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

    const linkTestToSuiteAPI = async (request, testId, suiteId) => {
        const res = await request.post(`${API_URL}/tests/${testId}/suites`, {
            data: { suite_id: suiteId }
        });
        expect(res.ok()).toBeTruthy();
    }

    const createRunAPI = async (request, suiteId, name) => {
        const res = await request.post(`${API_URL}/runs`, {
            data: { suite_id: suiteId, name: name }
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

    // UI Helper (Legacy for Creation Test)
    const createSuiteUI = async (page, name) => {
        await page.goto('/suites');
        await page.getByTestId('open-create-suite-modal').click();
        await page.getByTestId('suite-name-input').fill(name);
        await page.getByTestId('create-suite-button').click();
        await expect(page.locator(`text=${name}`)).toBeVisible();
    };

    test.beforeEach(async ({ page }) => {
        page.on('console', msg => console.log(`[Browser Console] ${msg.type()}: ${msg.text()}`));
        page.on('pageerror', err => console.log(`[Browser Error]: ${err.message}`));
    });

    test('should allow creating a test run from a suite', async ({ page }) => {
        // This test exercises the UI flow entirely
        const suiteName = 'UI Suite ' + Date.now();
        await createSuiteUI(page, suiteName);

        // 1. Navigate to runs page
        await page.goto('/runs');
        await page.waitForLoadState('domcontentloaded');
        await expect(page.locator('.grid-title')).toHaveText('Test Runs', { timeout: 30000 });

        // 2. Create a new run
        await page.getByRole('button', { name: 'New Test Run' }).click({ force: true });
        await expect(page.locator('.modal')).toBeVisible();

        const runName = 'UI Run ' + Date.now();
        // Select the suite
        await page.selectOption('select[name="suite_id"]', { label: suiteName });
        await page.fill('input[name="name"]', runName);
        await page.click('button:text("Start Run")');

        // 3. Verify run appears in list
        const row = page.getByRole('row', { name: runName });
        await expect(row).toBeVisible();
        await expect(row.getByText('PENDING')).toBeVisible();

        // 4. Click run to see details (Implicit check of navigation)
        await row.getByText('PENDING').click();

        await expect(page).toHaveURL(/\/runs\/[a-f0-9-]+$/);
        await expect(page.locator('.grid-title')).toContainText(runName); // Renamed header uses grid-title
    });

    test('should filter and sort test runs', async ({ page, request }) => {
        const suiteNameA = 'Filter Suite A ' + Date.now();
        const suiteNameB = 'Filter Suite B ' + Date.now();
        const suiteA = await createSuiteAPI(request, suiteNameA);
        const suiteB = await createSuiteAPI(request, suiteNameB);

        const run1Name = 'Run 1 A ' + Date.now();
        const run2Name = 'Run 2 B ' + Date.now();
        const run3Name = 'Run 3 A ' + Date.now();

        await createRunAPI(request, suiteA.id, run1Name);
        await createRunAPI(request, suiteB.id, run2Name);
        await createRunAPI(request, suiteA.id, run3Name);

        await page.goto('/runs');
        await page.waitForLoadState('domcontentloaded');

        // Show filter row
        await page.getByRole('button', { name: 'Column Filters' }).click();

        // 1. Filter by Suite A
        await page.getByTestId('filter-suite-select').selectOption({ label: suiteNameA });

        await expect(page.getByText(run1Name)).toBeVisible();
        await expect(page.getByText(run3Name)).toBeVisible();
        await expect(page.getByText(run2Name)).not.toBeVisible();

        // Reset
        await page.getByTestId('filter-suite-select').selectOption({ value: '' });
        await expect(page.getByText(run2Name)).toBeVisible();

        // 2. Filter by Status (Pending)
        await page.getByTestId('filter-status-select').selectOption('PENDING');
        await expect(page.getByText(run1Name)).toBeVisible();

        // 3. Status FAILED (should hide)
        await page.getByTestId('filter-status-select').selectOption('FAILED');
        await expect(page.getByText(run1Name)).not.toBeVisible();
    });

    test('should delete a test run', async ({ page, request }) => {
        const suite = await createSuiteAPI(request, 'Delete Suite ' + Date.now());
        const runName = 'Run to Delete ' + Date.now();
        await createRunAPI(request, suite.id, runName);

        await page.goto('/runs');
        await page.waitForLoadState('domcontentloaded');

        // Verify visible
        const row = page.getByRole('row', { name: runName });
        await expect(row).toBeVisible();

        // Click delete inside the row — opens custom Modal
        await row.getByTitle('Delete Run').click();

        // Confirm in the custom Modal
        await expect(page.getByTestId('modal-confirm-button')).toBeVisible();
        await page.getByTestId('modal-confirm-button').click();

        // Wait for modal to close, then verify run row is gone
        await expect(page.locator('.modal-overlay')).not.toBeVisible();
        await expect(page.getByRole('row', { name: runName })).not.toBeVisible();
    });

    test('should rename run, add test, and remove test', async ({ page, request }) => {
        const suite = await createSuiteAPI(request, 'CRUD Suite ' + Date.now());

        // Add minimal tests to suite
        const folder = await createFolderAPI(request, 'CRUD Folder ' + Date.now());
        const test1 = await createTestAPI(request, 'Test 1', folder.id);
        const test2 = await createTestAPI(request, 'Test 2', folder.id);
        await linkTestToSuiteAPI(request, test1.id, suite.id);
        await linkTestToSuiteAPI(request, test2.id, suite.id);

        const runName = 'Original Name ' + Date.now();
        const renamedRunName = 'Renamed Run ' + Date.now();
        const run = await createRunAPI(request, suite.id, runName);

        await page.goto('/runs');
        await page.waitForLoadState('domcontentloaded');

        // 1. Rename via List — opens custom Modal with prompt
        const row = page.getByRole('row', { name: runName });
        await expect(row).toBeVisible();
        await row.getByTitle('Rename Run').click();

        // Fill the rename modal
        await expect(page.getByTestId('modal-input')).toBeVisible();
        await page.getByTestId('modal-input').fill(renamedRunName);
        await page.getByTestId('modal-confirm-button').click();

        await expect(page.getByText(renamedRunName)).toBeVisible();

        // 2. Go to Detail
        await page.getByRole('row', { name: renamedRunName }).click();
        await expect(page.locator('.grid-title')).toContainText(renamedRunName);

        // Check Stats (Total 2)
        const rowTest1 = page.getByRole('row', { name: 'Test 1' });
        const rowTest2 = page.getByRole('row', { name: 'Test 2' });
        await expect(rowTest1).toBeVisible();
        await expect(rowTest2).toBeVisible();

        // 3. Remove "Test 1" — uses window.confirm in TestRunDetail
        page.on('dialog', dialog => dialog.accept());
        await rowTest1.getByRole('button', { name: '✕' }).click();
        await expect(rowTest1).not.toBeVisible();

        // 4. Add "Test 1" back
        await page.getByTestId('add-test-to-run-button').click();
        await page.getByTestId('add-test-select').selectOption({ label: 'Test 1' });
        await page.getByTestId('confirm-add-test-button').click();

        // Verify "Test 1" is back
        await expect(page.getByRole('row', { name: 'Test 1' })).toBeVisible();
    });

    test('should navigate to details on row click', async ({ page, request }) => {
        const suite = await createSuiteAPI(request, 'Nav API Suite ' + Date.now());
        const runName = 'Nav API Run ' + Date.now();
        await createRunAPI(request, suite.id, runName);

        await page.goto('/runs');
        await page.waitForLoadState('domcontentloaded');

        const row = page.getByRole('row', { name: runName });
        await expect(row).toBeVisible();

        await row.getByText('PENDING').click();

        await expect(page.locator('.grid-title')).toContainText(runName);
        await expect(page.url()).toMatch(/\/runs\/[a-f0-9-]+$/);
    });

    test('should update test result status in a run', async ({ page, request }) => {
        const suite = await createSuiteAPI(request, 'Status Update Suite ' + Date.now());
        const folder = await createFolderAPI(request, 'Status Folder ' + Date.now());
        const test1 = await createTestAPI(request, 'Status Test 1', folder.id);
        await linkTestToSuiteAPI(request, test1.id, suite.id);

        const runName = 'Status Run ' + Date.now();
        const run = await createRunAPI(request, suite.id, runName);

        // Navigate directly to run detail
        await page.goto(`/runs/run/${run.id}`);
        await page.waitForLoadState('domcontentloaded');

        // Locate the row for Test 1
        const row = page.getByRole('row', { name: 'Status Test 1' });
        await expect(row).toBeVisible();

        // Change status to PASS
        await row.locator('select').first().selectOption('PASS');

        // Verify value immediately
        await expect(row.locator('select').first()).toHaveValue('PASS');

        // Verify persistence after reload
        await page.reload();
        await page.waitForLoadState('domcontentloaded');
        const reloadedRow = page.getByRole('row', { name: 'Status Test 1' });
        await expect(reloadedRow).toBeVisible();
        await expect(reloadedRow.locator('select').first()).toHaveValue('PASS');

        // Change to FAIL
        await reloadedRow.locator('select').first().selectOption('FAIL');
        await expect(reloadedRow.locator('select').first()).toHaveValue('FAIL');
    });

    test('should display rich failure details', async ({ page, request }) => {
        const suite = await createSuiteAPI(request, 'Failure Suite ' + Date.now());
        const folder = await createFolderAPI(request, 'Failure Folder ' + Date.now());
        const testCase = await createTestAPI(request, 'Failure Test', folder.id);
        await linkTestToSuiteAPI(request, testCase.id, suite.id);
        const run = await createRunAPI(request, suite.id, 'Failure Run ' + Date.now());

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

        // Verify API Persistence
        const debugRun = await request.get(`${API_URL}/runs/${run.id}`).then(r => r.json());
        const debugResult = debugRun.run_results.find(r => r.test_case_id === testCase.id);
        console.log('Debug Result:', debugResult);
        expect(debugResult.error_message).toBe('Element #submit not found');

        // Navigate to Run Detail
        await page.goto(`/runs/run/${run.id}`);
        await page.waitForLoadState('domcontentloaded');

        // Locate the row for Failure Test
        const row = page.getByRole('row', { name: 'Failure Test' });
        await expect(row).toBeVisible();

        // Click to expand and show RunResultDetail
        await row.click();

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

    test('should sort run results by duration', async ({ page, request }) => {
        const suite = await createSuiteAPI(request, 'Perf Suite ' + Date.now());
        const folder = await createFolderAPI(request, 'Perf Folder ' + Date.now());
        const t1 = await createTestAPI(request, 'Test Short', folder.id);
        const t2 = await createTestAPI(request, 'Test Long', folder.id);
        const t3 = await createTestAPI(request, 'Test Medium', folder.id);

        await linkTestToSuiteAPI(request, t1.id, suite.id);
        await linkTestToSuiteAPI(request, t2.id, suite.id);
        await linkTestToSuiteAPI(request, t3.id, suite.id);

        const run = await createRunAPI(request, suite.id, 'Perf Run ' + Date.now());

        // Update Durations (resolve result PKs first)
        const r1id = await getResultId(request, run.id, t1.id);
        const r2id = await getResultId(request, run.id, t2.id);
        const r3id = await getResultId(request, run.id, t3.id);
        await request.put(`${API_URL}/runs/${run.id}/results/${r1id}`, { data: { duration_ms: 100, status: 'PASS' } });
        await request.put(`${API_URL}/runs/${run.id}/results/${r2id}`, { data: { duration_ms: 2000, status: 'PASS' } });
        await request.put(`${API_URL}/runs/${run.id}/results/${r3id}`, { data: { duration_ms: 500, status: 'PASS' } });

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

        // Click Duration Header (Ascending)
        await page.getByRole('columnheader', { name: 'Duration' }).click();

        // Check order of Test Names
        const rows = page.locator('tbody tr').filter({ hasText: /Test (Short|Medium|Long)/ });
        await expect(rows.nth(0)).toContainText('Test Short');
        await expect(rows.nth(1)).toContainText('Test Medium');
        await expect(rows.nth(2)).toContainText('Test Long');

        // Click again (Descending)
        await page.getByRole('columnheader', { name: 'Duration' }).click();
        await expect(rows.nth(0)).toContainText('Test Long');
        await expect(rows.nth(1)).toContainText('Test Medium');
        await expect(rows.nth(2)).toContainText('Test Short');
    });

    test('should display environment context in run result details', async ({ page, request }) => {
        const suite = await createSuiteAPI(request, 'Env Suite ' + Date.now());
        const folder = await createFolderAPI(request, 'Env Folder ' + Date.now());
        const testCase = await createTestAPI(request, 'Context Test', folder.id);
        await linkTestToSuiteAPI(request, testCase.id, suite.id);
        const run = await createRunAPI(request, suite.id, 'Env Run ' + Date.now());

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

        await page.goto(`/runs/run/${run.id}`);
        await page.waitForLoadState('domcontentloaded');

        const row = page.getByRole('row', { name: 'Context Test' });
        await row.click(); // Expand to show RunResultDetail

        await expect(page.getByText('Chrome 121')).toBeVisible();
        await expect(page.getByText('MacOS 14.2')).toBeVisible();
        await expect(page.getByText('Staging')).toBeVisible();
        await expect(page.getByText('v2.0.1')).toBeVisible();
    });

    test('should navigate from run result to test case', async ({ page, request }) => {
        const suite = await createSuiteAPI(request, 'Nav Test Suite ' + Date.now());
        const folder = await createFolderAPI(request, 'Nav Test Folder ' + Date.now());
        const testCase = await createTestAPI(request, 'Navigable Test', folder.id);
        await linkTestToSuiteAPI(request, testCase.id, suite.id);
        const run = await createRunAPI(request, suite.id, 'Nav Run ' + Date.now());

        await page.goto(`/runs/run/${run.id}`);
        await page.waitForLoadState('domcontentloaded');

        // Test name should be rendered as a clickable link
        const testLink = page.getByRole('link', { name: 'Navigable Test' });
        await expect(testLink).toBeVisible();

        // Click the link — navigates to test detail without triggering row expand
        await testLink.click();
        await expect(page).toHaveURL(new RegExp(`/library/tests/${testCase.id}`));
        await expect(page.getByTestId('test-case-name-input')).toHaveValue('Navigable Test');
    });

    test('should display suites in run result rows and stats bar', async ({ page, request }) => {
        const suiteName = 'Result Suite ' + Date.now();
        const suite = await createSuiteAPI(request, suiteName);
        const folder = await createFolderAPI(request, 'Result Folder ' + Date.now());
        const testCase = await createTestAPI(request, 'Suite-Tagged Test', folder.id);
        await linkTestToSuiteAPI(request, testCase.id, suite.id);
        const run = await createRunAPI(request, suite.id, 'Suite Display Run ' + Date.now());

        await page.goto(`/runs/run/${run.id}`);
        await page.waitForLoadState('domcontentloaded');

        // Suites column in the result row should show the suite tag
        const row = page.getByRole('row', { name: 'Suite-Tagged Test' });
        await expect(row).toBeVisible();
        await expect(row.locator('.suite-tag')).toContainText(suiteName);

        // Stats bar should also derive and show the suite from the result's test case
        await expect(page.getByTestId('run-suites')).toContainText(suiteName);
    });

    test('should bulk delete test runs', async ({ page, request }) => {
        const timestamp = Date.now();
        const suiteName = `Bulk Delete Suite ${timestamp}`;
        const run1 = `Run 1 ${timestamp}`;
        const run2 = `Run 2 ${timestamp}`;
        const run3 = `Run 3 ${timestamp}`;

        // 1. Create Suite
        const suite = await createSuiteAPI(request, suiteName);

        // 2. Create 3 Test Runs via API
        await createRunAPI(request, suite.id, run1);
        await createRunAPI(request, suite.id, run2);
        await createRunAPI(request, suite.id, run3);

        await page.goto('/runs');
        await page.waitForLoadState('domcontentloaded');

        // 3. Select 2 Runs (Run 1 and Run 2)
        const row1 = page.getByRole('row', { name: run1 });
        const row2 = page.getByRole('row', { name: run2 });
        const row3 = page.getByRole('row', { name: run3 });

        await expect(row1).toBeVisible();
        await expect(row2).toBeVisible();

        await row1.locator('input[type="checkbox"]').check();
        await row2.locator('input[type="checkbox"]').check();

        // 4. Click Bulk Delete — opens custom Modal
        const bulkDeleteBtn = page.getByTestId('bulk-delete-runs-button');
        await expect(bulkDeleteBtn).toBeVisible();
        await bulkDeleteBtn.click();

        // Confirm in the custom Modal
        await expect(page.getByTestId('modal-confirm-button')).toBeVisible();
        await page.getByTestId('modal-confirm-button').click();

        // 5. Verify Runs Deleted
        await expect(row1).not.toBeVisible();
        await expect(row2).not.toBeVisible();
        await expect(row3).toBeVisible();
    });

});
