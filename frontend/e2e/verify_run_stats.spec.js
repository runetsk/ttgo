import { test, expect } from '@playwright/test';

const API_URL = 'http://localhost:8080/api';

test.describe('Test Run Stats Verification', () => {
    let runId;
    let suiteId;
    let folderId;
    let testIds = [];

    test.beforeAll(async ({ request }) => {
        const timestamp = Date.now();

        // 1. Create a unique suite for isolation
        const suiteRes = await request.post(`${API_URL}/suites`, {
            data: { name: `Stats Suite ${timestamp}`, description: 'E2E stats test' }
        });
        expect(suiteRes.ok()).toBeTruthy();
        const suite = await suiteRes.json();
        suiteId = suite.id;

        // 2. Create a folder
        const folderRes = await request.post(`${API_URL}/folders`, {
            data: { name: `Stats Folder ${timestamp}`, parent_id: null }
        });
        expect(folderRes.ok()).toBeTruthy();
        const folder = await folderRes.json();
        folderId = folder.id;

        // 3. Create a test run
        const runRes = await request.post(`${API_URL}/runs`, {
            data: { suite_id: suiteId, name: `Stats Verify Run ${timestamp}` }
        });
        expect(runRes.ok()).toBeTruthy();
        const run = await runRes.json();
        runId = run.id;

        // 4. Create 3 tests and add as run results
        testIds = [];
        const resultIds = [];
        for (let i = 0; i < 3; i++) {
            const testRes = await request.post(`${API_URL}/tests`, {
                data: { name: `Stats Test ${i} ${timestamp}`, folder_id: folderId, description: 'Temp test' }
            });
            expect(testRes.ok()).toBeTruthy();
            const t = await testRes.json();
            testIds.push(t.id);

            const resultRes = await request.post(`${API_URL}/runs/${runId}/results`, {
                data: { test_case_id: t.id }
            });
            expect(resultRes.ok()).toBeTruthy();
            const result = await resultRes.json();
            resultIds.push(result.id);
        }

        // 5. Update statuses: 1 PASS, 1 FAIL, 1 remains PENDING
        await request.put(`${API_URL}/runs/${runId}/results/${resultIds[0]}`, {
            data: { status: 'PASS' }
        });
        await request.put(`${API_URL}/runs/${runId}/results/${resultIds[1]}`, {
            data: { status: 'FAIL' }
        });
    });

    test('should display correct stats in TestRunList columns', async ({ page }) => {
        await page.goto('/runs');

        // Show filter row and filter by our specific suite
        await page.getByRole('button', { name: 'Column Filters' }).click();
        const suiteSelect = page.getByTestId('filter-suite-select');
        await suiteSelect.selectOption(suiteId);

        const checkbox = page.locator(`[data-testid="select-run-checkbox-${runId}"]`);
        await expect(checkbox).toBeVisible();

        // Check columns
        await expect(page.getByTestId(`run-passed-${runId}`)).toHaveText('1');
        await expect(page.getByTestId(`run-failed-${runId}`)).toHaveText('1');
        await expect(page.getByTestId(`run-pending-${runId}`)).toHaveText('1');
        await expect(page.getByTestId(`run-total-${runId}`)).toHaveText('3');
    });

    test('should display correct stats in TestRunDetail header', async ({ page }) => {
        await page.goto(`/runs/run/${runId}`);

        await expect(page.getByTestId('stats-total')).toContainText('3');
        await expect(page.getByTestId('stats-passed')).toContainText('1');
        await expect(page.getByTestId('stats-failed')).toContainText('1');
        await expect(page.getByTestId('stats-pending')).toContainText('1');
    });
});
