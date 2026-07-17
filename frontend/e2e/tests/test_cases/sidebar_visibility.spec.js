import { test, expect } from '../../fixtures/test.js';

test.describe('Sidebar Visibility', () => {

    test('should show/hide sidebar based on route', async ({ page, libraryPage }) => {
        await test.step('Open the home page and confirm the sidebar is visible', async () => {
            await libraryPage.open();
            await expect(page.locator('.sidebar')).toBeVisible();
        });

        await test.step('Navigate to Categories and confirm the sidebar is hidden', async () => {
            // Categories lives under the top-nav "Quality" section.
            await libraryPage.nav('Quality');
            await libraryPage.nav('Categories');
            await expect(page.url()).toContain('/categories');
            await expect(page.locator('.sidebar')).not.toBeVisible();
        });

        await test.step('Navigate to Settings and confirm the sidebar is hidden', async () => {
            await libraryPage.nav('Settings');
            await expect(page.url()).toContain('/settings');
            await expect(page.locator('.sidebar')).not.toBeVisible();
        });

        await test.step('Return to the Tests page and confirm the sidebar is visible', async () => {
            await libraryPage.nav('Tests');
            await expect(page.locator('.sidebar')).toBeVisible();
        });

        await test.step('Create and open a folder and confirm the sidebar is visible', async () => {
            const folderName = `SidebarTest ${Date.now()}`;
            await libraryPage.createRootFolder(folderName);
            await libraryPage.selectFolder(folderName);

            await expect(page.url()).toContain('/library/folders/');
            await expect(page.locator('.sidebar')).toBeVisible();
        });
    });
});
