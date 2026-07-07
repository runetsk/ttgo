import { test, expect } from '@playwright/test';
import { createFolderAPI, createTestAPI, createRunAPI, addRunResultAPI } from '../../helpers/api.js';

test.describe('New run modal — tree picker', () => {
    test('pick a folder in the tree, create the run, and see it in the sidebar', async ({ page, request }) => {
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

        await test.step('Ticking the folder selects both of its tests', async () => {
            await page.getByTestId('create-run-name-input').fill(runName);
            await page.getByTestId(`test-tree-folder-${folder.id}`).check();
            await expect(page.getByTestId('test-tree-selected-count')).toContainText('2');
        });

        await test.step('Create Run lands on the run detail with both tests', async () => {
            await page.getByTestId('create-run-submit').click();
            await expect(page).toHaveURL(/\/runs\/run\/[a-f0-9-]+$/);
            // stats-passed renders "{passed} / {total}" — both tests are in the run.
            await expect(page.getByTestId('stats-passed')).toContainText('/ 2');
        });

        await test.step('The new run appears in the left run sidebar without a refresh', async () => {
            const sidebar = page.getByTestId('run-folder-sidebar');
            const toggle = page.locator('[data-testid="uncategorised-entry"] .expand-toggle');
            const expanded = await toggle.evaluate(el => el.classList.contains('expanded'));
            if (!expanded) await toggle.click();
            await expect(sidebar.getByText(runName)).toBeVisible();
        });
    });

    test('run detail Execute runs only the checked results', async ({ page, request }) => {
        const stamp = Date.now();
        const names = ['Alpha exec', 'Beta exec', 'Gamma exec'];
        let run;
        const tc = {};

        await test.step('Seed a run with three cases via API', async () => {
            const folder = await createFolderAPI(request, `Exec Folder ${stamp}`);
            run = await createRunAPI(request, `Exec Run ${stamp}`);
            for (const n of names) {
                tc[n] = await createTestAPI(request, n, folder.id);
                await addRunResultAPI(request, run.id, tc[n].id);
            }
        });

        await test.step('With nothing checked, Execute carries no count', async () => {
            await page.goto(`/runs/run/${run.id}`);
            await expect(page.getByTestId('stats-passed')).toContainText('/ 3');
            await expect(page.getByTestId('execute-run-button')).not.toContainText('(');
        });

        await test.step('Checking two rows updates the Execute count to (2)', async () => {
            // The row checkbox is a custom-styled label wrapping a collapsed native
            // input; clicking the label natively toggles it and fires onChange.
            await page.getByTestId(`select-result-${tc['Beta exec'].id}`).click();
            await page.getByTestId(`select-result-${tc['Gamma exec'].id}`).click();
            await expect(page.getByTestId('execute-run-button')).toContainText('(2)');
        });

        await test.step('Execute scopes the queue to just those two tests', async () => {
            await page.getByTestId('execute-run-button').click();
            await expect(page).toHaveURL(/\/execute\?only=/);
            await expect(page.getByTestId('execute-progress')).toContainText('0 / 2');
            // Queue is sorted by name, so Beta is current and Alpha is absent.
            await expect(page.getByTestId('execute-current-name')).toHaveText('Beta exec');
        });
    });
});
