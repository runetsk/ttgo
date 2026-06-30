import { test, expect } from '@playwright/test';
import { API_URL } from '../../config.js';
import {
    createFolderAPI,
    createTestAPI,
    createRunAPI,
    addRunResultAPI,
    getRunAPI,
} from '../../helpers/api.js';

test.describe('Run Comparison (Compare tab)', () => {
    const applyStatuses = async (request, runId, statusByTc) => {
        const run = await getRunAPI(request, runId);
        for (const rr of run.run_results) {
            const st = statusByTc[rr.test_case_id];
            if (!st) continue;
            const res = await request.put(`${API_URL}/runs/${runId}/results/${rr.id}`, { data: { status: st } });
            expect(res.ok()).toBeTruthy();
        }
    };

    // Seeds two runs covering every bucket. Returns ids needed by the tests.
    const seed = async (request) => {
        const stamp = Date.now();
        const folder = await createFolderAPI(request, `Cmp Folder ${stamp}`);
        const shared = [];
        for (const n of ['s1', 's2', 's3', 's4', 's5']) shared.push(await createTestAPI(request, `Cmp ${n} ${stamp}`, folder.id, 'compare e2e'));
        const onlyA = await createTestAPI(request, `Cmp onlyA ${stamp}`, folder.id, 'compare e2e');
        const onlyB = await createTestAPI(request, `Cmp onlyB ${stamp}`, folder.id, 'compare e2e');
        const runA = await createRunAPI(request, `Cmp A ${stamp}`);
        const runB = await createRunAPI(request, `Cmp B ${stamp}`);
        // Add shared tests to both runs
        for (const t of shared) {
            await addRunResultAPI(request, runA.id, t.id);
            await addRunResultAPI(request, runB.id, t.id);
        }
        // Add exclusive tests to their respective runs only
        await addRunResultAPI(request, runA.id, onlyA.id);
        await addRunResultAPI(request, runB.id, onlyB.id);
        // s1 regression (runA=FAIL, runB=PASS), s2 fixed (runA=PASS, runB=FAIL),
        // s3 still-failing (both FAIL), s4 unchanged (both PASS), s5 other-change (runA=PASS, runB=SKIP)
        await applyStatuses(request, runA.id, {
            [shared[0].id]: 'FAIL', [shared[1].id]: 'PASS', [shared[2].id]: 'FAIL',
            [shared[3].id]: 'PASS', [shared[4].id]: 'PASS', [onlyA.id]: 'PASS',
        });
        await applyStatuses(request, runB.id, {
            [shared[0].id]: 'PASS', [shared[1].id]: 'FAIL', [shared[2].id]: 'FAIL',
            [shared[3].id]: 'PASS', [shared[4].id]: 'SKIP', [onlyB.id]: 'SKIP',
        });
        return { runA, runB, shared };
    };

    test('groups tests by outcome, shows summary, and expands a regression', async ({ page, request }) => {
        let runA;
        let runB;
        let shared;

        await test.step('Seed two runs covering every comparison bucket', async () => {
            ({ runA, runB, shared } = await seed(request));
        });

        await test.step('Open the compare view for run A against run B', async () => {
            await page.goto(`/runs/run/${runA.id}?compareWith=${runB.id}`);
            await page.waitForLoadState('domcontentloaded');

            await expect(page.getByTestId('run-compare-tab')).toBeVisible({ timeout: 30000 });
        });

        await test.step('Verify every bucket has exactly one test', async () => {
            // Every bucket has exactly one test.
            for (const key of ['regressions', 'fixed', 'stillFailing', 'otherChanges', 'unchanged', 'onlyThis', 'onlyCompared']) {
                await expect(page.getByTestId(`compare-group-${key}-count`)).toHaveText('1', { timeout: 15000 });
            }
        });

        await test.step('Verify the summary chips', async () => {
            // Summary chips.
            await expect(page.getByTestId('compare-count-regressions')).toHaveText(/1/, { timeout: 15000 });
            await expect(page.getByTestId('compare-count-shared')).toHaveText(/5/, { timeout: 15000 });
        });

        await test.step('Expand the regression row and verify both runs\' statuses', async () => {
            // The regression row is s1; expanding it shows both runs' statuses.
            const regRow = page.getByTestId(`compare-row-${shared[0].id}`);
            await expect(regRow).toBeVisible();
            await regRow.click();
            const detail = page.getByTestId(`compare-detail-${shared[0].id}`);
            await expect(detail).toBeVisible();
            await expect(detail.getByText('Fail', { exact: true })).toBeVisible();
            await expect(detail.getByText('Pass', { exact: true })).toBeVisible();
        });

        await test.step('Verify the deep-link round-trips across reload', async () => {
            // Deep-link round-trips across reload.
            await page.reload();
            await page.waitForLoadState('domcontentloaded');
            await expect(page.getByTestId('compare-group-regressions-count')).toHaveText('1', { timeout: 30000 });
        });
    });

    test('guards against comparing a run with itself', async ({ page, request }) => {
        let runA;

        await test.step('Seed two runs covering every comparison bucket', async () => {
            ({ runA } = await seed(request));
        });

        await test.step('Open the compare view for run A against itself and verify the guard', async () => {
            await page.goto(`/runs/run/${runA.id}?compareWith=${runA.id}`);
            await page.waitForLoadState('domcontentloaded');
            await expect(page.getByTestId('compare-same-run')).toBeVisible({ timeout: 30000 });
        });
    });
});
