import { test, expect } from '../../fixtures/test.js';
import { TIMEOUTS } from '../../config.js';

test('has title', async ({ page, libraryPage }) => {
    await test.step('Open the app and verify the page title', async () => {
        await libraryPage.open();
        await expect(page).toHaveTitle(/TTGO/);
    });
});

test('sidebar has library heading', async ({ page, libraryPage }) => {
    await test.step('Open the app and verify the sidebar title is visible', async () => {
        await libraryPage.open();
        await expect(page.getByTestId('sidebar-title')).toBeVisible({ timeout: TIMEOUTS.APP_RENDER });
    });
});
