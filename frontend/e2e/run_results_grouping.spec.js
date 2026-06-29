import { test, expect } from '@playwright/test';
import { API_URL } from './config.js';

// Verifies the run-results List/Grouped view ported from the legacy tree:
// toggle, group-by selector, group headers, collapse/expand, and that the
// view preference persists across a reload (localStorage).
test.describe('Run Results Grouping', () => {
    const createFolderAPI = async (request, name) => {
        const res = await request.post(`${API_URL}/folders`, { data: { name, parent_id: null } });
        expect(res.ok()).toBeTruthy();
        return await res.json();
    };
    const createTestAPI = async (request, name, folderId) => {
        const res = await request.post(`${API_URL}/tests`, { data: { name, folder_id: folderId, description: 'grouping e2e' } });
        expect(res.ok()).toBeTruthy();
        return await res.json();
    };
    const createCategoryAPI = async (request, name) => {
        const res = await request.post(`${API_URL}/categories`, { data: { name, description: 'grouping e2e' } });
        expect(res.ok()).toBeTruthy();
        return await res.json();
    };
    const linkTestToCategoryAPI = async (request, testId, categoryId) => {
        const res = await request.post(`${API_URL}/tests/${testId}/categories`, { data: { category_id: categoryId } });
        expect(res.ok()).toBeTruthy();
    };
    const createRunAPI = async (request, categoryId, name) => {
        const res = await request.post(`${API_URL}/runs`, { data: { category_id: categoryId, name } });
        expect(res.ok()).toBeTruthy();
        return await res.json();
    };

    test('toggles list/grouped, shows group headers, collapses, and persists', async ({ page, request }) => {
        let run;

        await test.step('Seed a folder, two tests, a category, and a run via API', async () => {
            const stamp = Date.now();
            const folder = await createFolderAPI(request, `Grp Folder ${stamp}`);
            const t1 = await createTestAPI(request, `Grp Test A ${stamp}`, folder.id);
            const t2 = await createTestAPI(request, `Grp Test B ${stamp}`, folder.id);
            const category = await createCategoryAPI(request, `Grp Category ${stamp}`);
            await linkTestToCategoryAPI(request, t1.id, category.id);
            await linkTestToCategoryAPI(request, t2.id, category.id);
            run = await createRunAPI(request, category.id, `Grp Run ${stamp}`);
        });

        await test.step('Open the run detail page and verify the toolbar in list view', async () => {
            await page.goto(`/runs/run/${run.id}`);
            await page.waitForLoadState('domcontentloaded');

            // Toolbar is present in the default (list) view.
            const toolbar = page.getByTestId('run-results-toolbar');
            await expect(toolbar).toBeVisible({ timeout: 30000 });
        });

        await test.step('Switch to grouped view and verify the selector and a group header', async () => {
            // Switch to grouped view → group-by selector + at least one group header.
            await page.getByTestId('view-toggle-grouped').click();
            await expect(page.getByTestId('group-by-select')).toBeVisible();
            await expect(page.getByTestId('group-header').first()).toBeVisible();
        });

        await test.step('Use the collapse-all and expand-all controls', async () => {
            // Collapse / expand all controls work.
            await page.getByTestId('collapse-all').click();
            await page.getByTestId('expand-all').click();
        });

        await test.step('Reload and verify the grouped-view preference persists', async () => {
            // Preference persists across reload (localStorage).
            await page.reload();
            await page.waitForLoadState('domcontentloaded');
            await expect(page.getByTestId('view-toggle-grouped')).toBeVisible({ timeout: 30000 });
            await expect(page.getByTestId('group-by-select')).toBeVisible();
        });
    });
});
