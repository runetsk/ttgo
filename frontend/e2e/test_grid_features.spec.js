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

        let firstTestName;
        const searchInput = page.locator('.modern-input[placeholder="Quick search..."]');

        await test.step('Search for the first test name and verify only matching rows show', async () => {
            firstTestName = await rows.first().locator('div').first().textContent();

            await searchInput.fill(firstTestName);

            // Should only show matching tests
            await expect(rows).toHaveCount(await rows.filter({ hasText: firstTestName }).count());
        });

        await test.step('Search for non-existent text and verify the empty state', async () => {
            await searchInput.fill('NonExistentTestXYZ123');
            await expect(page.getByText('No tests found matching your criteria')).toBeVisible();
            await expect(rows).toHaveCount(0);
        });
    });

    test('should select and deselect items with checkboxes', async ({ page }) => {
        const rows = page.getByTestId('test-row');
        if (await rows.count() < 2) return;

        const firstCheckbox = rows.first().locator('input[type="checkbox"]');
        const secondCheckbox = rows.nth(1).locator('input[type="checkbox"]');

        await test.step('Select the first row and verify one item selected', async () => {
            await firstCheckbox.click();
            await expect(page.locator('.bulk-action-bar')).toContainText('1 items selected');
        });

        await test.step('Select the second row and verify two items selected', async () => {
            await secondCheckbox.click();
            await expect(page.locator('.bulk-action-bar')).toContainText('2 items selected');
        });

        await test.step('Deselect the first row and verify one item selected', async () => {
            await firstCheckbox.click();
            await expect(page.locator('.bulk-action-bar')).toContainText('1 items selected');
        });

        await test.step('Deselect the last row and verify the bulk bar hides', async () => {
            await secondCheckbox.click();
            await expect(page.locator('.bulk-action-bar')).not.toBeVisible();
        });
    });

    test('should select all and deselect all via header checkbox', async ({ page }) => {
        const rows = page.getByTestId('test-row');
        const count = await rows.count();
        if (count === 0) return;

        const selectAllCheckbox = page.locator('thead input[type="checkbox"]');

        await test.step('Select all rows via the header checkbox and verify the count', async () => {
            await selectAllCheckbox.click();
            await expect(page.locator('.bulk-action-bar')).toContainText(`${count} items selected`);
        });

        await test.step('Deselect all rows and verify the bulk bar hides', async () => {
            await selectAllCheckbox.click();
            await expect(page.locator('.bulk-action-bar')).not.toBeVisible();
        });
    });

    test('should clear selection after bulk action', async ({ page }) => {
        const rows = page.getByTestId('test-row');
        if (await rows.count() === 0) return;

        await test.step('Select all rows and verify the bulk bar shows', async () => {
            await page.locator('thead input[type="checkbox"]').click();
            await expect(page.locator('.bulk-action-bar')).toBeVisible();
        });

        await test.step('Run Bulk Pass and verify the selection clears', async () => {
            await page.getByRole('button', { name: 'Bulk Pass' }).click();

            // Bar should disappear and selection should be empty
            await expect(page.locator('.bulk-action-bar')).not.toBeVisible();
        });
    });
});
