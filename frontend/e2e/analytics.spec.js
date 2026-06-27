import { test, expect } from '@playwright/test';

test.describe('Analytics Dashboard (US2)', () => {
  test('analytics page is accessible via nav', async ({ page }) => {
    await page.goto('/');
    await page.getByRole('button', { name: /analytics/i }).click();
    await expect(page).toHaveURL('/analytics');
  });

  test('analytics dashboard shows summary cards', async ({ page }) => {
    await page.goto('/analytics');
    await expect(page.getByText('Total Runs')).toBeVisible();
    await expect(page.getByText('Pass Rate')).toBeVisible();
  });

  test('analytics page renders without errors on empty data', async ({ page }) => {
    await page.goto('/analytics');
    const errors = [];
    page.on('pageerror', e => errors.push(e.message));
    await page.waitForTimeout(500);
    expect(errors).toHaveLength(0);
  });
});
