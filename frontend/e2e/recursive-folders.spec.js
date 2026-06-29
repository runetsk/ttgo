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
        let parentFolder;
        let childFolder;

        await test.step('Create the parent folder', async () => {
            await page.getByTestId('create-root-folder-button').click();
            await page.getByTestId('modal-input').fill(parentFolderName);
            await page.getByTestId('modal-confirm-button').click();

            parentFolder = page.getByTestId('folder-name').filter({ hasText: parentFolderName });
            await expect(parentFolder).toBeVisible();
        });

        await test.step('Select the parent folder and add a parent test', async () => {
            await parentFolder.click();
            await page.getByTestId('create-test-button').click();
            await page.getByTestId('modal-input').fill(parentTestName);
            await page.getByTestId('modal-confirm-button').click();
            await expect(page.getByTestId('test-row').filter({ hasText: parentTestName })).toBeVisible();
        });

        await test.step('Create a child folder under the parent', async () => {
            await parentFolder.click({ button: 'right' });
            await page.getByTestId('context-menu-create-subfolder').click();
            await page.getByTestId('modal-input').fill(childFolderName);
            await page.getByTestId('modal-confirm-button').click();

            childFolder = page.getByTestId('folder-name').filter({ hasText: childFolderName });
            await expect(childFolder).toBeVisible();
        });

        await test.step('Select the child folder and add a child test', async () => {
            await childFolder.click();
            await page.getByTestId('create-test-button').click();
            await page.getByTestId('modal-input').fill(childTestName);
            await page.getByTestId('modal-confirm-button').click();

            // In Child folder: Child Test should be visible, Parent Test should NOT
            await expect(page.getByTestId('test-row').filter({ hasText: childTestName })).toBeVisible();
            await expect(page.getByTestId('test-row').filter({ hasText: parentTestName })).not.toBeVisible();
        });

        await test.step('Reselect the parent folder and verify both tests are visible', async () => {
            await parentFolder.click();
            await expect(page.getByTestId('test-row').filter({ hasText: parentTestName })).toBeVisible();
            await expect(page.getByTestId('test-row').filter({ hasText: childTestName })).toBeVisible();
        });
    });
});
