import { test, expect } from '@playwright/test';

test('has title', async ({ page }) => {
    await test.step('Open the app and verify the page title', async () => {
        await page.goto('/');
        await expect(page).toHaveTitle(/TTGO/);
    });
});

test('sidebar has library heading', async ({ page }) => {
    await test.step('Open the app and verify the sidebar title is visible', async () => {
        await page.goto('/');
        const title = page.getByTestId('sidebar-title');
        await expect(title).toBeVisible({ timeout: 15000 });
    });
});
