import { test, expect } from '../../fixtures/test.js';

test.describe('Per-step verdicts', () => {
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
