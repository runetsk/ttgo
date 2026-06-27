import { test, expect } from '@playwright/test';

const API_URL = 'http://localhost:8080/api';

test.describe('Deep Linking', () => {
    test.setTimeout(60000);

    test('should navigate to test detail via URL and persist', async ({ page }) => {
        const folderName = `DeepLink Folder ${Date.now()}`;
        const testName = `DeepLink Test ${Date.now()}`;

        // 1. Create Data
        await page.goto('/');
        await page.getByText('+ Root').click();
        await page.getByPlaceholder('Folder name').fill(folderName);
        await page.getByRole('button', { name: 'Confirm' }).click();

        await expect(page.getByTestId('folder-name').filter({ hasText: folderName })).toBeVisible();
        await page.getByTestId('folder-name').filter({ hasText: folderName }).click();

        await page.getByText('+ New Test').click();
        await page.getByPlaceholder('Test case name...').fill(testName);
        await page.getByRole('button', { name: 'Confirm' }).click();

        // 2. Click to Navigate
        await page.getByText(testName).click();

        // Verify URL
        expect(page.url()).toContain('/library/tests/');
        const testId = page.url().split('/library/tests/')[1];

        // Verify Detail Page
        await expect(page.locator(`input[value="${testName}"]`)).toBeVisible();

        // 3. Reload Page
        await page.reload();
        await expect(page.locator(`input[value="${testName}"]`)).toBeVisible();
        expect(page.url()).toContain(testId);

        // 4. Navigate Back (Close) — folder exists so navigates to /library/folders/:id
        await page.getByRole('button', { name: '×' }).click();
        await expect(page.url()).not.toContain('/library/tests/');
    });

    test('should show warning banner and folder picker when test folder has been deleted', async ({ page, request }) => {
        const folderName = 'Deleted Folder ' + Date.now();
        const testName = 'Orphaned Test ' + Date.now();

        // 1. Create folder and test via API
        const folderRes = await request.post(`${API_URL}/folders`, {
            data: { name: folderName, parent_id: null }
        });
        expect(folderRes.ok()).toBeTruthy();
        const folder = await folderRes.json();

        const testRes = await request.post(`${API_URL}/tests`, {
            data: { name: testName, folder_id: folder.id }
        });
        expect(testRes.ok()).toBeTruthy();
        const testCase = await testRes.json();

        // 2. Delete the folder via API
        const deleteRes = await request.delete(`${API_URL}/folders/${folder.id}`);
        expect(deleteRes.status()).toBe(204);

        // 3. Navigate directly to the orphaned test
        await page.goto(`/library/tests/${testCase.id}`);
        await page.waitForLoadState('domcontentloaded');

        // 4. Verify warning banner is displayed
        await expect(page.getByText("This test's folder has been deleted")).toBeVisible({ timeout: 10000 });

        // 5. Verify folder picker and disabled Move button are present
        const moveBtn = page.getByRole('button', { name: 'Move' });
        await expect(moveBtn).toBeVisible();
        await expect(moveBtn).toBeDisabled(); // disabled until a folder is selected

        // 6. Test still loads correctly — name input shows the test name
        await expect(page.getByTestId('test-case-name-input')).toHaveValue(testName);

        // 7. Close navigates to /library (no valid folder to return to)
        await page.getByTestId('close-modal-button').click();
        await expect(page).toHaveURL(/\/library$/);
    });

    test('should move orphaned test to a new folder via the warning banner picker', async ({ page, request }) => {
        const folderName = 'ToDelete Folder ' + Date.now();
        const targetFolderName = 'Target Folder ' + Date.now();
        const testName = 'Move-Me Test ' + Date.now();

        // 1. Create two folders and a test via API
        const delFolderRes = await request.post(`${API_URL}/folders`, { data: { name: folderName, parent_id: null } });
        expect(delFolderRes.ok()).toBeTruthy();
        const delFolder = await delFolderRes.json();

        const targetFolderRes = await request.post(`${API_URL}/folders`, { data: { name: targetFolderName, parent_id: null } });
        expect(targetFolderRes.ok()).toBeTruthy();

        const testRes = await request.post(`${API_URL}/tests`, { data: { name: testName, folder_id: delFolder.id } });
        expect(testRes.ok()).toBeTruthy();
        const testCase = await testRes.json();

        // 2. Delete the source folder
        await request.delete(`${API_URL}/folders/${delFolder.id}`);

        // 3. Navigate to orphaned test
        await page.goto(`/library/tests/${testCase.id}`);
        await page.waitForLoadState('domcontentloaded');
        await expect(page.getByText("This test's folder has been deleted")).toBeVisible({ timeout: 10000 });

        // 4. Select target folder and click Move
        const folderPicker = page.locator('select.meta-select').last();
        await folderPicker.selectOption({ label: targetFolderName });

        const moveBtn = page.getByRole('button', { name: 'Move' });
        await expect(moveBtn).toBeEnabled();
        await moveBtn.click();

        // 5. After move the warning banner disappears (folder now exists)
        await expect(page.getByText("This test's folder has been deleted")).not.toBeVisible({ timeout: 10000 });

        // 6. Sidebar should highlight the target folder
        await expect(page.locator('.folder-header.selected')).toContainText(targetFolderName);
    });
});
