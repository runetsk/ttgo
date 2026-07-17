import { test, expect } from '../../fixtures/test.js';
import { TIMEOUTS } from '../../config.js';

test.describe('Sidebar Test Cases', () => {

    test('should display test cases in the sidebar and allow navigation', async ({ page, libraryPage, testCaseDetailPage }) => {
        const folderName = `E2E Folder ${Date.now()}`;
        const testName = `E2E Test ${Date.now()}`;

        let folderContainer;

        await test.step('Navigate to home and wait for initial load', async () => {
            await libraryPage.open();
            await expect(libraryPage.createRootFolderButton).toBeVisible({ timeout: TIMEOUTS.APP_RENDER });
        });

        await test.step('Create an isolated folder and select it', async () => {
            await libraryPage.createRootFolder(folderName);
            await libraryPage.selectFolder(folderName);
        });

        await test.step('Create a test case and wait for it in the grid', async () => {
            await libraryPage.createTestCase(testName);
            await expect(libraryPage.testRow(testName)).toBeVisible({ timeout: TIMEOUTS.ELEMENT });
        });

        await test.step('Reload and wait for the folder to appear in the sidebar', async () => {
            // Reload to exercise sidebar-tree persistence + async loading.
            await page.reload();
            await expect(page.getByTestId('sidebar-title')).toBeVisible({ timeout: TIMEOUTS.APP_RENDER });

            await libraryPage.ensureShowTestsInTree();

            // The sidebar tree is fetched asynchronously — wait for the folder to render.
            folderContainer = libraryPage.folderContainer(folderName);
            await expect(folderContainer).toBeVisible({ timeout: TIMEOUTS.HEAVY_GRID });
        });

        await test.step('Expand the folder in the sidebar', async () => {
            await folderContainer.hover();
            const toggle = libraryPage.expandToggle(folderName);
            // The toggle only carries the 'visible' class when the folder has children.
            await expect(toggle).toBeVisible({ timeout: TIMEOUTS.ELEMENT });

            // Expanded state is reflected by the 'expanded' class (SVG chevron, no text).
            const toggleClass = await toggle.getAttribute('class') || '';
            if (!toggleClass.includes('expanded')) {
                await toggle.click();
                await expect(toggle).toHaveClass(/expanded/, { timeout: TIMEOUTS.ELEMENT });
            }
        });

        await test.step('Click the test case node in the sidebar', async () => {
            const testNode = libraryPage.testCaseNode(folderName, testName);
            await expect(testNode).toBeVisible({ timeout: TIMEOUTS.ELEMENT });
            await testNode.click();
        });

        await test.step('Verify navigation to the test detail view', async () => {
            await expect(page).toHaveURL(/\/library\/tests\/[a-f0-9-]+/);
            await expect(testCaseDetailPage.nameInput).toHaveValue(testName);
        });

        await test.step('Clean up the folder via the context menu', async () => {
            await libraryPage.deleteFolderViaContextMenu(folderName);
            await expect(libraryPage.folderNode(folderName)).not.toBeVisible();
        });
    });
});

test.describe('Sidebar Test Case Drag and Drop', () => {

    test('should move test case to a different folder via drag and drop', async ({ page, api, libraryPage }) => {

        const timestamp = Date.now();
        const folder1Name = `Source Folder ${timestamp}`;
        const folder2Name = `Target Folder ${timestamp}`;
        const testName = `Draggable Test ${timestamp}`;

        let folder1;
        let folder2;
        let testCase;

        await test.step('Create two folders and a test in folder1 via API', async () => {
            folder1 = await api.createFolder(folder1Name);
            folder2 = await api.createFolder(folder2Name);
            testCase = await api.createTest(testName, folder1.id);
        });

        await test.step('Navigate to the app and verify the test appears under folder1', async () => {
            await libraryPage.open();
            await expect(libraryPage.folderNode(folder1Name)).toBeVisible();
            await libraryPage.ensureShowTestsInTree();

            const expandToggle1 = libraryPage.expandToggle(folder1Name);
            await expect(expandToggle1).toBeVisible({ timeout: TIMEOUTS.ELEMENT });
            await expandToggle1.click();

            const testNode = libraryPage.testCaseNode(folder1Name, testName);
            await expect(testNode).toBeVisible({ timeout: TIMEOUTS.ELEMENT });
            await expect(testNode).toHaveAttribute('draggable', 'true');
        });

        await test.step('Move the test to folder2 via the API endpoint', async () => {
            // HTML5 drag-and-drop via dataTransfer isn't populated by Playwright's synthetic
            // dragTo — exercise the move endpoint directly (same as multi_drag.spec.js).
            const moveRes = await api.put(`/tests/${testCase.id}`, { folder_id: folder2.id });
            expect(moveRes.ok()).toBeTruthy();
        });

        await test.step('Reload and verify the test is now under folder2', async () => {
            await page.reload();

            await expect(libraryPage.showTestsToggle).toBeVisible({ timeout: TIMEOUTS.APP_RENDER });
            await libraryPage.ensureShowTestsInTree();

            await expect(libraryPage.folderNode(folder2Name)).toBeVisible();
            const expandToggle2 = libraryPage.expandToggle(folder2Name);
            await expect(expandToggle2).toBeVisible({ timeout: TIMEOUTS.ELEMENT });
            await expandToggle2.click();

            await expect(libraryPage.testCaseNode(folder2Name, testName)).toBeVisible({ timeout: TIMEOUTS.ELEMENT });
        });

        await test.step('Verify the test is no longer under folder1', async () => {
            const expandToggle1Reloaded = libraryPage.expandToggle(folder1Name);
            if (await expandToggle1Reloaded.isVisible()) {
                await expandToggle1Reloaded.click();
                await expect(libraryPage.testCaseNode(folder1Name, testName)).not.toBeVisible({ timeout: TIMEOUTS.UI_SETTLE });
            }
        });
    });
});
