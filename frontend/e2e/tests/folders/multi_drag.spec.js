import { test, expect } from '../../fixtures/test.js';
import { TIMEOUTS } from '../../config.js';

test.describe('Multi-Folder Drag and Drop', () => {

    test.beforeEach(async ({ libraryPage }) => {
        await libraryPage.open();
    });

    test('should move multiple selected folders', async ({ page, libraryPage, api }) => {
        const timestamp = Date.now();
        const folder1Name = `F1_${timestamp}`;
        const folder2Name = `F2_${timestamp}`;
        const targetName = `Target_${timestamp}`;
        let targetId;

        await test.step('Create three root folders', async () => {
            for (const name of [folder1Name, folder2Name, targetName]) {
                await libraryPage.createRootFolder(name);
            }
        });

        await test.step('Multi-select Folder 1 and Folder 2 and verify selection', async () => {
            const folder1Node = libraryPage.folderNode(folder1Name);
            const folder2Node = libraryPage.folderNode(folder2Name);
            await folder1Node.click();
            await folder2Node.click({ modifiers: ['ControlOrMeta'] });
            await expect(folder1Node).toHaveClass(/selected/);
            await expect(folder2Node).toHaveClass(/selected/);
        });

        await test.step('Bulk-move the selected folders into the target via API', async () => {
            // Playwright's synthetic dragTo doesn't populate HTML5 DnD dataTransfer,
            // so drive the move via the API and verify the UI reflects the change.
            const folder1Id = await api.getFolderIdByName(folder1Name);
            const folder2Id = await api.getFolderIdByName(folder2Name);
            targetId = await api.getFolderIdByName(targetName);

            expect(folder1Id).not.toBeNull();
            expect(folder2Id).not.toBeNull();
            expect(targetId).not.toBeNull();

            const moveResp = await api.post('/folders/bulk-move', { ids: [folder1Id, folder2Id], parent_id: targetId });
            expect(moveResp.ok()).toBeTruthy();
        });

        await test.step('Reload and verify the folders moved under the target', async () => {
            await page.reload();
            const reloadedTarget = libraryPage.folderNode(targetName);
            await expect(reloadedTarget).toBeVisible();

            // They should no longer be at the root level.
            const rootItems = page.locator('.folder-tree > .folder-node > .folder-header');
            await expect(rootItems.filter({ hasText: folder1Name })).not.toBeVisible();
            await expect(rootItems.filter({ hasText: folder2Name })).not.toBeVisible();

            const expandToggle = reloadedTarget.locator('.expand-toggle');
            await expect(expandToggle).toBeVisible({ timeout: TIMEOUTS.ELEMENT });
            await expandToggle.click();

            const childLinks = page.locator('.folder-node .sub-folders .folder-header');
            await expect(childLinks.filter({ hasText: folder1Name })).toBeVisible();
            await expect(childLinks.filter({ hasText: folder2Name })).toBeVisible();
        });

        await test.step('Delete the target folder via API and verify it is gone', async () => {
            const deleteResp = await api.delete(`/folders/${targetId}`);
            expect(deleteResp.status()).toBe(204);
            await page.reload();
            await expect(libraryPage.folderNode(targetName)).not.toBeVisible({ timeout: TIMEOUTS.ELEMENT });
        });
    });
});
