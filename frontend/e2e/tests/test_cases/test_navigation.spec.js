import { test, expect } from '@playwright/test';
import { createFolderAPI, createTestAPI } from '../../helpers/api.js';

test.describe('Test Case Navigation & Sidebar Sync', () => {
    test('should sync sidebar selection when navigating to a test case', async ({ page, request }) => {
        const folderName = 'Nav Folder ' + Date.now();
        let folder;
        let testCase;

        await test.step('Create a folder and test case via API and open the app', async () => {
            folder = await createFolderAPI(request, folderName);
            testCase = await createTestAPI(request, 'Nav Test', folder.id);

            await page.goto('/');
        });

        await test.step('Find the folder in the sidebar and select it', async () => {
            // Note: Filters by text, ensure uniqueness or strict match if possible
            const targetFolder = page.getByTestId('folder-name').filter({ hasText: folderName });
            await expect(targetFolder).toBeVisible();
            await targetFolder.click();
        });

        await test.step('Click the test case in the grid', async () => {
            // Find row by test name to be precise
            const testRow = page.getByRole('row', { name: 'Nav Test' });
            await expect(testRow).toBeVisible();

            // Click the Name cell (or just the row if clickable, but grid often has specific click handlers)
            // Usually index 2 is Name. Or use text locator.
            // As per previous test, we target the cell or link.
            await testRow.locator('td').filter({ hasText: 'Nav Test' }).click();
        });

        await test.step('Verify the URL and sidebar visibility', async () => {
            // Selecting a folder first scopes the route to /library/folders/<id>/tests/<id>,
            // so match the /tests/<id> tail (present whether or not a folder is in context).
            await expect(page).toHaveURL(new RegExp(`/tests/${testCase.id}`));
            await expect(page.getByTestId('sidebar')).toBeVisible();
        });

        await test.step('Verify the sidebar still has the correct folder selected', async () => {
            const selectedSidebarItem = page.locator('.folder-header.selected');
            await expect(selectedSidebarItem).toBeVisible();

            const selectedText = await selectedSidebarItem.textContent();
            expect(selectedText).toContain(folderName);
        });

        await test.step('Click Cancel and verify return to the folder view', async () => {
            await page.getByRole('button', { name: 'Cancel' }).click();
            await expect(page).toHaveURL(new RegExp(`/library/folders/${folder.id}`));
        });
    });

    test('should sync sidebar selection on deep link to test case', async ({ page, request }) => {
        const folderName = 'DeepLink Folder ' + Date.now();

        await test.step('Create a folder and test case via API and deep link to the test', async () => {
            const folder = await createFolderAPI(request, folderName);
            const testCase = await createTestAPI(request, 'DeepLink Test', folder.id);

            // Deep link directly
            const testUrl = `/library/tests/${testCase.id}`;
            await page.goto(testUrl);

            await expect(page.getByTestId('sidebar')).toBeVisible();
        });

        await test.step('Wait for sidebar sync and verify the correct folder is selected', async () => {
            await expect(page.locator('.folder-header.selected')).toBeVisible();
            const selectedText = await page.locator('.folder-header.selected').textContent();
            expect(selectedText).toContain(folderName);
        });
    });
});
