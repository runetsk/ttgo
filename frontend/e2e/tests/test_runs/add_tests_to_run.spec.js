import { test, expect } from '@playwright/test';
import { createFolderAPI, createTestAPI, createRunAPI, addRunResultAPI } from '../../helpers/api.js';

test.describe('Add tests to an open run — tree picker', () => {
    test('locks tests already in the run and adds a newly picked one', async ({ page, request }) => {
        const stamp = Date.now();
        const names = [`Add-A ${stamp}`, `Add-B ${stamp}`];
        let run;
        const tc = {};

        await test.step('Seed a folder with two tests; a run holding only the first', async () => {
            const folder = await createFolderAPI(request, `Add Folder ${stamp}`);
            run = await createRunAPI(request, `Add Run ${stamp}`);
            for (const n of names) tc[n] = await createTestAPI(request, n, folder.id);
            await addRunResultAPI(request, run.id, tc[names[0]].id); // A already in the run
        });

        await test.step('Open the run — only test A is present', async () => {
            await page.goto(`/runs/run/${run.id}`);
            await expect(page.getByTestId('stats-passed')).toContainText('/ 1');
        });

        await test.step('Open the add modal — test A is locked (checked + disabled)', async () => {
            await page.getByTestId('add-test-to-run-button').click();
            await expect(page.getByTestId('test-tree-picker')).toBeVisible();
            const lockedA = page.getByTestId(`test-tree-test-${tc[names[0]].id}`);
            await expect(lockedA).toBeChecked();
            await expect(lockedA).toBeDisabled();
        });

        await test.step('Tick test B and add it', async () => {
            await page.getByTestId(`test-tree-test-${tc[names[1]].id}`).check();
            await expect(page.getByTestId('test-tree-selected-count')).toContainText('1');
            await page.getByTestId('add-tests-submit').click();
        });

        await test.step('The run now holds both tests', async () => {
            await expect(page.getByTestId('test-tree-picker')).not.toBeVisible(); // modal closed
            await expect(page.getByTestId(`select-result-${tc[names[1]].id}`)).toBeVisible();
            await expect(page.getByTestId('stats-passed')).toContainText('/ 2');
        });
    });
});
