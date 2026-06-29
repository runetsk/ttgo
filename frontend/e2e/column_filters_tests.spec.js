import { test, expect } from '@playwright/test';

const LS_KEY_TESTS = 'ttgo_columns_test-cases';

async function setupFolderAndTest(page) {
    const ts = Date.now();
    const folderName = `Filters ${ts}`;
    const testName = `TC ${ts}`;
    await page.goto('/');
    await page.evaluate((k) => localStorage.removeItem(k), LS_KEY_TESTS);
    await page.getByTestId('create-root-folder-button').click();
    await page.getByTestId('modal-input').fill(folderName);
    await page.getByTestId('modal-confirm-button').click();
    const folder = page.getByTestId('folder-name').filter({ hasText: folderName }).first();
    await expect(folder).toBeVisible();
    await folder.click();
    await page.getByTestId('create-test-button').click();
    await page.getByTestId('modal-input').fill(testName);
    await page.getByTestId('modal-confirm-button').click();
    await expect(page.getByTestId('test-table')).toBeVisible();
    return { testName };
}

test.describe('TestGrid typed column filters', () => {
    test('date range filter narrows the grid', async ({ page }) => {
        let testName;

        await test.step('Set up a folder with one test and open Column Filters', async () => {
            ({ testName } = await setupFolderAndTest(page));
            await page.getByRole('button', { name: 'Column Filters' }).click();
        });

        await test.step('Verify the row is visible before filtering', async () => {
            await expect(page.getByTestId('test-row').filter({ hasText: testName })).toBeVisible();
        });

        await test.step('Apply a past date range and verify the test is excluded', async () => {
            await page.getByTestId('filter-created_at').click();
            await page.getByTestId('filter-created_at-from').fill('2000-01-01');
            await page.getByTestId('filter-created_at-to').fill('2000-01-02');
            await expect(page.getByTestId('test-row').filter({ hasText: testName })).toHaveCount(0);
        });

        await test.step('Apply the Today preset and verify the test reappears', async () => {
            await page.getByTestId('filter-created_at-preset-today').click();
            await expect(page.getByTestId('test-row').filter({ hasText: testName })).toBeVisible();
        });
    });
});
