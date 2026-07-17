import { test, expect } from '../../fixtures/test.js';
import { TIMEOUTS } from '../../config.js';

test.describe('US1 — Create and Manage Run Folders', () => {

    test('sidebar is visible on /runs and shows "All Runs"', async ({ runsPage }) => {
        await test.step('Open the runs page and verify the sidebar shows "All Runs"', async () => {
            await runsPage.open();
            await expect(runsPage.sidebar).toBeVisible();
            await expect(runsPage.allRunsEntry).toBeVisible();
            await expect(runsPage.allRunsEntry).toContainText('All Runs');
        });
    });

    test('create folder appears in sidebar', async ({ page, runsPage, api }) => {
        const name = `Smoke-${Date.now()}`;

        await test.step('Open the runs page', async () => {
            await runsPage.open();
        });

        await test.step('Create a folder via the add-folder modal', async () => {
            await runsPage.openAddFolderModal();
            await expect(runsPage.modalInput).toBeVisible();
            await runsPage.fillModal(name);
            await runsPage.confirmModal();
        });

        await test.step('Verify the new folder appears in the sidebar', async () => {
            await expect(page.locator('.run-folder-item').filter({ hasText: name })).toBeVisible();
        });

        await test.step('Clean up the created folder via API', async () => {
            const folders = await api.get('/run-folders').then(r => r.json());
            const f = (folders.run_folders || []).find(f => f.name === name);
            if (f) await api.deleteRunFolder(f.id);
        });
    });

    test('rename folder updates name', async ({ page, runsPage, api }) => {
        const newName = `Renamed-${Date.now()}`;
        const folder = await api.createRunFolder(`ToRename-${Date.now()}`);

        await test.step('Open the runs page and locate the folder in the sidebar', async () => {
            await runsPage.open();
            await expect(runsPage.folderItem(folder.id)).toBeVisible();
        });

        await test.step('Open the folder menu and rename it', async () => {
            await runsPage.openFolderMenu(folder.id);
            await runsPage.clickRenameFolder(folder.id);
            await expect(runsPage.modalInput).toBeVisible();
            await runsPage.fillModal(newName);
            await runsPage.confirmModal();
        });

        await test.step('Verify the new name shows and the old name is gone', async () => {
            await expect(page.locator('.run-folder-item').filter({ hasText: newName })).toBeVisible();
            await expect(page.locator('.run-folder-item').filter({ hasText: folder.name })).not.toBeVisible();
        });

        await test.step('Clean up the folder via API', async () => {
            await api.deleteRunFolder(folder.id);
        });
    });

    test('empty name is rejected with inline error', async ({ page, runsPage }) => {
        await test.step('Open the runs page', async () => {
            await runsPage.open();
        });

        await test.step('Open the add-folder modal and confirm with an empty name', async () => {
            await runsPage.openAddFolderModal();
            await expect(runsPage.modalInput).toBeVisible();
            await runsPage.confirmModal();
        });

        await test.step('Verify an error message is shown', async () => {
            await expect(page.locator('[data-testid="folder-create-error"], .error-banner, .run-folder-error')).toBeVisible();
        });
    });

    test('delete folder removes it from sidebar and does not delete runs', async ({ page, runsPage, api }) => {
        const { category } = await api.setupCategoryWithTest(`DelTest-${Date.now()}`);
        const folder = await api.createRunFolder(`ToDelete-${Date.now()}`);
        const run = await api.createRun(`Run-InFolder-${Date.now()}`, { categoryId: category.id, runFolderId: folder.id });

        await test.step('Open the runs page and locate the folder in the sidebar', async () => {
            await runsPage.open();
            await expect(runsPage.folderItem(folder.id)).toBeVisible();
        });

        await test.step('Delete the folder via its menu and confirm', async () => {
            await runsPage.openFolderMenu(folder.id);
            await runsPage.clickDeleteFolder(folder.id);
            await expect(runsPage.modalConfirm).toBeVisible();
            await runsPage.confirmModal();
        });

        await test.step('Verify the folder is gone from the sidebar', async () => {
            await expect(runsPage.folderItem(folder.id)).not.toBeVisible({ timeout: TIMEOUTS.UI_SETTLE });
        });

        await test.step('Verify the run still exists under All Runs', async () => {
            await runsPage.allRunsEntry.click();
            await expect(page.getByText(run.name)).toBeVisible({ timeout: TIMEOUTS.UI_SETTLE });
        });
    });
});

