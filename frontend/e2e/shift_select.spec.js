
import { test, expect } from '@playwright/test';

test.describe('Sidebar Selection', () => {

    // Helper to create a root folder
    const createRootFolder = async (page, name) => {
        await page.getByTestId('create-root-folder-button').click();
        await page.getByTestId('modal-input').fill(name);
        await page.getByTestId('modal-confirm-button').click();
        await expect(page.getByText(name).first()).toBeVisible({ timeout: 10000 });
        // Allow sidebar state to stabilize after refresh
        await page.waitForTimeout(300);
    };

    // Helper to create a subfolder
    const createSubfolder = async (page, parentName, name) => {
        const parentNode = page.getByTestId('folder-name').filter({ hasText: parentName }).first();
        await parentNode.click({ button: 'right' });
        await page.getByTestId('context-menu-create-subfolder').click();
        await page.getByTestId('modal-input').fill(name);
        await page.getByTestId('modal-confirm-button').click();
        await expect(page.getByText(name).first()).toBeVisible({ timeout: 10000 });
        // Allow sidebar state to stabilize
        await page.waitForTimeout(300);
    };

    // Helper to get a folder node
    const getFolderNode = (page, name) => {
        return page.getByTestId('folder-name').filter({ hasText: name }).first();
    };

    test.beforeEach(async ({ page }) => {
        await page.goto('/');
    });

    // Validates Shift-Click range selection with sibling folders
    test('should support shift-click range selection', async ({ page }) => {
        test.setTimeout(90000);
        const timestamp = Date.now();
        const folders = [`Folder1_${timestamp}`, `Folder2_${timestamp}`, `Folder3_${timestamp}`];

        for (const f of folders) {
            await createRootFolder(page, f);
        }

        const nodeA = getFolderNode(page, folders[0]);
        const nodeB = getFolderNode(page, folders[1]); // Intermediate
        const nodeC = getFolderNode(page, folders[2]);

        // Select A
        await nodeA.click();
        await expect(nodeA).toHaveClass(/selected/);

        // Shift-Select C
        await nodeC.click({ modifiers: ['Shift'] });

        // Verify A and C (endpoints) are selected
        await expect(nodeA).toHaveClass(/selected/);
        await expect(nodeC).toHaveClass(/selected/);

        // Verify bulk action bar appeared
        await expect(page.getByTestId('bulk-delete-folders-button')).toBeVisible();

        // Cleanup: wait for the delete API to complete, then verify
        const deletePromise = page.waitForResponse(
            resp => resp.url().includes('/folders/bulk-delete') && resp.ok(),
            { timeout: 15000 }
        );
        await page.getByTestId('bulk-delete-folders-button').click();
        await page.getByTestId('modal-confirm-button').click();
        await deletePromise;
        await expect(page.getByTestId('folder-name').filter({ hasText: folders[0] })).not.toBeVisible({ timeout: 15000 });
    });

    // Validates Shift-Click range selection within nested structures
    test('should handle range selection with nested folders', async ({ page }) => {
        test.setTimeout(90000);
        const timestamp = Date.now();
        const rootName = `Alpha_${timestamp}`;
        const subName = `Beta_${timestamp}`;
        const siblingName = `Gamma_${timestamp}`;

        // 1. Setup Hierarchy
        await createRootFolder(page, rootName);
        await createSubfolder(page, rootName, subName);
        await createRootFolder(page, siblingName);

        const rootNode = getFolderNode(page, rootName);
        const subNode = getFolderNode(page, subName);
        const siblingNode = getFolderNode(page, siblingName);

        // 2. Selection Action
        // Select Sub (Beta)
        await subNode.click();
        await expect(subNode).toHaveClass(/selected/);

        // Shift-Select Sibling (Gamma)
        // Range: Beta -> Gamma
        await siblingNode.click({ modifiers: ['Shift'] });

        // 3. Verification
        await expect(siblingNode).toHaveClass(/selected/);
        await expect(subNode).toHaveClass(/selected/);

        // Root (Alpha) should NOT be selected as it's the parent, but range selection 
        // in our implementation is based on visual flattening. 
        // If Beta is inside Alpha, and Gamma is after Alpha, Alpha might be in the range.
        // However, the test specifically expects Alpha NOT to be selected.
        await expect(rootNode).not.toHaveClass(/selected/);

        // 4. Cleanup: bulk-delete the selected folders (Beta + Gamma)
        await page.getByTestId('bulk-delete-folders-button').click();
        await page.getByTestId('modal-confirm-button').click();
        // Wait for sidebar to re-render after delete before interacting again
        await expect(page.getByTestId('folder-name').filter({ hasText: siblingName })).not.toBeVisible({ timeout: 15000 });

        // Cleanup Root Alpha (still exists since it wasn't selected)
        // Use a fresh locator after the DOM re-render
        const freshRootNode = page.getByTestId('folder-name').filter({ hasText: rootName }).first();
        if (await freshRootNode.isVisible({ timeout: 5000 }).catch(() => false)) {
            await freshRootNode.click({ button: 'right' });
            await page.getByTestId('context-menu-delete-folder').click();
            await page.getByTestId('modal-confirm-button').click();
        }
        await expect(page.getByTestId('folder-name').filter({ hasText: rootName })).not.toBeVisible({ timeout: 15000 });
    });
});
