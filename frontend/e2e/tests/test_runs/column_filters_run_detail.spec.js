import { test, expect } from '@playwright/test';
import { API_URL } from '../../config.js';

test.describe('Run detail results filters', () => {
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
        let run;

        await test.step('Seed a folder, two tests, a run, and two PENDING results via API', async () => {
            // Seed: folder + 2 tests + a run + 2 PENDING run results
            const stamp = Date.now();
            const folder = await createFolderAPI(request, `CF Folder ${stamp}`);
            const t1 = await createTestAPI(request, `CF Test A ${stamp}`, folder.id);
            const t2 = await createTestAPI(request, `CF Test B ${stamp}`, folder.id);
            run = await createRunAPI(request, `CF Run ${stamp}`);
            await addRunResultAPI(request, run.id, t1.id, 'PENDING');
            await addRunResultAPI(request, run.id, t2.id, 'PENDING');
        });

        await test.step('Open the run detail page and verify the toolbar is visible', async () => {
            await page.goto(`/runs/run/${run.id}`);
            await page.waitForLoadState('domcontentloaded');

            // Toolbar must be visible
            const toolbar = page.getByTestId('run-results-toolbar');
            await expect(toolbar).toBeVisible({ timeout: 30000 });
        });

        await test.step('Toggle Column Filters on and verify the status filter appears', async () => {
            // Initially, status filter should not be visible
            await expect(page.getByTestId('filter-result-status')).not.toBeVisible();

            // Click the Column Filters toggle button
            await page.getByRole('button', { name: 'Column Filters' }).click();

            // Status filter control is now visible
            await expect(page.getByTestId('filter-result-status')).toBeVisible();
        });

        await test.step('Verify two rows are visible before filtering', async () => {
            // 2 rows visible before filtering
            await expect(page.locator('tbody tr[data-result-id]')).toHaveCount(2);
        });

        await test.step('Filter to PASS and verify no rows remain', async () => {
            // Filter to PASS → 0 result rows (all are PENDING)
            await page.getByTestId('filter-result-status').selectOption('PASS');
            await expect(page.locator('tbody tr[data-result-id]')).toHaveCount(0);
        });

        await test.step('Filter back to PENDING and verify two rows return', async () => {
            // Filter back to PENDING → 2 result rows
            await page.getByTestId('filter-result-status').selectOption('PENDING');
            await expect(page.locator('tbody tr[data-result-id]')).toHaveCount(2);
        });
    });

    test('filter row toggle shows and hides', async ({ page, request }) => {
        let run;

        await test.step('Seed a folder, a test, a run, and a PENDING result via API', async () => {
            const stamp = Date.now();
            const folder = await createFolderAPI(request, `CF2 Folder ${stamp}`);
            const t1 = await createTestAPI(request, `CF2 Test A ${stamp}`, folder.id);
            run = await createRunAPI(request, `CF2 Run ${stamp}`);
            await addRunResultAPI(request, run.id, t1.id, 'PENDING');
        });

        await test.step('Open the run detail page and verify the toolbar is visible', async () => {
            await page.goto(`/runs/run/${run.id}`);
            await page.waitForLoadState('domcontentloaded');
            await expect(page.getByTestId('run-results-toolbar')).toBeVisible({ timeout: 30000 });
        });

        await test.step('Show the filters and verify the status filter appears', async () => {
            // Show filters
            await page.getByRole('button', { name: 'Column Filters' }).click();
            await expect(page.getByTestId('filter-result-status')).toBeVisible();
        });

        await test.step('Hide the filters and verify the status filter disappears', async () => {
            // Hide filters
            await page.getByRole('button', { name: 'Hide Filters' }).click();
            await expect(page.getByTestId('filter-result-status')).not.toBeVisible();
        });
    });
});
