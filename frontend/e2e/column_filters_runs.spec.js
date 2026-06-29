import { test, expect } from '@playwright/test';

test.describe('Runs list typed column filters', () => {
    test('date range filter wires created_from/created_to into GET /api/runs request', async ({ page }) => {
        let responsePromise;

        await test.step('Open the runs page and the Column Filters panel', async () => {
            await page.goto('/runs');
            await expect(page.getByRole('button', { name: 'Column Filters' })).toBeVisible();
            await page.getByRole('button', { name: 'Column Filters' }).click();
        });

        await test.step('Arm the network waiter for both date params before any input', async () => {
            // Set up the network waiter BEFORE any input — predicate requires BOTH params,
            // so it won't resolve on the intermediate request (from-only).
            responsePromise = page.waitForResponse(
                res => res.url().includes('/api/runs')
                    && res.url().includes('created_from=2099-01-01')
                    && res.url().includes('created_to=2099-12-31'),
                { timeout: 10000 }
            );
        });

        await test.step('Open the Created date popover and fill both fields', async () => {
            await page.getByTestId('filter-run-created_at').click();
            await page.getByTestId('filter-run-created_at-from').fill('2099-01-01');
            await page.getByTestId('filter-run-created_at-to').fill('2099-12-31');
        });

        await test.step('Await the filtered response and verify the request params and status', async () => {
            // Wait for the filtered response — proves server-side wiring
            const filteredResponse = await responsePromise;
            const url = filteredResponse.url();
            expect(url).toContain('created_from=2099-01-01');
            expect(url).toContain('created_to=2099-12-31');
            expect(filteredResponse.status()).toBe(200);
        });

        await test.step('Verify no run rows remain and the empty state shows', async () => {
            // After the response lands, actual run-data rows (data-testid="run-row-<id>") should be 0.
            // (The empty-state <tr> is a separate locator.)
            await expect(page.locator('[data-testid^="run-row-"]')).toHaveCount(0);
            // The empty state message is visible
            await expect(page.getByText('No runs found matching your criteria')).toBeVisible();
        });
    });
});
