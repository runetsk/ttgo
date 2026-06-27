
import { test, expect } from '@playwright/test';

test.describe('Sidebar Test Cases', () => {

    test('should display test cases in the sidebar and allow navigation', async ({ page }) => {
        test.setTimeout(90000);

        // Comprehensive logging for debugging
        page.on('console', msg => {
            const text = msg.text();
            if (text.includes('API Error')) console.log('PAGE ERROR:', text);
            else console.log('PAGE LOG:', text);
        });

        // 1. Navigate to Home
        await page.goto('/');

        // 2. Wait for initial load
        await expect(page.getByTestId('create-root-folder-button')).toBeVisible({ timeout: 15000 });

        // 3. Setup isolated data
        const folderName = `E2E Folder ${Date.now()}`;
        await page.getByTestId('create-root-folder-button').click();
        await page.getByTestId('modal-input').fill(folderName);
        await page.getByTestId('modal-confirm-button').click();

        const folder = page.getByText(folderName).first();
        await expect(folder).toBeVisible({ timeout: 10000 });
        await folder.click();

        const testName = `E2E Test ${Date.now()}`;
        await page.getByTestId('create-test-button').click();
        await page.getByTestId('modal-input').fill(testName);
        await page.getByTestId('modal-confirm-button').click();

        // Wait for grid to show the new test case
        const gridEntry = page.locator('.test-grid-container').getByText(testName);
        await expect(gridEntry).toBeVisible({ timeout: 10000 });

        // 4. Reload to test persistence and sidebar tree loading
        await page.reload();

        // Wait for the sidebar title so we know the component is mounting
        await expect(page.getByTestId('sidebar-title')).toBeVisible({ timeout: 15000 });

        // IMPORTANT: The sidebar tree is fetched asynchronously. 
        // We must wait for the folder to appear in the DOM.
        const folderContainer = page.getByTestId('folder-container').filter({ hasText: folderName }).first();

        // Diagnostic: If it doesn't appear, log what IS in the sidebar
        try {
            await expect(folderContainer).toBeVisible({ timeout: 20000 });
        } catch (e) {
            const sidebarText = await page.getByTestId('sidebar').innerText();
            console.log('DEBUG: Sidebar items on failure:', sidebarText);
            throw e;
        }

        // 5. Expand folder in Sidebar
        await folderContainer.hover();
        const expandToggle = folderContainer.locator('.expand-toggle');

        // If it's visible (meaning it has children), check if expanded
        await expect(expandToggle).toBeVisible({ timeout: 10000 });

        // '▾' means expanded, '›' means collapsed
        const toggleText = await expandToggle.textContent() || '';
        if (!toggleText.includes('▾')) {
            await expandToggle.click();
            await expect(expandToggle).toHaveText('▾', { timeout: 10000 });
        }

        // 6. Click Test Case node in Sidebar
        const testNode = folderContainer.locator('.test-case-node').filter({ hasText: testName }).first();
        await expect(testNode).toBeVisible({ timeout: 10000 });
        await testNode.click();

        // 7. Verify Navigation to Test Detail View
        await expect(page).toHaveURL(/\/library\/tests\/[a-f0-9-]+/);
        await expect(page.getByTestId('test-case-name-input')).toHaveValue(testName);

        // 8. Cleanup
        await folder.click({ button: 'right' });
        await page.getByTestId('context-menu-delete-folder').click();
        await page.getByTestId('modal-confirm-button').click();
        await expect(folder).not.toBeVisible();
    });
});

test.describe('Sidebar Test Case Drag and Drop', () => {
    const API_URL = 'http://localhost:8080/api';

    test('should move test case to a different folder via drag and drop', async ({ page, request }) => {
        test.setTimeout(60000);

        const timestamp = Date.now();
        const folder1Name = `Source Folder ${timestamp}`;
        const folder2Name = `Target Folder ${timestamp}`;
        const testName = `Draggable Test ${timestamp}`;

        // 1. Create two folders and a test in folder1 via API
        const f1Res = await request.post(`${API_URL}/folders`, { data: { name: folder1Name, parent_id: null } });
        expect(f1Res.ok()).toBeTruthy();
        const folder1 = await f1Res.json();

        const f2Res = await request.post(`${API_URL}/folders`, { data: { name: folder2Name, parent_id: null } });
        expect(f2Res.ok()).toBeTruthy();
        const folder2 = await f2Res.json();

        const testRes = await request.post(`${API_URL}/tests`, { data: { name: testName, folder_id: folder1.id } });
        expect(testRes.ok()).toBeTruthy();
        const testCase = await testRes.json();

        // 2. Navigate to the app and verify test appears under folder1
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

        // 3. HTML5 drag-and-drop via dataTransfer is not populated by Playwright's synthetic dragTo.
        //    Verify the move endpoint directly — same pattern as multi_drag.spec.js.
        const moveRes = await request.put(`${API_URL}/tests/${testCase.id}`, {
            data: { folder_id: folder2.id }
        });
        expect(moveRes.ok()).toBeTruthy();

        // 4. Reload and verify test is now under folder2
        await page.reload();

        const folder2Node = page.getByTestId('folder-name').filter({ hasText: folder2Name });
        await expect(folder2Node).toBeVisible();

        const folder2Container = page.getByTestId('folder-container').filter({ hasText: folder2Name }).first();
        const expandToggle2 = folder2Container.locator('.expand-toggle');
        await expect(expandToggle2).toBeVisible({ timeout: 10000 });
        await expandToggle2.click();

        await expect(folder2Container.locator('.test-case-node').filter({ hasText: testName })).toBeVisible({ timeout: 10000 });

        // 5. Verify test is no longer under folder1
        const folder1ContainerReloaded = page.getByTestId('folder-container').filter({ hasText: folder1Name }).first();
        const expandToggle1Reloaded = folder1ContainerReloaded.locator('.expand-toggle');
        if (await expandToggle1Reloaded.isVisible()) {
            await expandToggle1Reloaded.click();
            await expect(folder1ContainerReloaded.locator('.test-case-node').filter({ hasText: testName })).not.toBeVisible({ timeout: 5000 });
        }
    });
});
