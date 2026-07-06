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

    test('execution mode walks through the run queue', async ({ page, request }) => {
        const runName = 'Exec Run ' + Date.now();
        let run;

        await test.step('Seed a run with three cases via API', async () => {
            const folder = await createFolderAPI(request, 'Exec Folder ' + Date.now());
            run = await createRunAPI(request, runName);
            for (const name of ['Alpha check', 'Beta check', 'Gamma check']) {
                const tc = await createTestAPI(request, name, folder.id);
                await addRunResultAPI(request, run.id, tc.id);
            }
        });

        await test.step('Enter execution mode from the run header', async () => {
            await page.goto(`/runs/run/${run.id}`);
            await page.getByTestId('execute-run-button').click();
            await expect(page).toHaveURL(new RegExp(`/runs/run/${run.id}/execute$`));
        });

        await test.step('First pending test is shown with progress', async () => {
            await expect(page.getByTestId('execute-current-name')).toHaveText('Alpha check');
            await expect(page.getByTestId('execute-progress')).toContainText('0 / 3');
        });

        await test.step('Next / Prev navigate the queue', async () => {
            await page.getByTestId('execute-next').click();
            await expect(page.getByTestId('execute-current-name')).toHaveText('Beta check');
            await page.getByTestId('execute-prev').click();
            await expect(page.getByTestId('execute-current-name')).toHaveText('Alpha check');
        });

        await test.step('Sidebar jump selects a specific test', async () => {
            await page.getByText('Gamma check', { exact: true }).click();
            await expect(page.getByTestId('execute-current-name')).toHaveText('Gamma check');
        });
    });

    test('execution mode shows the authored steps of the current test', async ({ page, request }) => {
        const runName = 'Steps Run ' + Date.now();
        let run;

        await test.step('Seed a run whose test has two authored steps', async () => {
            const folder = await createFolderAPI(request, 'Steps Folder ' + Date.now());
            const tc = await createTestAPI(request, 'Stepped case', folder.id, 'API Test', {
                steps: [
                    { action: '<p>Open the login page</p>', expected_result: '<p>Form is visible</p>', order_index: 0 },
                    { action: '<p>Submit valid credentials</p>', expected_result: '<p>Dashboard loads</p>', order_index: 1 },
                ],
            });
            run = await createRunAPI(request, runName);
            await addRunResultAPI(request, run.id, tc.id);
        });

        await test.step('Steps render on the execute page', async () => {
            await page.goto(`/runs/run/${run.id}/execute`);
            await expect(page.getByTestId('execute-current-name')).toHaveText('Stepped case');
            const steps = page.getByTestId('execute-steps');
            await expect(steps.getByText('Open the login page')).toBeVisible();
            await expect(steps.getByText('Form is visible')).toBeVisible();
            await expect(steps.getByText('Submit valid credentials')).toBeVisible();
        });
    });
});
