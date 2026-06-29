import { test, expect } from '@playwright/test';
import { API_URL } from './config.js';

test.describe('Test Runs Pagination', () => {
    const createCategoryAPI = async (request, name) => {
        const res = await request.post(`${API_URL}/categories`, {
            data: { name: name, description: 'Pagination Test Category' }
        });
        expect(res.ok()).toBeTruthy();
        return await res.json();
    };

    const createRunAPI = async (request, categoryId, name) => {
        const res = await request.post(`${API_URL}/runs`, {
            data: { category_id: categoryId, name: name }
        });
        expect(res.ok()).toBeTruthy();
        return await res.json();
    };

    test('should paginate test runs correctly', async ({ page, request }) => {
        let category;
        let timestamp;

        await test.step('Seed a category and create 25 runs via API', async () => {
            timestamp = Date.now();
            const categoryName = `Pagination Category ${timestamp}`;
            category = await createCategoryAPI(request, categoryName);

            // Create 25 runs to test pagination (default limit is 20)
            console.log('Creating 25 test runs sequentially...');
            for (let i = 1; i <= 25; i++) {
                await createRunAPI(request, category.id, `Paginated Run ${i} ${timestamp}`);
            }
        });

        await test.step('Open the runs page and filter by the seeded category', async () => {
            await page.goto('/runs');
            await page.getByRole('button', { name: 'Column Filters' }).click();

            // Open the CategoryFilter popover and select our category
            await page.getByTestId('filter-run-category').click();
            await page.getByTestId(`filter-run-category-option-${category.id}`).click();
            // Close the popover by pressing Escape
            await page.keyboard.press('Escape');

            await page.waitForSelector('text=Showing');
        });

        await test.step('Verify the default state shows 20 per page', async () => {
            // 1. Verify default state (20 per page)
            // Component renders "Showing 1–20 of 25" format
            await expect(page.locator('[data-testid="page-size-selector"]')).toHaveValue('20');
            await expect(page.getByText(/Showing 1.20 of 25/)).toBeVisible();
        });

        await test.step('Verify the Next button advances to the second page', async () => {
            // 2. Verify "Next" button works
            await page.click('[data-testid="next-page"]');
            await expect(page.getByText(/Showing 21.25 of 25/)).toBeVisible();
        });

        await test.step('Verify the Prev button returns to the first page', async () => {
            // 3. Verify "Prev" button works
            await page.click('[data-testid="prev-page"]');
            await expect(page.getByText(/Showing 1.20 of 25/)).toBeVisible();
        });

        await test.step('Change page size to 50 and verify all runs fit on one page', async () => {
            // 4. Change page size to 50 — all 25 fit on one page, no pagination buttons
            await page.selectOption('[data-testid="page-size-selector"]', '50');
            await expect(page.getByText(/Showing 1.25 of 25/)).toBeVisible();
            await expect(page.locator('[data-testid="next-page"]')).not.toBeVisible();
        });

        await test.step('Change page size to 10 and verify pagination reappears', async () => {
            // 5. Change page size to 10
            await page.selectOption('[data-testid="page-size-selector"]', '10');
            await expect(page.getByText(/Showing 1.10 of 25/)).toBeVisible();
            await expect(page.locator('[data-testid="next-page"]')).toBeVisible();
        });
    });
});
