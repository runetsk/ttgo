import { test, expect } from '@playwright/test';

test.describe('Runs list typed column filters', () => {
    test('date range filter wires created_from/created_to into GET /api/runs request', async ({ page }) => {
        await page.goto('/runs');
        await expect(page.getByRole('button', { name: 'Column Filters' })).toBeVisible();
        await page.getByRole('button', { name: 'Column Filters' }).click();

        // Set up the network waiter BEFORE any input — predicate requires BOTH params,
        // so it won't resolve on the intermediate request (from-only).
        const responsePromise = page.waitForResponse(
            res => res.url().includes('/api/runs')
                && res.url().includes('created_from=2099-01-01')
                && res.url().includes('created_to=2099-12-31'),
            { timeout: 10000 }
        );

        // Open the Created date popover and fill both fields
        await page.getByTestId('filter-run-created_at').click();
        await page.getByTestId('filter-run-created_at-from').fill('2099-01-01');
        await page.getByTestId('filter-run-created_at-to').fill('2099-12-31');

        // Wait for the filtered response — proves server-side wiring
        const filteredResponse = await responsePromise;
        const url = filteredResponse.url();
        expect(url).toContain('created_from=2099-01-01');
        expect(url).toContain('created_to=2099-12-31');
        expect(filteredResponse.status()).toBe(200);

        // After the response lands, actual run-data rows (data-testid="run-row-<id>") should be 0.
        // (The empty-state <tr> is a separate locator.)
        await expect(page.locator('[data-testid^="run-row-"]')).toHaveCount(0);
        // The empty state message is visible
        await expect(page.getByText('No runs found matching your criteria')).toBeVisible();
    });
});