test.describe('US2 — Assign Runs to Folders', () => {

    test('folder dropdown appears in Create Run modal', async ({ runsPage, api }) => {
        const folder = await api.createRunFolder(`ModalFolder-${Date.now()}`);

        await test.step('Open the Create Run modal and verify the folder option is present', async () => {
            await runsPage.open();
            await runsPage.openNewRunModal();
            await expect(runsPage.runFolderSelect).toBeVisible();
            await expect(runsPage.runFolderSelect.locator(`option[value="${folder.id}"]`)).toBeAttached();
        });

        await test.step('Clean up the folder via API', async () => {
            await api.deleteRunFolder(folder.id);
        });
    });

    test('run created with folder appears under that folder in the API', async ({ page, runsPage, api }) => {
        const runName = `US2-Run-${Date.now()}`;
        const folder = await api.createRunFolder(`AssignFolder-${Date.now()}`);

        await test.step('Create a run with the folder selected via the modal', async () => {
            await runsPage.open();
            await runsPage.createRun({ name: runName, folder: folder.id });
        });

        await test.step('Verify the run appears in the list', async () => {
            await expect(page.getByText(runName)).toBeVisible({ timeout: TIMEOUTS.UI_SETTLE });
        });

        await test.step('Verify via API that the run has run_folder_id set', async () => {
            const runsRes = await api.get(`/runs?run_folder_id=${folder.id}`);
            const runsData = await runsRes.json();
            const created = (runsData.runs || []).find(r => r.name === runName);
            expect(created).toBeTruthy();
            expect(created.run_folder_id).toBe(folder.id);
        });

        await test.step('Clean up the folder via API', async () => {
            await api.deleteRunFolder(folder.id);
        });
    });

    test('run created pre-selects active folder from sidebar', async ({ runsPage, api }) => {
        const folder = await api.createRunFolder(`PreSelect-${Date.now()}`);
        await api.setupCategoryWithTest(`PreSelect-${Date.now()}`);

        await test.step('Open the runs page and select the folder in the sidebar', async () => {
            await runsPage.open();
            await runsPage.folderItem(folder.id).click();
        });

        await test.step('Open the create modal and verify the folder is pre-selected', async () => {
            await runsPage.openNewRunModal();
            await expect(runsPage.runFolderSelect).toHaveValue(folder.id);
            await runsPage.cancelNewRun();
        });

        await test.step('Clean up the folder via API', async () => {
            await api.deleteRunFolder(folder.id);
        });
    });
});

