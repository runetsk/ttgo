import { test, expect } from '@playwright/test';
import { API_URL } from '../../config.js';
import { createFolderAPI, createTestAPI, createRunAPI, addRunResultAPI } from '../../helpers/api.js';

test.describe('Per-step verdicts', () => {
    test('mark steps in execution mode; verdicts persist and derive FAIL', async ({ page, request }) => {
        const stamp = Date.now();
        const testName = `Stepped ${stamp}`;
        let run;

        await test.step('Seed a run whose test has two steps', async () => {
            const folder = await createFolderAPI(request, 'Step Folder ' + stamp);
            const tc = await createTestAPI(request, testName, folder.id, 'API Test', {
                steps: [
                    { action: '<p>Open the login page</p>', expected_result: '<p>Form visible</p>', order_index: 0 },
                    { action: '<p>Submit credentials</p>', expected_result: '<p>Dashboard</p>', order_index: 1 },
                ],
            });
            run = await createRunAPI(request, 'Step Run ' + stamp);
            await addRunResultAPI(request, run.id, tc.id);
        });

        await test.step('Mark step 1 pass and step 2 fail with a note', async () => {
            await page.goto(`/runs/run/${run.id}/execute`);
            await expect(page.getByTestId('execute-current-name')).toHaveText(testName);
            await page.getByTestId('execute-step-pass-0').click();
            await page.getByTestId('execute-step-fail-1').click();
            // Marking a step Fail opens the run-level fail panel.
            await expect(page.getByTestId('execute-defect-type')).toBeVisible();
            await page.getByTestId('execute-step-note-1').fill('Got a 500 page');
        });

        await test.step('Confirm the failure and assert persistence', async () => {
            await page.getByTestId('execute-fail-confirm').click();
            const fresh = await request.get(`${API_URL}/runs/${run.id}`).then(r => r.json());
            const result = fresh.run_results.find(r => r.test_name_snapshot === testName);
            expect(result.status).toBe('FAIL');
            const byIndex = Object.fromEntries((result.steps || []).map(s => [s.order_index, s]));
            expect(byIndex[0].status).toBe('PASS');
            expect(byIndex[1].status).toBe('FAIL');
            expect(byIndex[1].note).toBe('Got a 500 page');
        });

        await test.step('The result detail renders the step checklist', async () => {
            await page.goto(`/runs/run/${run.id}`);
            const row = page.getByRole('row', { name: testName });
            await row.locator('td').nth(1).click({ position: { x: 6, y: 8 } });
            await expect(page.getByTestId('run-result-detail')).toBeVisible();
            const checklist = page.getByTestId('result-step-checklist');
            await expect(checklist).toBeVisible();
            await expect(checklist.getByText('Open the login page')).toBeVisible();
            await expect(checklist.getByText('Got a 500 page')).toBeVisible();
        });
    });
});
