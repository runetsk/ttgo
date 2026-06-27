import { test, expect } from '@playwright/test';

test.describe('Folder Deep Linking', () => {
    test.setTimeout(60000);

    test('should navigate to folder via URL and persist', async ({ page }) => {
        const folderName = `DeepFolder ${Date.now()}`;

        // 1. Create Folder
        await page.goto('/');
        await page.getByText('+ Root').click();
        await page.getByPlaceholder('Folder name').fill(folderName);
        await page.getByRole('button', { name: 'Confirm' }).click();

        await expect(page.getByTestId('folder-name').filter({ hasText: folderName })).toBeVisible();

        // 2. Click to Navigate
        await page.getByTestId('folder-name').filter({ hasText: folderName }).click();

        // Verify URL
        expect(page.url()).toContain('/library/folders/');
        const folderId = page.url().split('/library/folders/')[1];

        // Verify Test Grid Header matches folder name
        await expect(page.locator('h2.grid-title')).toHaveText(folderName);

        // 3. Reload Page
        await page.reload();
        await expect(page.locator('h2.grid-title')).toHaveText(folderName);
        expect(page.url()).toContain(folderId);

        // 4. Deselect (Navigate to /)
        // Clicking "Library" or logo should go to /
        await page.getByText('TestTracker').click();
        await expect(page.url()).not.toContain('/library/folders/');
        await expect(page.url()).not.toContain('/library/tests/');

        // Verify Grid Empty State
        await expect(page.getByText('Select folders to view tests')).toBeVisible();
    });
});
