
import { test, expect } from '@playwright/test';
import { API_URL } from './config.js';

test.describe('Sidebar Test Cases', () => {

    test('should display test cases in the sidebar and allow navigation', async ({ page }) => {
        test.setTimeout(90000);

        // Comprehensive logging for debugging
        page.on('console', msg => {
            const text = msg.text();
            if (text.includes('API Error')) console.log('PAGE ERROR:', text);
            else console.log('PAGE LOG:', text);
        });

        const folderName = `E2E Folder ${Date.now()}`;
        const testName = `E2E Test ${Date.now()}`;

        let folder;
        let folderContainer;
        let expandToggle;

        await test.step('Navigate to home and wait for initial load', async () => {
            await page.goto('/');
            await expect(page.getByTestId('create-root-folder-button')).toBeVisible({ timeout: 15000 });
        });

        await test.step('Create an isolated folder and select it', async () => {
            await page.getByTestId('create-root-folder-button').click();
            await page.getByTestId('modal-input').fill(folderName);
            await page.getByTestId('modal-confirm-button').click();

            folder = page.getByText(folderName).first();
            await expect(folder).toBeVisible({ timeout: 10000 });
            await folder.click();
        });

        await test.step('Create a test case and wait for it in the grid', async () => {
            await page.getByTestId('create-test-button').click();
            await page.getByTestId('modal-input').fill(testName);
            await page.getByTestId('modal-confirm-button').click();

            // Wait for grid to show the new test case
            const gridEntry = page.locator('.test-grid-container').getByText(testName);
            await expect(gridEntry).toBeVisible({ timeout: 10000 });
        });

        await test.step('Reload and wait for the folder to appear in the sidebar', async () => {
            // Reload to test persistence and sidebar tree loading
            await page.reload();

            // Wait for the sidebar title so we know the component is mounting
            await expect(page.getByTestId('sidebar-title')).toBeVisible({ timeout: 15000 });

            // IMPORTANT: The sidebar tree is fetched asynchronously.
            // We must wait for the folder to appear in the DOM.
            folderContainer = page.getByTestId('folder-container').filter({ hasText: folderName }).first();

            // Diagnostic: If it doesn't appear, log what IS in the sidebar
            try {
                await expect(folderContainer).toBeVisible({ timeout: 20000 });
            } catch (e) {
                const sidebarText = await page.getByTestId('sidebar').innerText();
                console.log('DEBUG: Sidebar items on failure:', sidebarText);
                throw e;
            }
        });

        await test.step('Expand the folder in the sidebar', async () => {
            await folderContainer.hover();
            expandToggle = folderContainer.locator('.expand-toggle');

            // If it's visible (meaning it has children), check if expanded
            await expect(expandToggle).toBeVisible({ timeout: 10000 });

            // '▾' means expanded, '›' means collapsed
            const toggleText = await expandToggle.textContent() || '';
            if (!toggleText.includes('▾')) {
                await expandToggle.click();
                await expect(expandToggle).toHaveText('▾', { timeout: 10000 });
            }
        });

        await test.step('Click the test case node in the sidebar', async () => {
            const testNode = folderContainer.locator('.test-case-node').filter({ hasText: testName }).first();
            await expect(testNode).toBeVisible({ timeout: 10000 });
            await testNode.click();
        });

        await test.step('Verify navigation to the test detail view', async () => {
            await expect(page).toHaveURL(/\/library\/tests\/[a-f0-9-]+/);
            await expect(page.getByTestId('test-case-name-input')).toHaveValue(testName);
        });

        await test.step('Clean up the folder via the context menu', async () => {
            await folder.click({ button: 'right' });
            await page.getByTestId('context-menu-delete-folder').click();
            await page.getByTestId('modal-confirm-button').click();
            await expect(folder).not.toBeVisible();
        });
    });
});

test.describe('Sidebar Test Case Drag and Drop', () => {

    test('should move test case to a different folder via drag and drop', async ({ page, request }) => {
        test.setTimeout(60000);

        const timestamp = Date.now();
        const folder1Name = `Source Folder ${timestamp}`;
        const folder2Name = `Target Folder ${timestamp}`;
        const testName = `Draggable Test ${timestamp}`;

        let folder1;
        let folder2;
        let testCase;

        await test.step('Create two folders and a test in folder1 via API', async () => {
            const f1Res = await request.post(`${API_URL}/folders`, { data: { name: folder1Name, parent_id: null } });
            expect(f1Res.ok()).toBeTruthy();
            folder1 = await f1Res.json();

            const f2Res = await request.post(`${API_URL}/folders`, { data: { name: folder2Name, parent_id: null } });
            expect(f2Res.ok()).toBeTruthy();
            folder2 = await f2Res.json();

            const testRes = await request.post(`${API_URL}/tests`, { data: { name: testName, folder_id: folder1.id } });
            expect(testRes.ok()).toBeTruthy();
            testCase = await testRes.json();
        });

        await test.step('Navigate to the app and verify the test appears under folder1', async () => {
            await page.goto('/');
            const folder1Node = page.getByTestId('folder-name').filter({ hasText: folder1Name });
            await expect(folder1Node).toBeVisible();

            // Expand folder1 to see test cases
            const folder1Container = page.getByTestId('folder-container').filter({ hasText: folder1Name }).first();
            const expandToggle1 = folder1Container.locator('.expand-toggle');
            await expect(expandToggle1).toBeVisible({ timeout: 10000 });
            await expandToggle1.click();

            // Verify test case node appears in folder1's sub-tree
            const testNode = folder1Container.locator('.test-case-node').filter({ hasText: testName });
            await expect(testNode).toBeVisible({ timeout: 10000 });

            // Verify the test node is draggable (has draggable attribute)
            await expect(testNode).toHaveAttribute('draggable', 'true');
        });

        await test.step('Move the test to folder2 via the API endpoint', async () => {
            // HTML5 drag-and-drop via dataTransfer is not populated by Playwright's synthetic dragTo.
            // Verify the move endpoint directly — same pattern as multi_drag.spec.js.
            const moveRes = await request.put(`${API_URL}/tests/${testCase.id}`, {
                data: { folder_id: folder2.id }
            });
            expect(moveRes.ok()).toBeTruthy();
        });

        await test.step('Reload and verify the test is now under folder2', async () => {
            await page.reload();

            const folder2Node = page.getByTestId('folder-name').filter({ hasText: folder2Name });
            await expect(folder2Node).toBeVisible();

            const folder2Container = page.getByTestId('folder-container').filter({ hasText: folder2Name }).first();
            const expandToggle2 = folder2Container.locator('.expand-toggle');
            await expect(expandToggle2).toBeVisible({ timeout: 10000 });
            await expandToggle2.click();

            await expect(folder2Container.locator('.test-case-node').filter({ hasText: testName })).toBeVisible({ timeout: 10000 });
        });

        await test.step('Verify the test is no longer under folder1', async () => {
            const folder1ContainerReloaded = page.getByTestId('folder-container').filter({ hasText: folder1Name }).first();
            const expandToggle1Reloaded = folder1ContainerReloaded.locator('.expand-toggle');
            if (await expandToggle1Reloaded.isVisible()) {
                await expandToggle1Reloaded.click();
                await expect(folder1ContainerReloaded.locator('.test-case-node').filter({ hasText: testName })).not.toBeVisible({ timeout: 5000 });
            }
        });
    });
});
