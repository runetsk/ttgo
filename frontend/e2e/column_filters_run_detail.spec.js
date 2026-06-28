import { test, expect } from '@playwright/test';

test.describe('Run detail results filters', () => {
    const API_URL = 'http://localhost:8080/api';

    const createFolderAPI = async (request, name) => {
        const res = await request.post(`${API_URL}/folders`, { data: { name, parent_id: null } });
        expect(res.ok()).toBeTruthy();
        return await res.json();
    };
    const createTestAPI = async (request, name, folderId) => {
        const res = await request.post(`${API_URL}/tests`, { data: { name, folder_id: folderId, description: 'col-filters e2e' } });
        expect(res.ok()).toBeTruthy();
        return await res.json();
    };
    const createRunAPI = async (request, name) => {
        const res = await request.post(`${API_URL}/runs`, { data: { name } });
        expect(res.ok()).toBeTruthy();
        return await res.json();
    };
    const addRunResultAPI = async (request, runId, testCaseId, status = 'PENDING') => {
        const res = await request.post(`${API_URL}/runs/${runId}/results`, {
            data: { test_case_id: testCaseId, status },
        });
        expect(res.ok()).toBeTruthy();
        return await res.json();
    };

    test('filter row toggles and status filter narrows results', async ({ page, request }) => {
        // Seed: folder + 2 tests + a run + 2 PENDING run results
        const stamp = Date.now();
        const folder = await createFolderAPI(request, `CF Folder ${stamp}`);
        const t1 = await createTestAPI(request, `CF Test A ${stamp}`, folder.id);
        const t2 = await createTestAPI(request, `CF Test B ${stamp}`, folder.id);
        const run = await createRunAPI(request, `CF Run ${stamp}`);
        await addRunResultAPI(request, run.id, t1.id, 'PENDING');
        await addRunResultAPI(request, run.id, t2.id, 'PENDING');

        await page.goto(`/runs/run/${run.id}`);
        await page.waitForLoadState('domcontentloaded');

        // Toolbar must be visible
        const toolbar = page.getByTestId('run-results-toolbar');
        await expect(toolbar).toBeVisible({ timeout: 30000 });

        // Initially, status filter should not be visible
        await expect(page.getByTestId('filter-result-status')).not.toBeVisible();

        // Click the Column Filters toggle button
        await page.getByRole('button', { name: 'Column Filters' }).click();

        // Status filter control is now visible
        await expect(page.getByTestId('filter-result-status')).toBeVisible();

        // 2 rows visible before filtering
        await expect(page.locator('tbody tr[data-result-id]')).toHaveCount(2);

        // Filter to PASS → 0 result rows (all are PENDING)
        await page.getByTestId('filter-result-status').selectOption('PASS');
        await expect(page.locator('tbody tr[data-result-id]')).toHaveCount(0);

        // Filter back to PENDING → 2 result rows
        await page.getByTestId('filter-result-status').selectOption('PENDING');
        await expect(page.locator('tbody tr[data-result-id]')).toHaveCount(2);
    });

    test('filter row toggle shows and hides', async ({ page, request }) => {
        const stamp = Date.now();
        const folder = await createFolderAPI(request, `CF2 Folder ${stamp}`);
        const t1 = await createTestAPI(request, `CF2 Test A ${stamp}`, folder.id);
        const run = await createRunAPI(request, `CF2 Run ${stamp}`);
        await addRunResultAPI(request, run.id, t1.id, 'PENDING');

        await page.goto(`/runs/run/${run.id}`);
        await page.waitForLoadState('domcontentloaded');
        await expect(page.getByTestId('run-results-toolbar')).toBeVisible({ timeout: 30000 });

        // Show filters
        await page.getByRole('button', { name: 'Column Filters' }).click();
        await expect(page.getByTestId('filter-result-status')).toBeVisible();

        // Hide filters
        await page.getByRole('button', { name: 'Hide Filters' }).click();
        await expect(page.getByTestId('filter-result-status')).not.toBeVisible();
    });
});