test.describe('US3 — Filter and Navigate by Folder', () => {

    test('clicking folder filters run list to only its runs', async ({ page, runsPage, api }) => {
        const { category } = await api.setupCategoryWithTest(`US3Filter-${Date.now()}`);
        const folder = await api.createRunFolder(`FilterFolder-${Date.now()}`);
        const runInFolder = await api.createRun(`InFolder-${Date.now()}`, { categoryId: category.id, runFolderId: folder.id });
        const runOutside = await api.createRun(`Outside-${Date.now()}`, { categoryId: category.id });

        await test.step('Open the runs page and verify both runs are visible under All Runs', async () => {
            await runsPage.open();
            await expect(page.getByText(runInFolder.name)).toBeVisible({ timeout: TIMEOUTS.UI_SETTLE });
            await expect(page.getByText(runOutside.name)).toBeVisible();
        });

        await test.step('Click the folder and verify only its run is visible', async () => {
            await runsPage.folderItem(folder.id).click();
            await expect(page.getByText(runInFolder.name)).toBeVisible({ timeout: TIMEOUTS.UI_SETTLE });
            await expect(page.getByText(runOutside.name)).not.toBeVisible({ timeout: TIMEOUTS.UI_SETTLE });
        });

        await test.step('Clean up the folder via API', async () => {
            await api.deleteRunFolder(folder.id);
        });
    });

    test('clicking "All Runs" shows all runs', async ({ page, runsPage, api }) => {
        const { category } = await api.setupCategoryWithTest(`US3All-${Date.now()}`);
        const folder = await api.createRunFolder(`AllFolder-${Date.now()}`);
        const runInFolder = await api.createRun(`InF-${Date.now()}`, { categoryId: category.id, runFolderId: folder.id });
        const runOutside = await api.createRun(`Out-${Date.now()}`, { categoryId: category.id });

        await test.step('Open the runs page and filter by the folder', async () => {
            await runsPage.open();
            await runsPage.folderItem(folder.id).click();
        });

        await test.step('Click All Runs and verify both runs are visible', async () => {
            await runsPage.allRunsEntry.click();
            await expect(page.getByText(runInFolder.name)).toBeVisible({ timeout: TIMEOUTS.UI_SETTLE });
            await expect(page.getByText(runOutside.name)).toBeVisible();
        });

        await test.step('Clean up the folder via API', async () => {
            await api.deleteRunFolder(folder.id);
        });
    });

    test('sidebar collapse persists across page refresh', async ({ page, runsPage }) => {
        await test.step('Open the runs page and verify the sidebar is expanded', async () => {
            await runsPage.open();
            await expect(runsPage.sidebar).toBeVisible();
        });

        await test.step('Collapse the sidebar', async () => {
            await runsPage.collapseSidebar();
            await expect(runsPage.sidebarCollapsed).toBeVisible();
        });

        await test.step('Reload and verify the collapsed state persists', async () => {
            await page.reload();
            await expect(runsPage.sidebarCollapsed).toBeVisible({ timeout: TIMEOUTS.UI_SETTLE });
        });

        await test.step('Expand the sidebar again to not affect other tests', async () => {
            await runsPage.expandSidebar();
            await expect(runsPage.sidebar).toBeVisible();
        });
    });

    test('new run defaults into selected folder when created without modal folder choice', async ({ page, runsPage, api }) => {
        const runName = `DefaultRun-${Date.now()}`;
        const folder = await api.createRunFolder(`DefaultFolder-${Date.now()}`);

        await test.step('Open the runs page and select the folder in the sidebar', async () => {
            await runsPage.open();
            await runsPage.folderItem(folder.id).click();
        });

        await test.step('Create a run without changing the pre-selected folder', async () => {
            await runsPage.openNewRunModal();
            await runsPage.runNameInput.fill(runName);
            await runsPage.submitNewRun();
        });

        await test.step('Verify the run appears in the filtered list', async () => {
            await expect(page.getByText(runName)).toBeVisible({ timeout: TIMEOUTS.UI_SETTLE });
        });

        await test.step('Verify via API that the run is in the folder', async () => {
            const res = await api.get(`/runs?run_folder_id=${folder.id}`);
            const data = await res.json();
            const created = (data.runs || []).find(r => r.name === runName);
            expect(created).toBeTruthy();
        });

        await test.step('Clean up the folder via API', async () => {
            await api.deleteRunFolder(folder.id);
        });
    });

    test('folder reorder persists across page refresh', async ({ page, runsPage, api }) => {
        let f1;
        let f2;
        let f3;

        await test.step('Create three run folders via API', async () => {
            f1 = await api.createRunFolder(`Reorder-A-${Date.now()}`);
            f2 = await api.createRunFolder(`Reorder-B-${Date.now()}`);
            f3 = await api.createRunFolder(`Reorder-C-${Date.now()}`);
        });

        await test.step('Reorder via API to move f3 before f1', async () => {
            await api.patch(`/run-folders/${f3.id}/order`, { display_order: f1.display_order - 5 });
        });

        await test.step('Open the runs page and reload', async () => {
            await runsPage.open();
            await page.reload();
        });

        await test.step('Verify f3 appears before f1 in the sidebar', async () => {
            const f3Item = runsPage.folderItem(f3.id);
            const f1Item = runsPage.folderItem(f1.id);
            await expect(f3Item).toBeVisible({ timeout: TIMEOUTS.ELEMENT });
            await expect(f1Item).toBeVisible({ timeout: TIMEOUTS.ELEMENT });
            const f3Box = await f3Item.boundingBox();
            const f1Box = await f1Item.boundingBox();
            expect(f3Box.y).toBeLessThan(f1Box.y);
        });

        await test.step('Clean up the folders via API', async () => {
            await api.deleteRunFolder(f1.id);
            await api.deleteRunFolder(f2.id);
            await api.deleteRunFolder(f3.id);
        });
    });
});
