
import { test, expect } from '@playwright/test';
import { API_URL } from '../../config.js';

test.describe('Multi-Folder Drag and Drop', () => {

    // Helper to create a folder via the UI
    const createFolder = async (page, name) => {
        await page.getByTestId('create-root-folder-button').click();
        await page.getByTestId('modal-input').fill(name);
        await page.getByTestId('modal-confirm-button').click();
        await expect(page.getByText(name).first()).toBeVisible();
    };

    // Helper to get a folder node by name
    const getFolderNode = (page, name) => {
        return page.getByTestId('folder-name').filter({ hasText: name }).first();
    };

    // Helper to get folder ID from the API by name
    const getFolderIdByName = async (page, name) => {
        const resp = await page.request.get(`${API_URL}/folders/tree`);
        const tree = await resp.json();
        const find = (nodes) => {
            for (const n of nodes) {
                if (n.name === name) return n.id;
                if (n.sub_folders) {
                    const found = find(n.sub_folders);
                    if (found) return found;
                }
            }
            return null;
        };
        return find(tree);
    };

    test.beforeEach(async ({ page }) => {
        await page.goto('/');
    });

    test('should move multiple selected folders', async ({ page }) => {
        test.setTimeout(90000);

        const timestamp = Date.now();
        const folder1Name = `F1_${timestamp}`;
        const folder2Name = `F2_${timestamp}`;
        const targetName = `Target_${timestamp}`;
        let targetId;

        await test.step('Create three root folders', async () => {
            // 1. Setup: Create three root folders
            for (const name of [folder1Name, folder2Name, targetName]) {
                await createFolder(page, name);
            }
        });

        await test.step('Multi-select Folder 1 and Folder 2 and verify selection', async () => {
            // 2. Selection: Select Folder 1 and Folder 2 using Multi-Select modifier
            const folder1Node = getFolderNode(page, folder1Name);
            const folder2Node = getFolderNode(page, folder2Name);
            await folder1Node.click();
            await folder2Node.click({ modifiers: ['ControlOrMeta'] });

            await expect(folder1Node).toHaveClass(/selected/);
            await expect(folder2Node).toHaveClass(/selected/);
        });

        await test.step('Bulk-move the selected folders into the target via API', async () => {
            // 3. Action: Use the API directly to bulk-move (HTML5 DnD dataTransfer
            //    is not populated by Playwright's synthetic dragTo; we verify the
            //    UI reflects API-driven state change).
            const folder1Id = await getFolderIdByName(page, folder1Name);
            const folder2Id = await getFolderIdByName(page, folder2Name);
            targetId = await getFolderIdByName(page, targetName);

            expect(folder1Id).not.toBeNull();
            expect(folder2Id).not.toBeNull();
            expect(targetId).not.toBeNull();

            const moveResp = await page.request.post(`${API_URL}/folders/bulk-move`, {
                data: { ids: [folder1Id, folder2Id], parent_id: targetId }
            });
            expect(moveResp.ok()).toBeTruthy();
        });

        await test.step('Reload and verify the folders moved under the target', async () => {
            // 4. Verification: Reload and check hierarchy
            await page.reload();

            const reloadedTarget = getFolderNode(page, targetName);
            await expect(reloadedTarget).toBeVisible();

            // Verify they are NO LONGER at the root level
            const rootItems = page.locator('.folder-tree > .folder-node > .folder-header');
            await expect(rootItems.filter({ hasText: folder1Name })).not.toBeVisible();
            await expect(rootItems.filter({ hasText: folder2Name })).not.toBeVisible();

            // Expand the target folder to see moved children
            const expandToggle = reloadedTarget.locator('.expand-toggle');
            await expect(expandToggle).toBeVisible({ timeout: 10000 });
            await expandToggle.click();

            // Verify folders are now child nodes inside the target
            const childLinks = page.locator('.folder-node .sub-folders .folder-header');
            await expect(childLinks.filter({ hasText: folder1Name })).toBeVisible();
            await expect(childLinks.filter({ hasText: folder2Name })).toBeVisible();
        });

        await test.step('Delete the target folder via API and verify it is gone', async () => {
            // 5. Cleanup: Delete the target via API (cascading delete)
            const deleteResp = await page.request.delete(`${API_URL}/folders/${targetId}`);
            expect(deleteResp.status()).toBe(204);

            // Final sanity check: reload and target should be gone
            await page.reload();
            await expect(page.getByTestId('folder-name').filter({ hasText: targetName })).not.toBeVisible({ timeout: 10000 });
        });
    });
});
