import { test, expect } from '../../fixtures/test.js';

test.describe('TestGrid typed column filters', () => {
    test('date range filter narrows the grid', async ({ page, libraryPage }) => {
        let testName;

        await test.step('Set up a folder with one test and open Column Filters', async () => {
            ({ testName } = await libraryPage.setupFolderAndTest('Filters'));
            await libraryPage.openColumnFilters();
        });

        await test.step('Verify the row is visible before filtering', async () => {
            await expect(libraryPage.testRow(testName)).toBeVisible();
        });

        await test.step('Apply a past date range and verify the test is excluded', async () => {
            await page.getByTestId('filter-created_at').click();
            await page.getByTestId('filter-created_at-from').fill('2000-01-01');
            await page.getByTestId('filter-created_at-to').fill('2000-01-02');
            await expect(libraryPage.testRow(testName)).toHaveCount(0);
        });

        await test.step('Apply the Today preset and verify the test reappears', async () => {
            await page.getByTestId('filter-created_at-preset-today').click();
            await expect(libraryPage.testRow(testName)).toBeVisible();
        });
    });
});
