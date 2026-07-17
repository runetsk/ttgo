import { test, expect } from '../../fixtures/test.js';

test.describe('Folder Deep Linking', () => {

    test('should navigate to folder via URL and persist', async ({ page, libraryPage }) => {
        const folderName = `DeepFolder ${Date.now()}`;
        let folderId;

        await test.step('Create a root folder', async () => {
            await libraryPage.open();
            await libraryPage.createRootFolder(folderName);
        });

        await test.step('Navigate into the folder and verify URL and grid header', async () => {
            await libraryPage.selectFolder(folderName);
            expect(page.url()).toContain('/library/folders/');
            folderId = page.url().split('/library/folders/')[1];
            // grid-title also holds a child rename (✏️) button, so assert containment.
            await expect(page.locator('h2.grid-title')).toContainText(folderName);
        });

        await test.step('Reload the page and verify the folder selection persists', async () => {
            await page.reload();
            await expect(page.locator('h2.grid-title')).toContainText(folderName);
            expect(page.url()).toContain(folderId);
        });

        await test.step('Deselect via the Tests nav and verify the grid empty state', async () => {
            await libraryPage.nav('Tests');
            await expect(page.url()).not.toContain('/library/folders/');
            await expect(page.url()).not.toContain('/library/tests/');
            await expect(page.getByText('Select folders to view tests')).toBeVisible();
        });
    });
});
