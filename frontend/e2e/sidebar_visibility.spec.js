import { test, expect } from '@playwright/test';

test.describe('Sidebar Visibility', () => {
    test.setTimeout(30000);

    test('should show/hide sidebar based on route', async ({ page }) => {
        await page.goto('/');

        // 1. Home (Tests) should have sidebar
        await expect(page.locator('.sidebar')).toBeVisible();

        // 2. Suites should NOT have sidebar
        await page.getByRole('button', { name: 'Suites' }).click();
        await expect(page.url()).toContain('/suites');
        await expect(page.locator('.sidebar')).not.toBeVisible();

        // 3. Settings should NOT have sidebar
        await page.getByRole('button', { name: 'Settings' }).click();
        await expect(page.url()).toContain('/settings');
        await expect(page.locator('.sidebar')).not.toBeVisible();

        // 4. Back to Home -> Sidebar Visible
        await page.getByRole('button', { name: 'Tests' }).click();
        await expect(page.locator('.sidebar')).toBeVisible();

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
