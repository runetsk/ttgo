import { test, expect } from '../../fixtures/test.js';

test.describe('Test Grid Filtering & Selection', () => {
    test.beforeEach(async ({ libraryPage }) => {
        await libraryPage.open();
        // Select a folder to show the grid.
        await expect(libraryPage.firstFolder).toBeVisible();
        await libraryPage.firstFolder.click();
        await expect(libraryPage.testTable).toBeVisible();
    });

    test('should filter tests by search text', async ({ page, libraryPage }) => {
        const rows = libraryPage.testRows;
        const initialCount = await rows.count();
        if (initialCount === 0) return; // Skip if no tests

        let firstTestName;

        await test.step('Search for the first test name and verify only matching rows show', async () => {
            firstTestName = await rows.first().locator('div').first().textContent();

            await libraryPage.searchInput.fill(firstTestName);

            await expect(rows).toHaveCount(await rows.filter({ hasText: firstTestName }).count());
        });

        await test.step('Search for non-existent text and verify the empty state', async () => {
            await libraryPage.searchInput.fill('NonExistentTestXYZ123');
            await expect(page.getByText('No tests found matching your criteria')).toBeVisible();
            await expect(rows).toHaveCount(0);
        });
    });

    test('should select and deselect items with checkboxes', async ({ libraryPage }) => {
        const rows = libraryPage.testRows;
        if (await rows.count() < 2) return;

        const firstCheckbox = rows.first().locator('input[type="checkbox"]');
        const secondCheckbox = rows.nth(1).locator('input[type="checkbox"]');

        await test.step('Select the first row and verify one item selected', async () => {
            await firstCheckbox.click();
            await expect(libraryPage.bulkActionBar).toContainText('1 items selected');
        });

        await test.step('Select the second row and verify two items selected', async () => {
            await secondCheckbox.click();
            await expect(libraryPage.bulkActionBar).toContainText('2 items selected');
        });

        await test.step('Deselect the first row and verify one item selected', async () => {
            await firstCheckbox.click();
            await expect(libraryPage.bulkActionBar).toContainText('1 items selected');
        });

        await test.step('Deselect the last row and verify the bulk bar hides', async () => {
            await secondCheckbox.click();
            await expect(libraryPage.bulkActionBar).not.toBeVisible();
        });
    });

    test('should select all and deselect all via header checkbox', async ({ libraryPage }) => {
        const rows = libraryPage.testRows;
        const count = await rows.count();
        if (count === 0) return;

        await test.step('Select all rows via the header checkbox and verify the count', async () => {
            await libraryPage.selectAllCheckbox.click();
            await expect(libraryPage.bulkActionBar).toContainText(`${count} items selected`);
        });

        await test.step('Deselect all rows and verify the bulk bar hides', async () => {
            await libraryPage.selectAllCheckbox.click();
            await expect(libraryPage.bulkActionBar).not.toBeVisible();
        });
    });

    test('should clear selection after bulk action', async ({ api, libraryPage }) => {
        const ts = Date.now();

        await test.step('Seed an isolated folder with two test cases via API', async () => {
            const folder = await api.createFolder(`Bulk Clear Folder ${ts}`);
            await api.createTest(`Bulk Clear A ${ts}`, folder.id);
            await api.createTest(`Bulk Clear B ${ts}`, folder.id);
        });

        await test.step('Open the seeded folder and wait for its rows', async () => {
            await libraryPage.open();
            await libraryPage.folderNode(`Bulk Clear Folder ${ts}`).click();
            await expect(libraryPage.testTable).toBeVisible();
            await expect(libraryPage.testRows).toHaveCount(2);
        });

        await test.step('Select all rows and verify the bulk bar shows', async () => {
            await libraryPage.selectAllCheckbox.click();
            await expect(libraryPage.bulkActionBar).toBeVisible();
        });

        await test.step('Run a bulk delete and verify the selection clears', async () => {
            // The test-cases grid bulk bar exposes Delete (with a confirm modal);
            // confirming it clears the selection and hides the bar.
            await libraryPage.bulkDeleteTests();
            await expect(libraryPage.bulkActionBar).not.toBeVisible();
        });
    });
});
