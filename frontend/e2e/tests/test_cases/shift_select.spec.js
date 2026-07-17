import { test, expect } from '../../fixtures/test.js';
import { TIMEOUTS } from '../../config.js';

test.describe('Sidebar Selection', () => {

    test.beforeEach(async ({ libraryPage }) => {
        await libraryPage.open();
    });

    // Validates Shift-Click range selection with sibling folders
    test('should support shift-click range selection', async ({ page, libraryPage }) => {
        const timestamp = Date.now();
        const folders = [`Folder1_${timestamp}`, `Folder2_${timestamp}`, `Folder3_${timestamp}`];
        let nodeA;
        let nodeC;

        await test.step('Create three sibling root folders', async () => {
            for (const f of folders) {
                await libraryPage.createRootFolder(f);
            }
        });

        await test.step('Resolve the folder nodes for the range endpoints', async () => {
            nodeA = libraryPage.folderNode(folders[0]);
            // folders[1] is the intermediate endpoint of the range (selected implicitly).
            nodeC = libraryPage.folderNode(folders[2]);
        });

        await test.step('Select the first folder', async () => {
            await nodeA.click();
            await expect(nodeA).toHaveClass(/selected/);
        });

        await test.step('Shift-click the last folder to select the range', async () => {
            await nodeC.click({ modifiers: ['Shift'] });
        });

        await test.step('Verify both endpoints are selected and the bulk action bar appears', async () => {
            await expect(nodeA).toHaveClass(/selected/);
            await expect(nodeC).toHaveClass(/selected/);
            await expect(libraryPage.bulkDeleteFoldersButton).toBeVisible();
        });

        await test.step('Bulk-delete the selected folders and verify removal', async () => {
            const deletePromise = page.waitForResponse(
                resp => resp.url().includes('/folders/bulk-delete') && resp.ok(),
                { timeout: TIMEOUTS.APP_RENDER }
            );
            await libraryPage.bulkDeleteFoldersButton.click();
            await libraryPage.confirmModal();
            await deletePromise;
            await expect(libraryPage.folderNode(folders[0])).not.toBeVisible({ timeout: TIMEOUTS.APP_RENDER });
        });
    });

    // Validates Shift-Click range selection within nested structures
    test('should handle range selection with nested folders', async ({ libraryPage }) => {
        const timestamp = Date.now();
        const rootName = `Alpha_${timestamp}`;
        const subName = `Beta_${timestamp}`;
        const siblingName = `Gamma_${timestamp}`;
        let rootNode;
        let subNode;
        let siblingNode;

        await test.step('Set up the folder hierarchy (root, subfolder, sibling)', async () => {
            await libraryPage.createRootFolder(rootName);
            await libraryPage.createSubfolder(rootName, subName);
            await libraryPage.createRootFolder(siblingName);

            rootNode = libraryPage.folderNode(rootName);
            subNode = libraryPage.folderNode(subName);
            siblingNode = libraryPage.folderNode(siblingName);
        });

        await test.step('Select the subfolder, then shift-click the sibling', async () => {
            await subNode.click();
            await expect(subNode).toHaveClass(/selected/);
            // Range: Beta -> Gamma
            await siblingNode.click({ modifiers: ['Shift'] });
        });

        await test.step('Verify the range endpoints are selected but the parent is not', async () => {
            await expect(siblingNode).toHaveClass(/selected/);
            await expect(subNode).toHaveClass(/selected/);
            // Alpha is the parent of Beta and outside the visual range, so it stays unselected.
            await expect(rootNode).not.toHaveClass(/selected/);
        });

        await test.step('Bulk-delete the selected folders (Beta + Gamma)', async () => {
            await libraryPage.bulkDeleteFolders();
            await expect(libraryPage.folderNode(siblingName)).not.toBeVisible({ timeout: TIMEOUTS.APP_RENDER });
        });

        await test.step('Clean up the remaining root folder (Alpha)', async () => {
            const freshRootNode = libraryPage.folderNode(rootName);
            if (await freshRootNode.isVisible({ timeout: TIMEOUTS.UI_SETTLE }).catch(() => false)) {
                await libraryPage.deleteFolderViaContextMenu(rootName);
            }
            await expect(libraryPage.folderNode(rootName)).not.toBeVisible({ timeout: TIMEOUTS.APP_RENDER });
        });
    });
});
