import { test, expect } from '@playwright/test';
import { API_URL } from '../../config.js';
import {
    createFolderAPI,
    createTestAPI,
    createRunAPI,
    addRunResultAPI,
    getRunAPI,
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

    test('verdicts advance the queue and persist details', async ({ page, request }) => {
        const runName = 'Verdict Run ' + Date.now();
        let run;

        await test.step('Seed a run with three cases via API', async () => {
            const folder = await createFolderAPI(request, 'Verdict Folder ' + Date.now());
            run = await createRunAPI(request, runName);
            for (const name of ['V-Alpha', 'V-Beta', 'V-Gamma']) {
                const tc = await createTestAPI(request, name, folder.id);
                await addRunResultAPI(request, run.id, tc.id);
            }
        });

        await test.step('Pass the first test — auto-advances to the second', async () => {
            await page.goto(`/runs/run/${run.id}/execute`);
            await expect(page.getByTestId('execute-current-name')).toHaveText('V-Alpha');
            await page.getByTestId('execute-pass').click();
            await expect(page.getByTestId('execute-current-name')).toHaveText('V-Beta');
            await expect(page.getByTestId('execute-progress')).toContainText('1 / 3');
        });

        await test.step('Fail the second test with a note and defect type', async () => {
            await page.getByTestId('execute-fail').click();
            await page.getByTestId('execute-defect-type').selectOption('product_bug');
            await page.getByTestId('execute-fail-note').fill('Button did nothing');
            await page.getByTestId('execute-fail-confirm').click();
            await expect(page.getByTestId('execute-current-name')).toHaveText('V-Gamma');
        });

        await test.step('Skip the last test — completion banner appears', async () => {
            await page.getByTestId('execute-skip').click();
            await expect(page.getByTestId('execute-done-banner')).toBeVisible();
            await expect(page.getByTestId('execute-progress')).toContainText('3 / 3');
        });

        await test.step('Verdicts persisted to the API', async () => {
            const fresh = await getRunAPI(request, run.id);
            const byName = Object.fromEntries(fresh.run_results.map(r => [r.test_name_snapshot, r]));
            if (byName['V-Alpha'].status !== 'PASS') throw new Error('V-Alpha not PASS');
            if (byName['V-Beta'].status !== 'FAIL') throw new Error('V-Beta not FAIL');
            if (byName['V-Beta'].defect_type !== 'product_bug') throw new Error('defect_type not saved');
            if (byName['V-Beta'].error_message !== 'Button did nothing') throw new Error('error_message not saved');
            if (byName['V-Gamma'].status !== 'SKIP') throw new Error('V-Gamma not SKIP');
            if (fresh.status !== 'RUNNING') throw new Error('run did not auto-start');
        });
    });

    test('keyboard-driven execution and run completion', async ({ page, request }) => {
        const runName = 'Keyboard Run ' + Date.now();
        let run;

        await test.step('Seed a run with two cases via API', async () => {
            const folder = await createFolderAPI(request, 'Kbd Folder ' + Date.now());
            run = await createRunAPI(request, runName);
            for (const name of ['K-One', 'K-Two']) {
                const tc = await createTestAPI(request, name, folder.id);
                await addRunResultAPI(request, run.id, tc.id);
            }
        });

        await test.step('Pass both tests with the P key', async () => {
            await page.goto(`/runs/run/${run.id}/execute`);
            await expect(page.getByTestId('execute-current-name')).toHaveText('K-One');
            await page.keyboard.press('p');
            await expect(page.getByTestId('execute-current-name')).toHaveText('K-Two');
            await page.keyboard.press('p');
            await expect(page.getByTestId('execute-done-banner')).toBeVisible();
        });

        await test.step('Verdict keys are disarmed once the run is fully executed', async () => {
            await page.keyboard.press('s');
            await page.keyboard.press('f');
            await expect(page.getByTestId('execute-fail-note')).not.toBeVisible();
            const fresh = await getRunAPI(request, run.id);
            const byName = Object.fromEntries(fresh.run_results.map(r => [r.test_name_snapshot, r]));
            if (byName['K-One'].status !== 'PASS') throw new Error('K-One was mutated by a stray key');
            if (byName['K-Two'].status !== 'PASS') throw new Error('K-Two was mutated by a stray key');
        });

        await test.step('Complete the run from the banner — back on the detail page as PASS', async () => {
            await page.getByTestId('execute-complete-run').click();
            await expect(page).toHaveURL(new RegExp(`/runs/run/${run.id}$`));
            await expect(page.getByTestId('run-status-select')).toHaveValue('PASS');
        });
    });
});
