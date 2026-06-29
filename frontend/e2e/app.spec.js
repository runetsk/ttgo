import { test, expect } from '@playwright/test';

test('has title', async ({ page }) => {
    await page.goto('/');
    await expect(page).toHaveTitle(/TTGO/);
});

test('sidebar has library heading', async ({ page }) => {
    await page.goto('/');
    const title = page.getByTestId('sidebar-title');
    await expect(title).toBeVisible({ timeout: 15000 });
});
