import { test, expect } from '@playwright/test';

test.describe('Test Grid Filtering & Selection', () => {
    test.beforeEach(async ({ page }) => {
        await page.goto('/');
        // Select a folder to show the grid
        const firstFolder = page.getByTestId('folder-name').first();
        await expect(firstFolder).toBeVisible();
        await firstFolder.click();
        await expect(page.getByTestId('test-table')).toBeVisible();
    });

    test('should filter tests by search text', async ({ page }) => {
        const rows = page.getByTestId('test-row');
        const initialCount = await rows.count();
        if (initialCount === 0) return; // Skip if no tests

        const firstTestName = await rows.first().locator('div').first().textContent();

        const searchInput = page.locator('.modern-input[placeholder="Quick search..."]');
        await searchInput.fill(firstTestName);

        // Should only show matching tests
        await expect(rows).toHaveCount(await rows.filter({ hasText: firstTestName }).count());

        // Search for non-existent text
        await searchInput.fill('NonExistentTestXYZ123');
        await expect(page.getByText('No tests found matching your criteria')).toBeVisible();
        await expect(rows).toHaveCount(0);
    });

    test('should select and deselect items with checkboxes', async ({ page }) => {
        const rows = page.getByTestId('test-row');
        if (await rows.count() < 2) return;

        const firstCheckbox = rows.first().locator('input[type="checkbox"]');
        const secondCheckbox = rows.nth(1).locator('input[type="checkbox"]');

        // Select one
        await firstCheckbox.click();
        await expect(page.locator('.bulk-action-bar')).toContainText('1 items selected');

        // Select another
        await secondCheckbox.click();
        await expect(page.locator('.bulk-action-bar')).toContainText('2 items selected');

        // Deselect one
        await firstCheckbox.click();
        await expect(page.locator('.bulk-action-bar')).toContainText('1 items selected');

        // Deselect all
        await secondCheckbox.click();
        await expect(page.locator('.bulk-action-bar')).not.toBeVisible();
    });

    test('should select all and deselect all via header checkbox', async ({ page }) => {
        const rows = page.getByTestId('test-row');
        const count = await rows.count();
        if (count === 0) return;

        const selectAllCheckbox = page.locator('thead input[type="checkbox"]');

        // Select All
        await selectAllCheckbox.click();
        await expect(page.locator('.bulk-action-bar')).toContainText(`${count} items selected`);

        // Deselect All
        await selectAllCheckbox.click();
        await expect(page.locator('.bulk-action-bar')).not.toBeVisible();
    });

    test('should clear selection after bulk action', async ({ page }) => {
        const rows = page.getByTestId('test-row');
        if (await rows.count() === 0) return;

        await page.locator('thead input[type="checkbox"]').click();
        await expect(page.locator('.bulk-action-bar')).toBeVisible();

        // Bulk Pass
        await page.getByRole('button', { name: 'Bulk Pass' }).click();

        // Bar should disappear and selection should be empty
        await expect(page.locator('.bulk-action-bar')).not.toBeVisible();
    });
});
