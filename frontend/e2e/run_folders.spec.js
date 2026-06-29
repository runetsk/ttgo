import { test, expect } from '@playwright/test';

const API_URL = 'http://localhost:8080/api';

// ── API helpers ──────────────────────────────────────────────────────────────

const createCategoryAPI = async (request, name) => {
    const res = await request.post(`${API_URL}/categories`, {
        data: { name, description: 'Run folder E2E category' }
    });
    expect(res.ok()).toBeTruthy();
    return res.json();
};

const createFolderAPI = async (request, name) => {
    const res = await request.post(`${API_URL}/folders`, {
        data: { name, parent_id: null }
    });
    expect(res.ok()).toBeTruthy();
    return res.json();
};

const createTestCaseAPI = async (request, name, folderId) => {
    const res = await request.post(`${API_URL}/tests`, {
        data: { name, folder_id: folderId, description: '' }
    });
    expect(res.ok()).toBeTruthy();
    return res.json();
};

const linkTestToCategoryAPI = async (request, testId, categoryId) => {
    const res = await request.post(`${API_URL}/tests/${testId}/categories`, {
        data: { category_id: categoryId }
    });
    expect(res.ok()).toBeTruthy();
};

const createRunAPI = async (request, categoryId, name, runFolderId = null) => {
    const res = await request.post(`${API_URL}/runs`, {
        data: { category_id: categoryId, name, run_folder_id: runFolderId }
    });
    expect(res.ok()).toBeTruthy();
    return res.json();
};

const createRunFolderAPI = async (request, name) => {
    const res = await request.post(`${API_URL}/run-folders`, {
        data: { name }
    });
    expect(res.ok()).toBeTruthy();
    return res.json();
};

const deleteRunFolderAPI = async (request, id) => {
    await request.delete(`${API_URL}/run-folders/${id}`);
};

// ── Set up: create a category with one test case (reusable fixture) ──────────
const setupCategory = async (request, tag) => {
    const folder = await createFolderAPI(request, `RF-Folder-${tag}`);
    const tc = await createTestCaseAPI(request, `RF-TC-${tag}`, folder.id);
    const category = await createCategoryAPI(request, `RF-Category-${tag}`);
    await linkTestToCategoryAPI(request, tc.id, category.id);
    return category;
};

// ════════════════════════════════════════════════════════════════════════════
// US1: Create and Manage Run Folders
// ════════════════════════════════════════════════════════════════════════════

test.describe('US1 — Create and Manage Run Folders', () => {
    test.setTimeout(60000);

    test('sidebar is visible on /runs and shows "All Runs"', async ({ page }) => {
        await page.goto('/runs');
        await expect(page.getByTestId('run-folder-sidebar')).toBeVisible();
        await expect(page.getByTestId('all-runs-entry')).toBeVisible();
        await expect(page.getByTestId('all-runs-entry')).toContainText('All Runs');
    });

    test('create folder appears in sidebar', async ({ page, request }) => {
        const name = `Smoke-${Date.now()}`;
        await page.goto('/runs');

        await page.getByTestId('add-folder-btn').click();
        await expect(page.getByTestId('modal-input')).toBeVisible();
        await page.getByTestId('modal-input').fill(name);
        await page.getByTestId('modal-confirm-button').click();

        // folder should appear in sidebar
        await expect(page.locator('.run-folder-item').filter({ hasText: name })).toBeVisible();

        // cleanup
        const folders = await request.get(`${API_URL}/run-folders`).then(r => r.json());
        const f = (folders.run_folders || []).find(f => f.name === name);
        if (f) await deleteRunFolderAPI(request, f.id);
    });

    test('rename folder updates name', async ({ page, request }) => {
        const folder = await createRunFolderAPI(request, `ToRename-${Date.now()}`);
        const newName = `Renamed-${Date.now()}`;

        await page.goto('/runs');
        const item = page.getByTestId(`run-folder-item-${folder.id}`);
        await expect(item).toBeVisible();

        // Hover to reveal rename button then click it
        await item.hover();
        await page.getByTestId(`rename-folder-${folder.id}`).click();

        await expect(page.getByTestId('modal-input')).toBeVisible();
        await page.getByTestId('modal-input').fill('');   // clear
        await page.getByTestId('modal-input').fill(newName);
        await page.getByTestId('modal-confirm-button').click();

        await expect(page.locator('.run-folder-item').filter({ hasText: newName })).toBeVisible();
        await expect(page.locator('.run-folder-item').filter({ hasText: folder.name })).not.toBeVisible();

        await deleteRunFolderAPI(request, folder.id);
    });

    test('empty name is rejected with inline error', async ({ page }) => {
        await page.goto('/runs');

        await page.getByTestId('add-folder-btn').click();
        await expect(page.getByTestId('modal-input')).toBeVisible();
        // Leave name empty and confirm
        await page.getByTestId('modal-confirm-button').click();

        // Error message should appear (either inline or in modal)
        await expect(page.locator('[data-testid="folder-create-error"], .error-banner, .run-folder-error')).toBeVisible();
    });

    test('delete folder removes it from sidebar and does not delete runs', async ({ page, request }) => {
        const suite = await setupCategory(request, `DelTest-${Date.now()}`);
        const folder = await createRunFolderAPI(request, `ToDelete-${Date.now()}`);
        const run = await createRunAPI(request, suite.id, `Run-InFolder-${Date.now()}`, folder.id);

        await page.goto('/runs');
        const item = page.getByTestId(`run-folder-item-${folder.id}`);
        await expect(item).toBeVisible();

        await item.hover();
        await page.getByTestId(`delete-folder-${folder.id}`).click();

        // Confirm delete modal
        await expect(page.getByTestId('modal-confirm-button')).toBeVisible();
        await page.getByTestId('modal-confirm-button').click();

        // Folder gone from sidebar
        await expect(item).not.toBeVisible({ timeout: 5000 });

        // Run still exists under All Runs
        await page.getByTestId('all-runs-entry').click();
        await expect(page.getByText(run.name)).toBeVisible({ timeout: 5000 });
    });
});

