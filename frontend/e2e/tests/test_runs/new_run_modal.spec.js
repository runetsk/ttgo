import { test, expect } from '@playwright/test';
import { createFolderAPI, createTestAPI } from '../../helpers/api.js';

test.describe('New run modal — tree picker + manual execute', () => {
    test('pick a folder in the tree, manual-execute the selected tests, run shows in sidebar', async ({ page, request }) => {
        const stamp = Date.now();
        const folderName = `Tree Folder ${stamp}`;
        const runName = `Tree Run ${stamp}`;
        const names = [`Tree-A ${stamp}`, `Tree-B ${stamp}`];
        let folder;

        await test.step('Seed a folder with two tests', async () => {
            folder = await createFolderAPI(request, folderName);
            for (const n of names) await createTestAPI(request, n, folder.id);
        });

        await test.step('Open the modal; the tree shows the folder and tests', async () => {
            await page.goto('/runs');
            await page.getByTestId('create-test-run-button').click();
            await expect(page.getByTestId('test-tree-picker')).toBeVisible();
            await expect(page.getByTestId(`test-tree-folder-${folder.id}`)).toBeVisible();
        });

        await test.step('Manual execute is disabled until a test is selected', async () => {
            await page.getByTestId('create-run-name-input').fill(runName);
            await expect(page.getByTestId('create-run-execute')).toBeDisabled();
            await page.getByTestId(`test-tree-folder-${folder.id}`).check();
            await expect(page.getByTestId('test-tree-selected-count')).toContainText('2');
            await expect(page.getByTestId('create-run-execute')).toBeEnabled();
        });

        await test.step('Manual execute lands on the execution screen with the selected tests', async () => {
            await page.getByTestId('create-run-execute').click();
            await expect(page).toHaveURL(/\/runs\/run\/[a-f0-9-]+\/execute$/);
            // Exactly the two selected tests are in the run's queue.
            await expect(page.getByTestId('execute-progress')).toContainText('0 / 2');
            // The queue is sorted by name, so the first (current) test is Tree-A.
            await expect(page.getByTestId('execute-current-name')).toHaveText(names[0]);
        });

        await test.step('The new run appears in the left run sidebar without a refresh', async () => {
            const sidebar = page.getByTestId('run-folder-sidebar');
            const toggle = page.locator('[data-testid="uncategorised-entry"] .expand-toggle');
            const expanded = await toggle.evaluate(el => el.classList.contains('expanded'));
            if (!expanded) await toggle.click();
            await expect(sidebar.getByText(runName)).toBeVisible();
        });
    });
});
