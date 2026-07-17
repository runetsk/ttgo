import { test, expect } from '../../fixtures/test.js';

test.describe('Recursive Folder Display', () => {
    const timestamp = Date.now();
    const parentFolderName = `Parent ${timestamp}`;
    const childFolderName = `Child ${timestamp}`;
    const parentTestName = `Parent Test ${timestamp}`;
    const childTestName = `Child Test ${timestamp}`;

    test.beforeEach(async ({ libraryPage }) => {
        await libraryPage.open();
    });

    test('should show tests from subfolders when parent folder is selected', async ({ libraryPage }) => {
        await test.step('Create the parent folder', async () => {
            await libraryPage.createRootFolder(parentFolderName);
        });

        await test.step('Select the parent folder and add a parent test', async () => {
            await libraryPage.selectFolder(parentFolderName);
            await libraryPage.createTestCase(parentTestName);
            await expect(libraryPage.testRow(parentTestName)).toBeVisible();
        });

        await test.step('Create a child folder under the parent', async () => {
            await libraryPage.createSubfolder(parentFolderName, childFolderName);
        });

        await test.step('Select the child folder and add a child test', async () => {
            await libraryPage.selectFolder(childFolderName);
            await libraryPage.createTestCase(childTestName);
            // In the child folder: the child test shows, the parent's does not.
            await expect(libraryPage.testRow(childTestName)).toBeVisible();
            await expect(libraryPage.testRow(parentTestName)).not.toBeVisible();
        });

        await test.step('Reselect the parent folder and verify both tests are visible', async () => {
            await libraryPage.selectFolder(parentFolderName);
            await expect(libraryPage.testRow(parentTestName)).toBeVisible();
            await expect(libraryPage.testRow(childTestName)).toBeVisible();
        });
    });
});