// ════════════════════════════════════════════════════════════════════════════
// US2: Assign Runs to Folders
// ════════════════════════════════════════════════════════════════════════════

test.describe('US2 — Assign Runs to Folders', () => {
    test.setTimeout(60000);

    test('folder dropdown appears in Create Run modal', async ({ page, request }) => {
        const folder = await createRunFolderAPI(request, `ModalFolder-${Date.now()}`);

        await page.goto('/runs');
        await page.getByTestId('create-test-run-button').click();

        const select = page.getByTestId('create-run-folder-select');
        await expect(select).toBeVisible();
        await expect(select.locator(`option[value="${folder.id}"]`)).toBeAttached();

        // cleanup
        await deleteRunFolderAPI(request, folder.id);
    });

    test('run created with folder appears under that folder in the API', async ({ page, request }) => {
        const suite = await setupCategory(request, `US2Create-${Date.now()}`);
        const folder = await createRunFolderAPI(request, `AssignFolder-${Date.now()}`);
        const runName = `US2-Run-${Date.now()}`;

        await page.goto('/runs');
        await page.getByTestId('create-test-run-button').click();

        await page.getByTestId('create-run-category-select').selectOption(suite.id);
        await page.getByTestId('create-run-name-input').fill(runName);
        await page.getByTestId('create-run-folder-select').selectOption(folder.id);
        await page.getByTestId('create-run-submit').click();

        // Run should appear in list
        await expect(page.getByText(runName)).toBeVisible({ timeout: 5000 });

        // Verify API: run has run_folder_id set
        const runsRes = await request.get(`${API_URL}/runs?run_folder_id=${folder.id}`);
        const runsData = await runsRes.json();
        const created = (runsData.runs || []).find(r => r.name === runName);
        expect(created).toBeTruthy();
        expect(created.run_folder_id).toBe(folder.id);

        await deleteRunFolderAPI(request, folder.id);
    });

    test('run created pre-selects active folder from sidebar', async ({ page, request }) => {
        const folder = await createRunFolderAPI(request, `PreSelect-${Date.now()}`);
        const suite = await setupCategory(request, `PreSelect-${Date.now()}`);

        await page.goto('/runs');

        // Select the folder in sidebar
        await page.getByTestId(`run-folder-item-${folder.id}`).click();

        // Open create modal
        await page.getByTestId('create-test-run-button').click();

        // Folder select should have the folder pre-selected
        const select = page.getByTestId('create-run-folder-select');
        await expect(select).toHaveValue(folder.id);

        await page.getByTestId('create-run-cancel').click();
        await deleteRunFolderAPI(request, folder.id);
    });
});

// ════════════════════════════════════════════════════════════════════════════
// US3: Filter and Navigate by Folder
// ════════════════════════════════════════════════════════════════════════════

