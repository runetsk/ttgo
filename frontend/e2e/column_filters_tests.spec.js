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
        const { testName } = await setupFolderAndTest(page);
        await page.getByRole('button', { name: 'Column Filters' }).click();

        // Row visible before filtering.
        await expect(page.getByTestId('test-row').filter({ hasText: testName })).toBeVisible();

        // A past range excludes the just-created test.
        await page.getByTestId('filter-created_at').click();
        await page.getByTestId('filter-created_at-from').fill('2000-01-01');
        await page.getByTestId('filter-created_at-to').fill('2000-01-02');
        await expect(page.getByTestId('test-row').filter({ hasText: testName })).toHaveCount(0);

        // "Today" preset brings it back.
        await page.getByTestId('filter-created_at-preset-today').click();
        await expect(page.getByTestId('test-row').filter({ hasText: testName })).toBeVisible();
    });

    test('QTest Status column is absent when QTest integration is off', async ({ page }) => {
        await setupFolderAndTest(page);
        await page.getByRole('button', { name: 'Columns' }).click();
        const popover = page.getByRole('dialog', { name: 'Column visibility' });
        await expect(popover).toBeVisible();
        await expect(popover.getByText('QTest Status')).toHaveCount(0);
    });
});
