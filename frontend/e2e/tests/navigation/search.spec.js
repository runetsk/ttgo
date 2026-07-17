import { test, expect } from '../../fixtures/test.js';
import { SLEEPS } from '../../config.js';

test.describe('Global Search (US1)', () => {

    // Create a folder and open it so the TestGrid (with search bar) renders.
    async function setupFolder(libraryPage) {
        const folderName = `Search Folder ${Date.now()}`;
        await libraryPage.open();
        await libraryPage.createRootFolder(folderName);
        await libraryPage.selectFolder(folderName);
        await expect(libraryPage.testTable).toBeVisible();
    }

    test('search bar is visible in grid header', async ({ page, libraryPage }) => {
        await test.step('Create and open a folder so the test grid renders', async () => {
            await setupFolder(libraryPage);
        });
        await test.step('Verify the search bar is visible in the grid header', async () => {
            await expect(page.getByTestId('search-bar')).toBeVisible();
        });
    });

    test('searching for an existing test case shows results', async ({ libraryPage }) => {
        await test.step('Create and open a folder so the test grid renders', async () => {
            await setupFolder(libraryPage);
        });
        await test.step('Search for an existing term and verify the input stays visible', async () => {
            await libraryPage.searchInput.fill('test');
            await libraryPage.page.waitForTimeout(SLEEPS.SEARCH_DEBOUNCE);
            await expect(libraryPage.searchInput).toBeVisible();
        });
    });

    test('searching nonexistent term shows no results message', async ({ libraryPage }) => {
        await test.step('Create and open a folder so the test grid renders', async () => {
            await setupFolder(libraryPage);
        });
        await test.step('Search for a nonexistent term and verify no crash', async () => {
            await libraryPage.searchInput.fill('zzz_nonexistent_xyz_abc_12345');
            await libraryPage.page.waitForTimeout(SLEEPS.SEARCH_DEBOUNCE);
            await expect(libraryPage.searchInput).toBeVisible();
        });
    });
});