test.describe('US3 — Filter and Navigate by Folder', () => {
    test.setTimeout(60000);

    test('clicking folder filters run list to only its runs', async ({ page, request }) => {
        const suite = await setupCategory(request, `US3Filter-${Date.now()}`);
        const folder = await createRunFolderAPI(request, `FilterFolder-${Date.now()}`);
        const runInFolder = await createRunAPI(request, suite.id, `InFolder-${Date.now()}`, folder.id);
        const runOutside = await createRunAPI(request, suite.id, `Outside-${Date.now()}`, null);

        await page.goto('/runs');

        // Initially "All Runs" selected — both runs visible
        await expect(page.getByText(runInFolder.name)).toBeVisible({ timeout: 5000 });
        await expect(page.getByText(runOutside.name)).toBeVisible();

        // Click folder
        await page.getByTestId(`run-folder-item-${folder.id}`).click();

        // Only the run in the folder should be visible
        await expect(page.getByText(runInFolder.name)).toBeVisible({ timeout: 5000 });
        await expect(page.getByText(runOutside.name)).not.toBeVisible({ timeout: 5000 });

        await deleteRunFolderAPI(request, folder.id);
    });

    test('clicking "All Runs" shows all runs', async ({ page, request }) => {
        const suite = await setupCategory(request, `US3All-${Date.now()}`);
        const folder = await createRunFolderAPI(request, `AllFolder-${Date.now()}`);
        const runInFolder = await createRunAPI(request, suite.id, `InF-${Date.now()}`, folder.id);
        const runOutside = await createRunAPI(request, suite.id, `Out-${Date.now()}`, null);

        await page.goto('/runs');
        await page.getByTestId(`run-folder-item-${folder.id}`).click();
        // now filtered

        // Click All Runs
        await page.getByTestId('all-runs-entry').click();

        await expect(page.getByText(runInFolder.name)).toBeVisible({ timeout: 5000 });
        await expect(page.getByText(runOutside.name)).toBeVisible();

        await deleteRunFolderAPI(request, folder.id);
    });

    test('sidebar collapse persists across page refresh', async ({ page }) => {
        await page.goto('/runs');

        // Initially expanded
        await expect(page.getByTestId('run-folder-sidebar')).toBeVisible();

        // Collapse
        await page.getByTestId('sidebar-collapse-btn').click();
        await expect(page.getByTestId('run-folder-sidebar-collapsed')).toBeVisible();

        // Refresh
        await page.reload();
        await expect(page.getByTestId('run-folder-sidebar-collapsed')).toBeVisible({ timeout: 5000 });

        // Expand again to not affect other tests
        await page.getByTestId('sidebar-expand-btn').click();
        await expect(page.getByTestId('run-folder-sidebar')).toBeVisible();
    });

    test('new run defaults into selected folder when created without modal folder choice', async ({ page, request }) => {
        const suite = await setupCategory(request, `US3Default-${Date.now()}`);
        const folder = await createRunFolderAPI(request, `DefaultFolder-${Date.now()}`);
        const runName = `DefaultRun-${Date.now()}`;

        await page.goto('/runs');

        // Select the folder first
        await page.getByTestId(`run-folder-item-${folder.id}`).click();

        // Open create modal — folder should be pre-selected
        await page.getByTestId('create-test-run-button').click();
        await page.getByTestId('create-run-category-select').selectOption(suite.id);
        await page.getByTestId('create-run-name-input').fill(runName);
        // Do NOT change the folder dropdown — it should already have folder.id selected
        await page.getByTestId('create-run-submit').click();

        // Run should appear in the filtered list
        await expect(page.getByText(runName)).toBeVisible({ timeout: 5000 });

        // Verify via API
        const res = await request.get(`${API_URL}/runs?run_folder_id=${folder.id}`);
        const data = await res.json();
        const created = (data.runs || []).find(r => r.name === runName);
        expect(created).toBeTruthy();

        await deleteRunFolderAPI(request, folder.id);
    });

    test('folder reorder persists across page refresh', async ({ page, request }) => {
        const f1 = await createRunFolderAPI(request, `Reorder-A-${Date.now()}`);
        const f2 = await createRunFolderAPI(request, `Reorder-B-${Date.now()}`);
        const f3 = await createRunFolderAPI(request, `Reorder-C-${Date.now()}`);

        // Reorder via API: move f3 before f1 (display_order = f1.display_order - 5)
        await request.patch(`${API_URL}/run-folders/${f3.id}/order`, {
            data: { display_order: f1.display_order - 5 }
        });

        await page.goto('/runs');
        await page.reload();

        // f3 should appear before f1 in the sidebar — compare vertical positions via data-testid
        const f3Item = page.getByTestId(`run-folder-item-${f3.id}`);
        const f1Item = page.getByTestId(`run-folder-item-${f1.id}`);
        await expect(f3Item).toBeVisible({ timeout: 10000 });
        await expect(f1Item).toBeVisible({ timeout: 10000 });
        const f3Box = await f3Item.boundingBox();
        const f1Box = await f1Item.boundingBox();
        expect(f3Box.y).toBeLessThan(f1Box.y);

        await deleteRunFolderAPI(request, f1.id);
        await deleteRunFolderAPI(request, f2.id);
        await deleteRunFolderAPI(request, f3.id);
    });
});
