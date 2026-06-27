import { test, expect } from '@playwright/test';

test.describe('Test Case Navigation & Sidebar Sync', () => {
    const API_URL = 'http://localhost:8080/api';

    const createFolderAPI = async (request, name) => {
        const res = await request.post(`${API_URL}/folders`, {
            data: { name: name, parent_id: null }
        });
        expect(res.ok()).toBeTruthy();
        return await res.json();
    }

    const createTestAPI = async (request, name, folderId) => {
        const res = await request.post(`${API_URL}/tests`, {
            data: { name: name, folder_id: folderId, description: 'API Test' }
        });
        expect(res.ok()).toBeTruthy();
        return await res.json();
    }

    test('should sync sidebar selection when navigating to a test case', async ({ page, request }) => {
        const folderName = 'Nav Folder ' + Date.now();
        const folder = await createFolderAPI(request, folderName);
        const testCase = await createTestAPI(request, 'Nav Test', folder.id);

        await page.goto('/');

        // 1. Find the folder in the sidebar
        // Note: Filters by text, ensure uniqueness or strict match if possible
        const targetFolder = page.getByTestId('folder-name').filter({ hasText: folderName });
        await expect(targetFolder).toBeVisible();
        await targetFolder.click();

        // 2. Click on the test case in the grid
        // Find row by test name to be precise
        const testRow = page.getByRole('row', { name: 'Nav Test' });
        await expect(testRow).toBeVisible();

        // Click the Name cell (or just the row if clickable, but grid often has specific click handlers)
        // Usually index 2 is Name. Or use text locator.
        // As per previous test, we target the cell or link.
        await testRow.locator('td').filter({ hasText: 'Nav Test' }).click();

        // 3. Verify URL and sidebar visibility
        await expect(page).toHaveURL(new RegExp(`/library/tests/${testCase.id}`));
        await expect(page.getByTestId('sidebar')).toBeVisible();

        // 4. Verify sidebar still has the correct folder selected
        const selectedSidebarItem = page.locator('.folder-header.selected');
        await expect(selectedSidebarItem).toBeVisible();

        const selectedText = await selectedSidebarItem.textContent();
        expect(selectedText).toContain(folderName);

        // 5. Click Cancel and verify return to folder view
        await page.getByRole('button', { name: 'Cancel' }).click();
        await expect(page).toHaveURL(new RegExp(`/library/folders/${folder.id}`));
    });

    test('should sync sidebar selection on deep link to test case', async ({ page, request }) => {
        const folderName = 'DeepLink Folder ' + Date.now();
        const folder = await createFolderAPI(request, folderName);
        const testCase = await createTestAPI(request, 'DeepLink Test', folder.id);

        // Deep link directly
        const testUrl = `/library/tests/${testCase.id}`;
        await page.goto(testUrl);

        await expect(page.getByTestId('sidebar')).toBeVisible();

        // Wait for sidebar sync
        await expect(page.locator('.folder-header.selected')).toBeVisible();
        const selectedText = await page.locator('.folder-header.selected').textContent();
        expect(selectedText).toContain(folderName);
    });
});
