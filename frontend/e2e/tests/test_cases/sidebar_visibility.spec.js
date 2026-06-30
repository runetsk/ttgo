import { test, expect } from '@playwright/test';

test.describe('Sidebar Visibility', () => {

    test('should show/hide sidebar based on route', async ({ page }) => {
        await test.step('Open the home page and confirm the sidebar is visible', async () => {
            await page.goto('/');

            // 1. Home (Tests) should have sidebar
            await expect(page.locator('.sidebar')).toBeVisible();
        });

        await test.step('Navigate to Categories and confirm the sidebar is hidden', async () => {
            // 2. Categories should NOT have sidebar
            await page.getByRole('button', { name: 'Categories' }).click();
            await expect(page.url()).toContain('/categories');
            await expect(page.locator('.sidebar')).not.toBeVisible();
        });

        await test.step('Navigate to Settings and confirm the sidebar is hidden', async () => {
            // 3. Settings should NOT have sidebar
            await page.getByRole('button', { name: 'Settings' }).click();
            await expect(page.url()).toContain('/settings');
            await expect(page.locator('.sidebar')).not.toBeVisible();
        });

        await test.step('Return to the Tests page and confirm the sidebar is visible', async () => {
            // 4. Back to Home -> Sidebar Visible
            await page.getByRole('button', { name: 'Tests' }).click();
            await expect(page.locator('.sidebar')).toBeVisible();
        });

        await test.step('Create and open a folder and confirm the sidebar is visible', async () => {
            // 5. Folder -> Sidebar Visible
            const folderName = `SidebarTest ${Date.now()}`;
            await page.getByTestId('create-root-folder-button').click();
            await page.getByTestId('modal-input').fill(folderName);
            await page.getByTestId('modal-confirm-button').click();
            await page.getByTestId('folder-name').filter({ hasText: folderName }).click();

            await expect(page.url()).toContain('/library/folders/');
            await expect(page.locator('.sidebar')).toBeVisible();
        });
    });
});
