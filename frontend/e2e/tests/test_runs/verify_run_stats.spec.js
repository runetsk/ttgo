import { test, expect } from '../../fixtures/test.js';
import { ApiClient } from '../../helpers/api.js';

test.describe('Test Run Stats Verification', () => {
    let runId;
    let categoryId;

    // beforeAll only has the worker-scoped `request` fixture, so build the client
    // from it directly (mirrors the AI-generation pilot).
    test.beforeAll(async ({ request }) => {
        const api = new ApiClient(request);
        const timestamp = Date.now();

        const category = await api.createCategory(`Stats Category ${timestamp}`, 'E2E stats test');
        categoryId = category.id;
        const folder = await api.createFolder(`Stats Folder ${timestamp}`);
        const run = await api.createRun(`Stats Verify Run ${timestamp}`, { categoryId });
        runId = run.id;

        // 3 tests as run results, then set statuses: 1 PASS, 1 FAIL, 1 left PENDING.
        const resultIds = [];
        for (let i = 0; i < 3; i++) {
            const t = await api.createTest(`Stats Test ${i} ${timestamp}`, folder.id, 'Temp test');
            const result = await api.addRunResult(runId, t.id);
            resultIds.push(result.id);
        }
        await api.updateRunResult(runId, resultIds[0], { status: 'PASS' });
        await api.updateRunResult(runId, resultIds[1], { status: 'FAIL' });
    });

    test('should display correct stats in TestRunList columns', async ({ runsPage }) => {
        await test.step('Open the runs page and filter by the seeded category', async () => {
            await runsPage.open();
            await runsPage.openColumnFilters();
            await runsPage.filterByCategory(categoryId);
        });

        await test.step('Verify the run row is visible and the stats columns are correct', async () => {
            await expect(runsPage.selectRunCheckbox(runId)).toBeVisible();
            // Passed/Failed are default-visible columns; Pending/Total are optional
            // columns hidden by default, so assert the visible stats here.
            await expect(runsPage.runPassed(runId)).toHaveText('1');
            await expect(runsPage.runFailed(runId)).toHaveText('1');
        });
    });

    test('should display correct stats in TestRunDetail header', async ({ runDetailPage }) => {
        await test.step('Open the run detail page', async () => {
            await runDetailPage.open(runId);
        });

        await test.step('Verify the stats bar shows correct passed, failed, and pending counts', async () => {
            // Redesigned stats bar shows passed as "{passed} / {total}" — no separate total testid.
            await expect(runDetailPage.stat('passed')).toContainText('1');
            await expect(runDetailPage.stat('passed')).toContainText('3');
            await expect(runDetailPage.stat('failed')).toContainText('1');
            await expect(runDetailPage.stat('pending')).toContainText('1');
        });
    });
});
