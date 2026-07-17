import { test, expect } from '../../fixtures/test.js';

test.describe('Test Run Retries', () => {

    test('retry button creates a new PENDING attempt and shows badge', async ({ page, runDetailPage, api }) => {
        const ts = Date.now();
        let run;
        let r1;
        let row;

        await test.step('Seed a run with a failed first attempt via API', async () => {
            const folder = await api.createFolder(`Retry Folder ${ts}`);
            const tc = await api.createTest(`Retry Test ${ts}`, folder.id);
            run = await api.createRun(`Retry Run ${ts}`);
            r1 = await api.addRunResult(run.id, tc.id);
            await api.updateRunResult(run.id, r1.id, { status: 'FAIL' });
        });

        await test.step('Open the run detail and confirm the row shows FAIL', async () => {
            await runDetailPage.open(run.id);
            row = runDetailPage.resultRow(`Retry Test ${ts}`);
            await expect(row).toBeVisible();
            await expect(row.locator('select').first()).toHaveValue('FAIL');
        });

        await test.step('Click retry and verify a new PENDING attempt 2 badge appears', async () => {
            page.on('dialog', d => d.dismiss()); // guard against accidental confirms
            await row.getByTitle('Retry this test').click();

            // After retry, the row shows PENDING (new attempt) with an attempt-2 badge.
            await expect(row.locator('select').first()).toHaveValue('PENDING');
            await expect(row.getByTitle(/Attempt 2/)).toBeVisible();
        });
    });

    test('retry creates new attempt and aggregation shows only latest', async ({ runDetailPage, api }) => {
        const ts = Date.now();
        let run;

        await test.step('Seed a run, fail attempt 1, then retry to a passing attempt 2', async () => {
            const folder = await api.createFolder(`Agg Folder ${ts}`);
            const tc = await api.createTest(`Agg Test ${ts}`, folder.id);
            run = await api.createRun(`Agg Run ${ts}`);
            const r1 = await api.addRunResult(run.id, tc.id);
            await api.updateRunResult(run.id, r1.id, { status: 'FAIL' });
            const r2 = await api.retryRunResult(run.id, r1.id);
            await api.updateRunResult(run.id, r2.id, { status: 'PASS' });
        });

        await test.step('Verify the stats bar reflects only the latest attempt', async () => {
            await runDetailPage.open(run.id);
            await expect(runDetailPage.stat('passed')).toContainText('1');
            await expect(runDetailPage.stat('failed')).toContainText('0');
        });
    });

    test('attempt badge click opens detail with timeline', async ({ runDetailPage, api }) => {
        const ts = Date.now();
        let run;
        let badge;

        await test.step('Seed a run with a failed attempt 1 and a passing attempt 2', async () => {
            const folder = await api.createFolder(`Expand Folder ${ts}`);
            const tc = await api.createTest(`Expand Test ${ts}`, folder.id);
            run = await api.createRun(`Expand Run ${ts}`);
            const r1 = await api.addRunResult(run.id, tc.id);
            await api.updateRunResult(run.id, r1.id, { status: 'FAIL' });
            const r2 = await api.retryRunResult(run.id, r1.id);
            await api.updateRunResult(run.id, r2.id, { status: 'PASS' });
        });

        await test.step('Open the run detail and confirm the detail panel is hidden initially', async () => {
            await runDetailPage.open(run.id);
            const row = runDetailPage.resultRow(`Expand Test ${ts}`);
            badge = row.getByTitle(/Attempt 2/);
            await expect(badge).toBeVisible();
            await expect(runDetailPage.resultDetail).not.toBeVisible();
        });

        await test.step('Click the badge to expand the detail panel and verify both attempts on the timeline', async () => {
            await badge.click();
            const detail = runDetailPage.resultDetail;
            await expect(detail).toBeVisible();
            await expect(detail.getByTitle(/Attempt 1/)).toBeVisible();
            await expect(detail.getByTitle(/Attempt 2/)).toBeVisible();
        });

        await test.step('Click the badge again to collapse the detail panel', async () => {
            await badge.click();
            await expect(runDetailPage.resultDetail).not.toBeVisible();
        });
    });

    test('retried_count indicator appears in summary bar', async ({ page, runDetailPage, api }) => {
        const ts = Date.now();
        let run;
        let r1;

        await test.step('Seed a run with a single result via API', async () => {
            const folder = await api.createFolder(`Summary Folder ${ts}`);
            const tc = await api.createTest(`Summary Test ${ts}`, folder.id);
            run = await api.createRun(`Summary Run ${ts}`);
            r1 = await api.addRunResult(run.id, tc.id);
        });

        await test.step('Open the run detail and confirm no retried indicator yet', async () => {
            await runDetailPage.open(run.id);
            await expect(runDetailPage.stat('retried')).not.toBeVisible();
        });

        await test.step('Retry the result via API and pass the new attempt', async () => {
            // Mark attempt 2 PASS so it counts as "passed after retry" (the summary
            // indicator only shows for passed retries).
            const r2 = await api.retryRunResult(run.id, r1.id);
            await api.updateRunResult(run.id, r2.id, { status: 'PASS' });
        });

        await test.step('Reload and verify the summary bar shows the retried count', async () => {
            await page.reload();
            await page.waitForLoadState('domcontentloaded');
            await expect(runDetailPage.stat('retried')).toBeVisible();
            await expect(runDetailPage.stat('retried')).toContainText('1 passed after retry');
        });
    });

    test('run detail shows attempt number in expanded RunResultDetail', async ({ page, runDetailPage, api }) => {
        const ts = Date.now();
        let run;

        await test.step('Seed a run and retry to a failing attempt 2 with an error message', async () => {
            const folder = await api.createFolder(`Detail Folder ${ts}`);
            const tc = await api.createTest(`Detail Test ${ts}`, folder.id);
            run = await api.createRun(`Detail Run ${ts}`);
            const r1 = await api.addRunResult(run.id, tc.id);
            const r2 = await api.retryRunResult(run.id, r1.id);
            await api.updateRunResult(run.id, r2.id, {
                status: 'FAIL',
                error_message: 'Still failing on attempt 2'
            });
        });

        await test.step('Open the run, expand the row, and verify the attempt 2 detail', async () => {
            await runDetailPage.open(run.id);
            // Click the attempt badge (which toggles the detail panel) rather than
            // the row center, since the status/defect cells stop click propagation.
            const row = runDetailPage.resultRow(`Detail Test ${ts}`);
            await row.getByTitle(/Attempt 2/).click();

            await expect(runDetailPage.resultDetail.getByText(/Attempt 2/)).toBeVisible();
            await expect(page.getByText('Still failing on attempt 2')).toBeVisible();
        });
    });

    test('retry API returns 404 for non-existent result', async ({ api }) => {
        await test.step('Retry a non-existent result and expect a 404', async () => {
            const run = await api.createRun(`404 Retry Run ${Date.now()}`);
            const res = await api.post(`/runs/${run.id}/results/non-existent-id/retry`);
            expect(res.status()).toBe(404);
        });
    });

    test('retry API returns 400 for orphaned result', async ({ api }) => {
        let run;
        let orphanRes;

        await test.step('Create a run and add an orphaned result (no test_case_id)', async () => {
            run = await api.createRun(`Orphan Retry Run ${Date.now()}`);
            orphanRes = await api.post(`/runs/${run.id}/results`, { test_case_id: null, test_name_snapshot: 'Orphan Test' });
        });

        await test.step('Retry the orphaned result and expect a 400 if it was created', async () => {
            // Orphaned results may be rejected at add time; if created, retry should 400.
            if (orphanRes.ok()) {
                const orphan = await orphanRes.json();
                const retryRes = await api.post(`/runs/${run.id}/results/${orphan.id}/retry`);
                expect(retryRes.status()).toBe(400);
            }
        });
    });
});
