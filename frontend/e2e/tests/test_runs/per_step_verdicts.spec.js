import { test, expect } from '../../fixtures/test.js';

test.describe('Per-step verdicts', () => {
    test('Exit waits for the latest step note to save', async ({ page, runExecutePage, api }) => {
        const folder = await api.createFolder('Exit steps ' + Date.now());
        const tc = await api.createTest('Exit step', folder.id, '', {
            steps: [{ action: 'Check', expected_result: 'OK', order_index: 0 }],
        });
        const run = await api.createRun('Exit run');
        await api.addRunResult(run.id, tc.id);
        await runExecutePage.open(run.id);
        await runExecutePage.stepPass(0);
        await runExecutePage.stepNote(0).fill('Saved before exit');
        await page.getByTestId('execute-exit').click();
        await expect(page).toHaveURL(new RegExp(`/runs/run/${run.id}$`));
        const fresh = await api.getRun(run.id);
        expect(fresh.run_results[0].steps[0].note).toBe('Saved before exit');
    });

    test('step drafts survive switching cases before autosave and later verdicts', async ({ runExecutePage, api }) => {
        const stamp = Date.now();
        const folder = await api.createFolder('Navigation steps ' + stamp);
        const steps = [
            { action: 'First step', expected_result: 'First result', order_index: 0 },
            { action: 'Second step', expected_result: 'Second result', order_index: 1 },
        ];
        const a = await api.createTest('A navigation ' + stamp, folder.id, '', { steps });
        const b = await api.createTest('B navigation ' + stamp, folder.id, '', { steps });
        const run = await api.createRun('Navigation run ' + stamp);
        await api.addRunResult(run.id, a.id);
        await api.addRunResult(run.id, b.id);
        await runExecutePage.open(run.id);
        await runExecutePage.stepPass(0);
        await runExecutePage.stepNote(0).fill('Keep this note');
        await runExecutePage.next();
        await expect(runExecutePage.currentName).toHaveText(b.name);
        await runExecutePage.stepPass(0);
        await runExecutePage.prev();
        await expect(runExecutePage.currentName).toHaveText(a.name);
        await expect(runExecutePage.stepNote(0)).toHaveValue('Keep this note');
        await runExecutePage.stepPass(1);
        await runExecutePage.pass();
        await expect(runExecutePage.currentName).toHaveText(b.name);
        const fresh = await api.getRun(run.id);
        const result = fresh.run_results.find(r => r.test_case_id === a.id);
        expect(result.steps.map(s => s.status)).toEqual(['PASS', 'PASS']);
        expect(result.steps[0].note).toBe('Keep this note');
    });

    test('mark steps in execution mode; verdicts persist and derive FAIL', async ({ runDetailPage, runExecutePage, api }) => {
        const stamp = Date.now();
        const testName = `Stepped ${stamp}`;
        let run;

        await test.step('Seed a run whose test has two steps', async () => {
            const folder = await api.createFolder('Step Folder ' + stamp);
            const tc = await api.createTest(testName, folder.id, 'API Test', {
                steps: [
                    { action: '<p>Open the login page</p>', expected_result: '<p>Form visible</p>', order_index: 0 },
                    { action: '<p>Submit credentials</p>', expected_result: '<p>Dashboard</p>', order_index: 1 },
                ],
            });
            run = await api.createRun('Step Run ' + stamp);
            await api.addRunResult(run.id, tc.id);
        });

        await test.step('Mark step 1 pass and step 2 fail with a note', async () => {
            await runExecutePage.open(run.id);
            await expect(runExecutePage.currentName).toHaveText(testName);
            await runExecutePage.stepPass(0);
            await runExecutePage.stepFail(1);
            // Marking a step Fail opens the run-level fail panel.
            await expect(runExecutePage.defectTypeSelect).toBeVisible();
            await runExecutePage.stepNote(1).fill('Got a 500 page');
        });

        await test.step('Confirm the failure and assert persistence', async () => {
            await runExecutePage.confirmFail();
            // The queue counts the verdict only after the server answers the save,
            // so reading the run any earlier races the write.
            await expect(runExecutePage.progress).toHaveText('1 / 1 executed');
            const fresh = await api.getRun(run.id);
            const result = fresh.run_results.find(r => r.test_name_snapshot === testName);
            expect(result.status).toBe('FAIL');
            const byIndex = Object.fromEntries((result.steps || []).map(s => [s.order_index, s]));
            expect(byIndex[0].status).toBe('PASS');
            expect(byIndex[1].status).toBe('FAIL');
            expect(byIndex[1].note).toBe('Got a 500 page');
        });

        await test.step('The result detail renders the step checklist', async () => {
            await runDetailPage.open(run.id);
            await runDetailPage.expandResultRow(testName);
            await expect(runDetailPage.resultDetail).toBeVisible();
            const checklist = runDetailPage.stepChecklist;
            await expect(checklist).toBeVisible();
            await expect(checklist.getByText('Open the login page')).toBeVisible();
            await expect(checklist.getByText('Got a 500 page')).toBeVisible();
        });
    });
});
