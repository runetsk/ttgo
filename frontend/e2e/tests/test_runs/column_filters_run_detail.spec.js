import { test, expect } from '../../fixtures/test.js';
import { TIMEOUTS } from '../../config.js';

test.describe('Run detail results filters', () => {
    test('filter row toggles and status filter narrows results', async ({ runDetailPage, api }) => {
        let run;

        await test.step('Seed a folder, two tests, a run, and two PENDING results via API', async () => {
            const stamp = Date.now();
            const folder = await api.createFolder(`CF Folder ${stamp}`);
            const t1 = await api.createTest(`CF Test A ${stamp}`, folder.id);
            const t2 = await api.createTest(`CF Test B ${stamp}`, folder.id);
            run = await api.createRun(`CF Run ${stamp}`);
            await api.addRunResult(run.id, t1.id, { status: 'PENDING' });
            await api.addRunResult(run.id, t2.id, { status: 'PENDING' });
        });

        await test.step('Open the run detail page and verify the toolbar is visible', async () => {
            await runDetailPage.open(run.id);
            await expect(runDetailPage.toolbar).toBeVisible({ timeout: TIMEOUTS.HEAVY_GRID });
        });

        await test.step('Toggle Column Filters on and verify the status filter appears', async () => {
            await expect(runDetailPage.resultStatusFilter).not.toBeVisible();
            await runDetailPage.openColumnFilters();
            await expect(runDetailPage.resultStatusFilter).toBeVisible();
        });

        await test.step('Verify two rows are visible before filtering', async () => {
            await expect(runDetailPage.resultRows).toHaveCount(2);
        });

        await test.step('Filter to PASS and verify no rows remain', async () => {
            await runDetailPage.resultStatusFilter.selectOption('PASS');
            await expect(runDetailPage.resultRows).toHaveCount(0);
        });

        await test.step('Filter back to PENDING and verify two rows return', async () => {
            await runDetailPage.resultStatusFilter.selectOption('PENDING');
            await expect(runDetailPage.resultRows).toHaveCount(2);
        });
    });

    test('filter row toggle shows and hides', async ({ runDetailPage, api }) => {
        let run;

        await test.step('Seed a folder, a test, a run, and a PENDING result via API', async () => {
            const stamp = Date.now();
            const folder = await api.createFolder(`CF2 Folder ${stamp}`);
            const t1 = await api.createTest(`CF2 Test A ${stamp}`, folder.id);
            run = await api.createRun(`CF2 Run ${stamp}`);
            await api.addRunResult(run.id, t1.id, { status: 'PENDING' });
        });

        await test.step('Open the run detail page and verify the toolbar is visible', async () => {
            await runDetailPage.open(run.id);
            await expect(runDetailPage.toolbar).toBeVisible({ timeout: TIMEOUTS.HEAVY_GRID });
        });

        await test.step('Show the filters and verify the status filter appears', async () => {
            await runDetailPage.openColumnFilters();
            await expect(runDetailPage.resultStatusFilter).toBeVisible();
        });

        await test.step('Hide the filters and verify the status filter disappears', async () => {
            await runDetailPage.hideColumnFilters();
            await expect(runDetailPage.resultStatusFilter).not.toBeVisible();
        });
    });
});
