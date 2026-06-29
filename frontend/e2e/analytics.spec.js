import { test, expect } from '@playwright/test';

test.describe('Analytics Dashboard (US2)', () => {
  test('analytics page is accessible via nav', async ({ page }) => {
    await test.step('Navigate to analytics from the home page and verify the URL', async () => {
      await page.goto('/');
      await page.getByRole('button', { name: /analytics/i }).click();
      await expect(page).toHaveURL('/analytics');
    });
  });

  test('analytics dashboard shows summary cards', async ({ page }) => {
    await test.step('Open the analytics page and verify the summary cards are visible', async () => {
      await page.goto('/analytics');
      await expect(page.getByText('Total Runs')).toBeVisible();
      await expect(page.getByText('Pass Rate')).toBeVisible();
    });
  });

  test('analytics page renders without errors on empty data', async ({ page }) => {
    const errors = [];
    await test.step('Open the analytics page and capture any page errors', async () => {
      await page.goto('/analytics');
      page.on('pageerror', e => errors.push(e.message));
      await page.waitForTimeout(500);
    });
    await test.step('Verify no page errors were thrown', async () => {
      expect(errors).toHaveLength(0);
    });
  });
});
