import { test, expect } from '../../fixtures/test.js';
import { TIMEOUTS } from '../../config.js';

test.describe('Runs list typed column filters', () => {
    test('date range filter wires created_from/created_to into GET /api/runs request', async ({ page, runsPage }) => {
        let responsePromise;

        await test.step('Open the runs page and the Column Filters panel', async () => {
            await runsPage.open();
            await expect(page.getByRole('button', { name: 'Column Filters' })).toBeVisible();
            await runsPage.openColumnFilters();
        });

        await test.step('Arm the network waiter for both date params before any input', async () => {
            // Predicate requires BOTH params, so it won't resolve on the intermediate
            // request (from-only).
            responsePromise = page.waitForResponse(
                res => res.url().includes('/api/runs')
                    && res.url().includes('created_from=2099-01-01')
                    && res.url().includes('created_to=2099-12-31'),
                { timeout: TIMEOUTS.ELEMENT }
            );
        });

        await test.step('Open the Created date popover and fill both fields', async () => {
            await runsPage.openCreatedFilter();
            await runsPage.createdFrom.fill('2099-01-01');
            await runsPage.createdTo.fill('2099-12-31');
        });

        await test.step('Await the filtered response and verify the request params and status', async () => {
            const filteredResponse = await responsePromise;
            const url = filteredResponse.url();
            expect(url).toContain('created_from=2099-01-01');
            expect(url).toContain('created_to=2099-12-31');
            expect(filteredResponse.status()).toBe(200);
        });

        await test.step('Verify no run rows remain and the empty state shows', async () => {
            await expect(runsPage.runRows).toHaveCount(0);
            await expect(page.getByText('No runs found matching your criteria')).toBeVisible();
        });
    });
});
