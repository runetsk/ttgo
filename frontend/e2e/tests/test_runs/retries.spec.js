import { test, expect } from '@playwright/test';
import { API_URL } from '../../config.js';
import {
    createFolderAPI,
    createTestAPI,
    createRunAPI,
    addRunResultAPI,
    updateRunResultAPI,
    retryRunResultAPI,
} from '../../helpers/api.js';

test.describe('Test Run Retries', () => {

    // ── Tests ─────────────────────────────────────────────────────────────────

    test('retry button creates a new PENDING attempt and shows badge', async ({ page, request }) => {
        const ts = Date.now();
        let run;
        let r1;
        let row;

        await test.step('Seed a run with a failed first attempt via API', async () => {
            const folder = await createFolderAPI(request, `Retry Folder ${ts}`);
            const tc = await createTestAPI(request, `Retry Test ${ts}`, folder.id);
            run = await createRunAPI(request, `Retry Run ${ts}`);
            r1 = await addRunResultAPI(request, run.id, tc.id);

            // Mark first attempt as FAIL
            await updateRunResultAPI(request, run.id, r1.id, { status: 'FAIL' });
        });

        await test.step('Open the run detail and confirm the row shows FAIL', async () => {
            await page.goto(`/runs/run/${run.id}`);
            await page.waitForLoadState('domcontentloaded');

            row = page.getByRole('row', { name: `Retry Test ${ts}` });
            await expect(row).toBeVisible();

            // Status shows FAIL
            await expect(row.locator('select').first()).toHaveValue('FAIL');
        });

        await test.step('Click retry and verify a new PENDING attempt 2 badge appears', async () => {
            // Click the retry (↻) button
            page.on('dialog', d => d.dismiss()); // guard against accidental confirms
            await row.getByTitle('Retry this test').click();

            // After retry, the row should now show PENDING (new attempt)
            await expect(row.locator('select').first()).toHaveValue('PENDING');

            // Attempt 2 badge should appear
            await expect(row.getByTitle(/Attempt 2/)).toBeVisible();
        });
    });

    test('retry creates new attempt and aggregation shows only latest', async ({ page, request }) => {
        const ts = Date.now();
        let run;

        await test.step('Seed a run, fail attempt 1, then retry to a passing attempt 2', async () => {
            const folder = await createFolderAPI(request, `Agg Folder ${ts}`);
            const tc = await createTestAPI(request, `Agg Test ${ts}`, folder.id);
            run = await createRunAPI(request, `Agg Run ${ts}`);
            const r1 = await addRunResultAPI(request, run.id, tc.id);

            // Attempt 1: FAIL
            await updateRunResultAPI(request, run.id, r1.id, { status: 'FAIL' });

            // Retry via API → attempt 2 PASS
            const r2 = await retryRunResultAPI(request, run.id, r1.id);
            await updateRunResultAPI(request, run.id, r2.id, { status: 'PASS' });
        });

        await test.step('Verify the stats bar reflects only the latest attempt', async () => {
            await page.goto(`/runs/run/${run.id}`);
            await page.waitForLoadState('domcontentloaded');

            // Stats bar should reflect latest attempt only: 1 passed, 0 failed
            await expect(page.getByTestId('stats-passed')).toContainText('1');
            await expect(page.getByTestId('stats-failed')).toContainText('0');
        });
    });

    test('attempt badge click opens detail with timeline', async ({ page, request }) => {
        const ts = Date.now();
        let run;
        let badge;

        await test.step('Seed a run with a failed attempt 1 and a passing attempt 2', async () => {
            const folder = await createFolderAPI(request, `Expand Folder ${ts}`);
            const tc = await createTestAPI(request, `Expand Test ${ts}`, folder.id);
            run = await createRunAPI(request, `Expand Run ${ts}`);
            const r1 = await addRunResultAPI(request, run.id, tc.id);

            await updateRunResultAPI(request, run.id, r1.id, { status: 'FAIL' });
            const r2 = await retryRunResultAPI(request, run.id, r1.id);
            await updateRunResultAPI(request, run.id, r2.id, { status: 'PASS' });
        });

        await test.step('Open the run detail and confirm the detail panel is hidden initially', async () => {
            await page.goto(`/runs/run/${run.id}`);
            await page.waitForLoadState('domcontentloaded');

            const row = page.getByRole('row', { name: `Expand Test ${ts}` });
            badge = row.getByTitle(/Attempt 2/);
            await expect(badge).toBeVisible();

            // Detail panel not visible before clicking badge
            await expect(page.getByTestId('run-result-detail')).not.toBeVisible();
        });

        await test.step('Click the badge to expand the detail panel and verify both attempts on the timeline', async () => {
            // Click badge to expand detail panel with timeline
            await badge.click();

            // Detail panel should appear with timeline dots for both attempts
            const detail = page.getByTestId('run-result-detail');
            await expect(detail).toBeVisible();
            await expect(detail.getByTitle(/Attempt 1/)).toBeVisible();
            await expect(detail.getByTitle(/Attempt 2/)).toBeVisible();
        });

        await test.step('Click the badge again to collapse the detail panel', async () => {
            // Click badge again to collapse
            await badge.click();
            await expect(page.getByTestId('run-result-detail')).not.toBeVisible();
        });
    });

    test('retried_count indicator appears in summary bar', async ({ page, request }) => {
        const ts = Date.now();
        let run;
        let r1;

        await test.step('Seed a run with a single result via API', async () => {
            const folder = await createFolderAPI(request, `Summary Folder ${ts}`);
            const tc = await createTestAPI(request, `Summary Test ${ts}`, folder.id);
            run = await createRunAPI(request, `Summary Run ${ts}`);
            r1 = await addRunResultAPI(request, run.id, tc.id);
        });

        await test.step('Open the run detail and confirm no retried indicator yet', async () => {
            // Before retry — no retried indicator
            await page.goto(`/runs/run/${run.id}`);
            await page.waitForLoadState('domcontentloaded');
            await expect(page.getByTestId('stats-retried')).not.toBeVisible();
        });

        await test.step('Retry the result via API and pass the new attempt', async () => {
            // Retry via API → attempt 2, then mark it PASS so it counts as
            // "passed after retry" (the summary indicator only shows for passed retries).
            const r2 = await retryRunResultAPI(request, run.id, r1.id);
            await updateRunResultAPI(request, run.id, r2.id, { status: 'PASS' });
        });

        await test.step('Reload and verify the summary bar shows the retried count', async () => {
            await page.reload();
            await page.waitForLoadState('domcontentloaded');

            // Summary bar should now show "↻ 1 passed after retry"
            await expect(page.getByTestId('stats-retried')).toBeVisible();
            await expect(page.getByTestId('stats-retried')).toContainText('1 passed after retry');
        });
    });

    test('run detail shows attempt number in expanded RunResultDetail', async ({ page, request }) => {
        const ts = Date.now();
        let run;

        await test.step('Seed a run and retry to a failing attempt 2 with an error message', async () => {
            const folder = await createFolderAPI(request, `Detail Folder ${ts}`);
            const tc = await createTestAPI(request, `Detail Test ${ts}`, folder.id);
            run = await createRunAPI(request, `Detail Run ${ts}`);
            const r1 = await addRunResultAPI(request, run.id, tc.id);

            const r2 = await retryRunResultAPI(request, run.id, r1.id);
            await updateRunResultAPI(request, run.id, r2.id, {
                status: 'FAIL',
                error_message: 'Still failing on attempt 2'
            });
        });

        await test.step('Open the run, expand the row, and verify the attempt 2 detail', async () => {
            await page.goto(`/runs/run/${run.id}`);
            await page.waitForLoadState('domcontentloaded');

            // Expand the result row to show RunResultDetail. Click the attempt badge
            // (which toggles the detail panel) rather than the row center, since the
            // status/defect cells stop click propagation and would not expand the row.
            const row = page.getByRole('row', { name: `Detail Test ${ts}` });
            await row.getByTitle(/Attempt 2/).click();

            // RunResultDetail should show "Attempt 2" header
            await expect(page.getByTestId('run-result-detail').getByText(/Attempt 2/)).toBeVisible();
            await expect(page.getByText('Still failing on attempt 2')).toBeVisible();
        });
    });

    test('retry API returns 404 for non-existent result', async ({ request }) => {
        await test.step('Retry a non-existent result and expect a 404', async () => {
            const run = await createRunAPI(request, `404 Retry Run ${Date.now()}`);
            const res = await request.post(`${API_URL}/runs/${run.id}/results/non-existent-id/retry`);
            expect(res.status()).toBe(404);
        });
    });

    test('retry API returns 400 for orphaned result', async ({ request }) => {
        let run;
        let orphanRes;

        await test.step('Create a run and add an orphaned result (no test_case_id)', async () => {
            run = await createRunAPI(request, `Orphan Retry Run ${Date.now()}`);

            // Add a result without a test_case_id (orphaned)
            orphanRes = await request.post(`${API_URL}/runs/${run.id}/results`, {
                data: { test_case_id: null, test_name_snapshot: 'Orphan Test' }
            });
        });

        await test.step('Retry the orphaned result and expect a 400 if it was created', async () => {
            // Orphaned results may be rejected at add time; if created, retry should 400
            if (orphanRes.ok()) {
                const orphan = await orphanRes.json();
                const retryRes = await request.post(`${API_URL}/runs/${run.id}/results/${orphan.id}/retry`);
                expect(retryRes.status()).toBe(400);
            }
        });
    });
});
