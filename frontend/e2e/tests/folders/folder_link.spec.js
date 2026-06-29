import { test, expect } from '@playwright/test';

test.describe('Folder Deep Linking', () => {
    test.setTimeout(60000);

    test('should navigate to folder via URL and persist', async ({ page }) => {
        const folderName = `DeepFolder ${Date.now()}`;
        let folderId;

        await test.step('Create a root folder', async () => {
            await page.goto('/');
            await page.getByTestId('create-root-folder-button').click();
            await page.getByPlaceholder('Folder name').fill(folderName);
            await page.getByRole('button', { name: 'Confirm' }).click();

            await expect(page.getByTestId('folder-name').filter({ hasText: folderName })).toBeVisible();
        });

        await test.step('Navigate into the folder and verify URL and grid header', async () => {
            await page.getByTestId('folder-name').filter({ hasText: folderName }).click();

            // Verify URL
            expect(page.url()).toContain('/library/folders/');
            folderId = page.url().split('/library/folders/')[1];

            // Verify Test Grid Header matches folder name. The <h2.grid-title>
            // also holds a child rename (✏️) button, so assert containment.
            await expect(page.locator('h2.grid-title')).toContainText(folderName);
        });

        await test.step('Reload the page and verify the folder selection persists', async () => {
            await page.reload();
            await expect(page.locator('h2.grid-title')).toContainText(folderName);
            expect(page.url()).toContain(folderId);
        });

        await test.step('Deselect via the Tests nav and verify the grid empty state', async () => {
            // The top-nav "Tests" button navigates to /library (clears selection).
            await page.getByRole('button', { name: 'Tests', exact: true }).click();
            await expect(page.url()).not.toContain('/library/folders/');
            await expect(page.url()).not.toContain('/library/tests/');

            // Verify Grid Empty State
            await expect(page.getByText('Select folders to view tests')).toBeVisible();
        });
    });
});
