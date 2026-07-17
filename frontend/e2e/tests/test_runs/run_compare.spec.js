import { test, expect } from '../../fixtures/test.js';
import { TIMEOUTS } from '../../config.js';

test.describe('Run Comparison (Compare tab)', () => {
    const applyStatuses = async (api, runId, statusByTc) => {
        const run = await api.getRun(runId);
        for (const rr of run.run_results) {
            const st = statusByTc[rr.test_case_id];
            if (!st) continue;
            await api.updateRunResult(runId, rr.id, { status: st });
        }
    };

    // Seeds two runs covering every bucket. Returns ids needed by the tests.
    const seed = async (api) => {
        const stamp = Date.now();
        const folder = await api.createFolder(`Cmp Folder ${stamp}`);
        const shared = [];
        for (const n of ['s1', 's2', 's3', 's4', 's5']) shared.push(await api.createTest(`Cmp ${n} ${stamp}`, folder.id, 'compare e2e'));
        const onlyA = await api.createTest(`Cmp onlyA ${stamp}`, folder.id, 'compare e2e');
        const onlyB = await api.createTest(`Cmp onlyB ${stamp}`, folder.id, 'compare e2e');
        const runA = await api.createRun(`Cmp A ${stamp}`);
        const runB = await api.createRun(`Cmp B ${stamp}`);
        // Add shared tests to both runs.
        for (const t of shared) {
            await api.addRunResult(runA.id, t.id);
            await api.addRunResult(runB.id, t.id);
        }
        // Add exclusive tests to their respective runs only.
        await api.addRunResult(runA.id, onlyA.id);
        await api.addRunResult(runB.id, onlyB.id);
        // s1 regression (runA=FAIL, runB=PASS), s2 fixed (runA=PASS, runB=FAIL),
        // s3 still-failing (both FAIL), s4 unchanged (both PASS), s5 other-change (runA=PASS, runB=SKIP)
        await applyStatuses(api, runA.id, {
            [shared[0].id]: 'FAIL', [shared[1].id]: 'PASS', [shared[2].id]: 'FAIL',
            [shared[3].id]: 'PASS', [shared[4].id]: 'PASS', [onlyA.id]: 'PASS',
        });
        await applyStatuses(api, runB.id, {
            [shared[0].id]: 'PASS', [shared[1].id]: 'FAIL', [shared[2].id]: 'FAIL',
            [shared[3].id]: 'PASS', [shared[4].id]: 'SKIP', [onlyB.id]: 'SKIP',
        });
        return { runA, runB, shared };
    };

    test('groups tests by outcome, shows summary, and expands a regression', async ({ page, runDetailPage, api }) => {
        let runA;
        let runB;
        let shared;

        await test.step('Seed two runs covering every comparison bucket', async () => {
            ({ runA, runB, shared } = await seed(api));
        });

        await test.step('Open the compare view for run A against run B', async () => {
            await runDetailPage.openCompare(runA.id, runB.id);
            await expect(runDetailPage.compareTab).toBeVisible({ timeout: TIMEOUTS.HEAVY_GRID });
        });

        await test.step('Verify every bucket has exactly one test', async () => {
            for (const key of ['regressions', 'fixed', 'stillFailing', 'otherChanges', 'unchanged', 'onlyThis', 'onlyCompared']) {
                await expect(runDetailPage.compareGroupCount(key)).toHaveText('1', { timeout: TIMEOUTS.APP_RENDER });
            }
        });

        await test.step('Verify the summary chips', async () => {
            await expect(runDetailPage.compareCount('regressions')).toHaveText(/1/, { timeout: TIMEOUTS.APP_RENDER });
            await expect(runDetailPage.compareCount('shared')).toHaveText(/5/, { timeout: TIMEOUTS.APP_RENDER });
        });

        await test.step('Expand the regression row and verify both runs\' statuses', async () => {
            // The regression row is s1; expanding it shows both runs' statuses.
            const regRow = runDetailPage.compareRow(shared[0].id);
            await expect(regRow).toBeVisible();
            await regRow.click();
            const detail = runDetailPage.compareDetail(shared[0].id);
            await expect(detail).toBeVisible();
            await expect(detail.getByText('Fail', { exact: true })).toBeVisible();
            await expect(detail.getByText('Pass', { exact: true })).toBeVisible();
        });

        await test.step('Verify the deep-link round-trips across reload', async () => {
            await page.reload();
            await page.waitForLoadState('domcontentloaded');
            await expect(runDetailPage.compareGroupCount('regressions')).toHaveText('1', { timeout: TIMEOUTS.HEAVY_GRID });
        });
    });

    test('guards against comparing a run with itself', async ({ runDetailPage, api }) => {
        let runA;

        await test.step('Seed two runs covering every comparison bucket', async () => {
            ({ runA } = await seed(api));
        });

        await test.step('Open the compare view for run A against itself and verify the guard', async () => {
            await runDetailPage.openCompare(runA.id, runA.id);
            await expect(runDetailPage.compareSameRun).toBeVisible({ timeout: TIMEOUTS.HEAVY_GRID });
        });
    });
});
