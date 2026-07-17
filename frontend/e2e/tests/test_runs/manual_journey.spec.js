import { test, expect } from '../../fixtures/test.js';

test.describe('Manual run journey', () => {
    test('creating a run lands on its detail page', async ({ page, runsPage, runDetailPage }) => {
        const runName = 'Journey Run ' + Date.now();

        await test.step('Create an empty run via the UI modal', async () => {
            await runsPage.open();
            await runsPage.createRun({ name: runName });
        });

        await test.step('The app navigates into the new run', async () => {
            await expect(page).toHaveURL(/\/runs\/run\/[a-f0-9-]+$/);
            await expect(runDetailPage.runTitle).toHaveText(runName);
        });
    });

    test('complete and reopen a run from the header', async ({ runDetailPage, api }) => {
        const runName = 'Completable Run ' + Date.now();
        let run;

        await test.step('Seed a run with one passing result via API', async () => {
            const folder = await api.createFolder('Complete Folder ' + Date.now());
            const tc = await api.createTest('Complete case', folder.id);
            run = await api.createRun(runName);
            await api.addRunResult(run.id, tc.id, { status: 'PASS' });
        });

        await test.step('Complete the run from the header', async () => {
            await runDetailPage.open(run.id);
            await runDetailPage.completeRunButton.click();
            await expect(runDetailPage.runStatusSelect).toHaveValue('PASS');
            await expect(runDetailPage.completeRunButton).not.toBeVisible();
        });

        await test.step('Reopen the run from the header', async () => {
            await runDetailPage.reopenRunButton.click();
            await expect(runDetailPage.runStatusSelect).toHaveValue('RUNNING');
            await expect(runDetailPage.reopenRunButton).not.toBeVisible();
        });
    });

    test('execution mode walks through the run queue', async ({ page, runDetailPage, runExecutePage, api }) => {
        const runName = 'Exec Run ' + Date.now();
        let run;

        await test.step('Seed a run with three cases via API', async () => {
            const folder = await api.createFolder('Exec Folder ' + Date.now());
            run = await api.createRun(runName);
            for (const name of ['Alpha check', 'Beta check', 'Gamma check']) {
                const tc = await api.createTest(name, folder.id);
                await api.addRunResult(run.id, tc.id);
            }
        });

        await test.step('Enter execution mode from the run header', async () => {
            await runDetailPage.open(run.id);
            // Execute is scoped to checked rows; select-all runs the whole queue.
            await runDetailPage.selectAllResults();
            await runDetailPage.executeRunButton.click();
            await expect(page).toHaveURL(new RegExp(`/runs/run/${run.id}/execute`));
        });

        await test.step('First pending test is shown with progress', async () => {
            await expect(runExecutePage.currentName).toHaveText('Alpha check');
            await expect(runExecutePage.progress).toContainText('0 / 3');
        });

        await test.step('Next / Prev navigate the queue', async () => {
            await runExecutePage.next();
            await expect(runExecutePage.currentName).toHaveText('Beta check');
            await runExecutePage.prev();
            await expect(runExecutePage.currentName).toHaveText('Alpha check');
        });

        await test.step('Sidebar jump selects a specific test', async () => {
            await page.getByText('Gamma check', { exact: true }).click();
            await expect(runExecutePage.currentName).toHaveText('Gamma check');
        });
    });

    test('execution mode shows the authored steps of the current test', async ({ runExecutePage, api }) => {
        const runName = 'Steps Run ' + Date.now();
        let run;

        await test.step('Seed a run whose test has two authored steps', async () => {
            const folder = await api.createFolder('Steps Folder ' + Date.now());
            const tc = await api.createTest('Stepped case', folder.id, 'API Test', {
                steps: [
                    { action: '<p>Open the login page</p>', expected_result: '<p>Form is visible</p>', order_index: 0 },
                    { action: '<p>Submit valid credentials</p>', expected_result: '<p>Dashboard loads</p>', order_index: 1 },
                ],
            });
            run = await api.createRun(runName);
            await api.addRunResult(run.id, tc.id);
        });

        await test.step('Steps render on the execute page', async () => {
            await runExecutePage.open(run.id);
            await expect(runExecutePage.currentName).toHaveText('Stepped case');
            const steps = runExecutePage.steps;
            await expect(steps.getByText('Open the login page')).toBeVisible();
            await expect(steps.getByText('Form is visible')).toBeVisible();
            await expect(steps.getByText('Submit valid credentials')).toBeVisible();
        });
    });

    test('verdicts advance the queue and persist details', async ({ runExecutePage, api }) => {
        const runName = 'Verdict Run ' + Date.now();
        let run;

        await test.step('Seed a run with three cases via API', async () => {
            const folder = await api.createFolder('Verdict Folder ' + Date.now());
            run = await api.createRun(runName);
            for (const name of ['V-Alpha', 'V-Beta', 'V-Gamma']) {
                const tc = await api.createTest(name, folder.id);
                await api.addRunResult(run.id, tc.id);
            }
        });

        await test.step('Pass the first test — auto-advances to the second', async () => {
            await runExecutePage.open(run.id);
            await expect(runExecutePage.currentName).toHaveText('V-Alpha');
            await runExecutePage.pass();
            await expect(runExecutePage.currentName).toHaveText('V-Beta');
            await expect(runExecutePage.progress).toContainText('1 / 3');
        });

        await test.step('Fail the second test with a note and defect type', async () => {
            await runExecutePage.fail({ defectType: 'product_bug', note: 'Button did nothing' });
            await expect(runExecutePage.currentName).toHaveText('V-Gamma');
        });

        await test.step('Skip the last test — completion banner appears', async () => {
            await runExecutePage.skip();
            await expect(runExecutePage.doneBanner).toBeVisible();
            await expect(runExecutePage.progress).toContainText('3 / 3');
        });

        await test.step('Verdicts persisted to the API', async () => {
            const fresh = await api.getRun(run.id);
            const byName = Object.fromEntries(fresh.run_results.map(r => [r.test_name_snapshot, r]));
            if (byName['V-Alpha'].status !== 'PASS') throw new Error('V-Alpha not PASS');
            if (byName['V-Beta'].status !== 'FAIL') throw new Error('V-Beta not FAIL');
            if (byName['V-Beta'].defect_type !== 'product_bug') throw new Error('defect_type not saved');
            if (byName['V-Beta'].error_message !== 'Button did nothing') throw new Error('error_message not saved');
            if (byName['V-Gamma'].status !== 'SKIP') throw new Error('V-Gamma not SKIP');
            if (fresh.status !== 'RUNNING') throw new Error('run did not auto-start');
        });
    });

    test('keyboard-driven execution and run completion', async ({ page, runDetailPage, runExecutePage, api }) => {
        const runName = 'Keyboard Run ' + Date.now();
        let run;

        await test.step('Seed a run with two cases via API', async () => {
            const folder = await api.createFolder('Kbd Folder ' + Date.now());
            run = await api.createRun(runName);
            for (const name of ['K-One', 'K-Two']) {
                const tc = await api.createTest(name, folder.id);
                await api.addRunResult(run.id, tc.id);
            }
        });

        await test.step('Pass both tests with the P key', async () => {
            await runExecutePage.open(run.id);
            await expect(runExecutePage.currentName).toHaveText('K-One');
            await page.keyboard.press('p');
            await expect(runExecutePage.currentName).toHaveText('K-Two');
            await page.keyboard.press('p');
            await expect(runExecutePage.doneBanner).toBeVisible();
        });

        await test.step('Verdict keys are disarmed once the run is fully executed', async () => {
            await page.keyboard.press('s');
            await page.keyboard.press('f');
            await expect(runExecutePage.failNote).not.toBeVisible();
            const fresh = await api.getRun(run.id);
            const byName = Object.fromEntries(fresh.run_results.map(r => [r.test_name_snapshot, r]));
            if (byName['K-One'].status !== 'PASS') throw new Error('K-One was mutated by a stray key');
            if (byName['K-Two'].status !== 'PASS') throw new Error('K-Two was mutated by a stray key');
        });

        await test.step('Complete the run from the banner — back on the detail page as PASS', async () => {
            await runExecutePage.completeRun();
            await expect(page).toHaveURL(new RegExp(`/runs/run/${run.id}$`));
            await expect(runDetailPage.runStatusSelect).toHaveValue('PASS');
        });
    });
});
