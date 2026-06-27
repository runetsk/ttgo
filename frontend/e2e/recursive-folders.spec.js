import { test, expect } from '@playwright/test';

test.describe('Recursive Folder Display', () => {
    const timestamp = Date.now();
    const parentFolderName = `Parent ${timestamp}`;
    const childFolderName = `Child ${timestamp}`;
    const parentTestName = `Parent Test ${timestamp}`;
    const childTestName = `Child Test ${timestamp}`;

    test.beforeEach(async ({ page }) => {
        await page.goto('/');
    });

    test('should show tests from subfolders when parent folder is selected', async ({ page }) => {
        // 1. Create Parent Folder
        await page.getByTestId('create-root-folder-button').click();
        await page.getByTestId('modal-input').fill(parentFolderName);
        await page.getByTestId('modal-confirm-button').click();

        const parentFolder = page.getByTestId('folder-name').filter({ hasText: parentFolderName });
        await expect(parentFolder).toBeVisible();

        // 2. Select Parent and Add Parent Test
        await parentFolder.click();
        await page.getByTestId('create-test-button').click();
        await page.getByTestId('modal-input').fill(parentTestName);
        await page.getByTestId('modal-confirm-button').click();
        await expect(page.getByTestId('test-row').filter({ hasText: parentTestName })).toBeVisible();

        // 3. Create Child Folder under Parent
        await parentFolder.click({ button: 'right' });
        await page.getByTestId('context-menu-create-subfolder').click();
        await page.getByTestId('modal-input').fill(childFolderName);
        await page.getByTestId('modal-confirm-button').click();

        const childFolder = page.getByTestId('folder-name').filter({ hasText: childFolderName });
        await expect(childFolder).toBeVisible();

        // 4. Select Child and Add Child Test
        await childFolder.click();
        await page.getByTestId('create-test-button').click();
        await page.getByTestId('modal-input').fill(childTestName);
        await page.getByTestId('modal-confirm-button').click();

        // In Child folder: Child Test should be visible, Parent Test should NOT
        await expect(page.getByTestId('test-row').filter({ hasText: childTestName })).toBeVisible();
        await expect(page.getByTestId('test-row').filter({ hasText: parentTestName })).not.toBeVisible();

        // 5. Select Parent again and verify both are visible
        await parentFolder.click();
        await expect(page.getByTestId('test-row').filter({ hasText: parentTestName })).toBeVisible();
        await expect(page.getByTestId('test-row').filter({ hasText: childTestName })).toBeVisible();
    });
});
