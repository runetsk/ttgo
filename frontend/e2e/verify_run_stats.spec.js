import { test, expect } from '@playwright/test';
import { API_URL } from './config.js';

test.describe('Test Run Stats Verification', () => {
    let runId;
    let categoryId;
    let folderId;
    let testIds = [];

    test.beforeAll(async ({ request }) => {
        const timestamp = Date.now();

        // 1. Create a unique category for isolation
        const categoryRes = await request.post(`${API_URL}/categories`, {
            data: { name: `Stats Category ${timestamp}`, description: 'E2E stats test' }
        });
        expect(categoryRes.ok()).toBeTruthy();
        const category = await categoryRes.json();
        categoryId = category.id;

        // 2. Create a folder
        const folderRes = await request.post(`${API_URL}/folders`, {
            data: { name: `Stats Folder ${timestamp}`, parent_id: null }
        });
        expect(folderRes.ok()).toBeTruthy();
        const folder = await folderRes.json();
        folderId = folder.id;

        // 3. Create a test run linked to the category
        const runRes = await request.post(`${API_URL}/runs`, {
            data: { category_id: categoryId, name: `Stats Verify Run ${timestamp}` }
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
        await test.step('Open the runs page and filter by the seeded category', async () => {
            await page.goto('/runs');

            // Show filter row and filter by our specific category
            await page.getByRole('button', { name: 'Column Filters' }).click();
            await page.getByTestId('filter-run-category').click();
            await page.getByTestId(`filter-run-category-option-${categoryId}`).click();
            await page.keyboard.press('Escape');
        });

        await test.step('Verify the run row is visible and the stats columns are correct', async () => {
            const checkbox = page.locator(`[data-testid="select-run-checkbox-${runId}"]`);
            await expect(checkbox).toBeVisible();

            // Check columns
            // Passed/Failed are default-visible columns; Pending/Total are optional
            // columns hidden by default, so assert the visible stats here.
            await expect(page.getByTestId(`run-passed-${runId}`)).toHaveText('1');
            await expect(page.getByTestId(`run-failed-${runId}`)).toHaveText('1');
        });
    });

    test('should display correct stats in TestRunDetail header', async ({ page }) => {
        await test.step('Open the run detail page', async () => {
            await page.goto(`/runs/run/${runId}`);
        });

        await test.step('Verify the stats bar shows correct passed, failed, and pending counts', async () => {
            // Redesigned stats bar shows passed as "{passed} / {total}" — no separate total testid.
            await expect(page.getByTestId('stats-passed')).toContainText('1');
            await expect(page.getByTestId('stats-passed')).toContainText('3');
            await expect(page.getByTestId('stats-failed')).toContainText('1');
            await expect(page.getByTestId('stats-pending')).toContainText('1');
        });
    });
});
