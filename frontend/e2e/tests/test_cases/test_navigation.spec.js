import { test, expect } from '../../fixtures/test.js';

test.describe('Test Case Navigation & Sidebar Sync', () => {
    test('should sync sidebar selection when navigating to a test case', async ({ page, api, libraryPage, testCaseDetailPage }) => {
        const folderName = 'Nav Folder ' + Date.now();
        let folder;
        let testCase;

        await test.step('Create a folder and test case via API and open the app', async () => {
            folder = await api.createFolder(folderName);
            testCase = await api.createTest('Nav Test', folder.id);
            await libraryPage.open();
        });

        await test.step('Find the folder in the sidebar and select it', async () => {
            const targetFolder = libraryPage.folderNode(folderName);
            await expect(targetFolder).toBeVisible();
            await targetFolder.click();
        });

        await test.step('Click the test case in the grid', async () => {
            // Library grid rows are getByTestId('test-row'), not the run-detail row role.
            const testRow = libraryPage.testRow('Nav Test');
            await expect(testRow).toBeVisible();
            await testRow.getByText('Nav Test').click();
        });

        await test.step('Verify the URL and sidebar visibility', async () => {
            // A selected folder scopes the route to /library/folders/<id>/tests/<id>,
            // so match the /tests/<id> tail (present with or without a folder in context).
            await expect(page).toHaveURL(new RegExp(`/tests/${testCase.id}`));
            await expect(libraryPage.sidebar).toBeVisible();
        });

        await test.step('Verify the sidebar still has the correct folder selected', async () => {
            await expect(libraryPage.selectedFolder).toBeVisible();
            const selectedText = await libraryPage.selectedFolder.textContent();
            expect(selectedText).toContain(folderName);
        });

        await test.step('Click Cancel and verify return to the folder view', async () => {
            await testCaseDetailPage.cancel();
            await expect(page).toHaveURL(new RegExp(`/library/folders/${folder.id}`));
        });
    });

    test('should sync sidebar selection on deep link to test case', async ({ api, libraryPage, testCaseDetailPage }) => {
        const folderName = 'DeepLink Folder ' + Date.now();

        await test.step('Create a folder and test case via API and deep link to the test', async () => {
            const folder = await api.createFolder(folderName);
            const testCase = await api.createTest('DeepLink Test', folder.id);

            await testCaseDetailPage.open(testCase.id);
            await expect(libraryPage.sidebar).toBeVisible();
        });

        await test.step('Wait for sidebar sync and verify the correct folder is selected', async () => {
            await expect(libraryPage.selectedFolder).toBeVisible();
            const selectedText = await libraryPage.selectedFolder.textContent();
            expect(selectedText).toContain(folderName);
        });
    });
});
