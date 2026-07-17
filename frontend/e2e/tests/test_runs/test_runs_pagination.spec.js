import { test, expect } from '../../fixtures/test.js';

test.describe('Test Runs Pagination', () => {
    test('should paginate test runs correctly', async ({ page, runsPage, api }) => {
        let category;
        let timestamp;

        await test.step('Seed a category and create 25 runs via API', async () => {
            timestamp = Date.now();
            category = await api.createCategory(`Pagination Category ${timestamp}`);

            // 25 runs exceeds the default page limit of 20, so pagination engages.
            for (let i = 1; i <= 25; i++) {
                await api.createRun(`Paginated Run ${i} ${timestamp}`, { categoryId: category.id });
            }
        });

        await test.step('Open the runs page and filter by the seeded category', async () => {
            await runsPage.open();
            await runsPage.openColumnFilters();
            await runsPage.filterByCategory(category.id);
            await page.waitForSelector('text=Showing');
        });

        await test.step('Verify the default state shows 20 per page', async () => {
            await expect(runsPage.pageSizeSelector).toHaveValue('20');
            await expect(page.getByText(/Showing 1.20 of 25/)).toBeVisible();
        });

        await test.step('Verify the Next button advances to the second page', async () => {
            await runsPage.nextPage();
            await expect(page.getByText(/Showing 21.25 of 25/)).toBeVisible();
        });

        await test.step('Verify the Prev button returns to the first page', async () => {
            await runsPage.prevPage();
            await expect(page.getByText(/Showing 1.20 of 25/)).toBeVisible();
        });

        await test.step('Change page size to 50 and verify all runs fit on one page', async () => {
            await runsPage.pageSizeSelector.selectOption('50');
            await expect(page.getByText(/Showing 1.25 of 25/)).toBeVisible();
            await expect(runsPage.nextPageButton).not.toBeVisible();
        });

        await test.step('Change page size to 10 and verify pagination reappears', async () => {
            await runsPage.pageSizeSelector.selectOption('10');
            await expect(page.getByText(/Showing 1.10 of 25/)).toBeVisible();
            await expect(runsPage.nextPageButton).toBeVisible();
        });
    });
});
