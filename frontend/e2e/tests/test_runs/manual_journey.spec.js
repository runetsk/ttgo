import { test, expect } from '@playwright/test';
import { API_URL } from '../../config.js';
import {
    createFolderAPI,
    createTestAPI,
    createRunAPI,
    addRunResultAPI,
} from '../../helpers/api.js';

test.describe('Manual run journey', () => {
    test('creating a run lands on its detail page', async ({ page }) => {
        const runName = 'Journey Run ' + Date.now();

        await test.step('Create an empty run via the UI modal', async () => {
            await page.goto('/runs');
            await page.getByTestId('create-test-run-button').click();
            await page.getByTestId('create-run-name-input').fill(runName);
            await page.getByTestId('create-run-submit').click();
        });

        await test.step('The app navigates into the new run', async () => {
            await expect(page).toHaveURL(/\/runs\/run\/[a-f0-9-]+$/);
            await expect(page.getByTestId('run-title')).toHaveText(runName);
        });
    });

    test('complete and reopen a run from the header', async ({ page, request }) => {
        const runName = 'Completable Run ' + Date.now();
        let run;

        await test.step('Seed a run with one passing result via API', async () => {
            const folder = await createFolderAPI(request, 'Complete Folder ' + Date.now());
            const tc = await createTestAPI(request, 'Complete case', folder.id);
            run = await createRunAPI(request, runName);
            await addRunResultAPI(request, run.id, tc.id, { status: 'PASS' });
        });

        await test.step('Complete the run from the header', async () => {
            await page.goto(`/runs/run/${run.id}`);
            await page.getByTestId('complete-run-button').click();
            await expect(page.getByTestId('run-status-select')).toHaveValue('PASS');
            await expect(page.getByTestId('complete-run-button')).not.toBeVisible();
        });

        await test.step('Reopen the run from the header', async () => {
            await page.getByTestId('reopen-run-button').click();
            await expect(page.getByTestId('run-status-select')).toHaveValue('RUNNING');
            await expect(page.getByTestId('reopen-run-button')).not.toBeVisible();
        });
    });
});
