import { test, expect } from '@playwright/test';

test.describe('Test Runs Pagination', () => {
    const API_URL = 'http://localhost:8080/api';

    const createSuiteAPI = async (request, name) => {
        const res = await request.post(`${API_URL}/suites`, {
            data: { name: name, description: 'Pagination Test Suite' }
        });
        expect(res.ok()).toBeTruthy();
        return await res.json();
    };

    const createRunAPI = async (request, suiteId, name) => {
        const res = await request.post(`${API_URL}/runs`, {
            data: { suite_id: suiteId, name: name }
        });
        expect(res.ok()).toBeTruthy();
        return await res.json();
    };

    test('should paginate test runs correctly', async ({ page, request }) => {
        const timestamp = Date.now();
        const suiteName = `Pagination Suite ${timestamp}`;
        const suite = await createSuiteAPI(request, suiteName);

        // Create 25 runs to test pagination (default limit is 20)
        console.log('Creating 25 test runs sequentially...');
        for (let i = 1; i <= 25; i++) {
            await createRunAPI(request, suite.id, `Paginated Run ${i} ${timestamp}`);
        }

        await page.goto('/runs');
        await page.getByRole('button', { name: 'Column Filters' }).click();
        await page.getByTestId('filter-suite-select').selectOption({ label: suiteName });
        await page.waitForSelector('text=Showing');

        // 1. Verify default state (20 per page)
        // Component renders "Showing 1–20 of 25" format
        await expect(page.locator('[data-testid="page-size-selector"]')).toHaveValue('20');
        await expect(page.getByText(/Showing 1.20 of 25/)).toBeVisible();

        // 2. Verify "Next" button works
        await page.click('[data-testid="next-page"]');
        await expect(page.getByText(/Showing 21.25 of 25/)).toBeVisible();

        // 3. Verify "Prev" button works
        await page.click('[data-testid="prev-page"]');
        await expect(page.getByText(/Showing 1.20 of 25/)).toBeVisible();

        // 4. Change page size to 50 — all 25 fit on one page, no pagination buttons
        await page.selectOption('[data-testid="page-size-selector"]', '50');
        await expect(page.getByText(/Showing 1.25 of 25/)).toBeVisible();
        await expect(page.locator('[data-testid="next-page"]')).not.toBeVisible();

        // 5. Change page size to 10
        await page.selectOption('[data-testid="page-size-selector"]', '10');
        await expect(page.getByText(/Showing 1.10 of 25/)).toBeVisible();
        await expect(page.locator('[data-testid="next-page"]')).toBeVisible();
    });
});
