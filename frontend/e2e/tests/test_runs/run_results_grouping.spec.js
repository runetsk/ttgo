import { test, expect } from '../../fixtures/test.js';
import { TIMEOUTS } from '../../config.js';

// Verifies the run-results List/Grouped view ported from the legacy tree:
// toggle, group-by selector, group headers, collapse/expand, and that the
// view preference persists across a reload (localStorage).
test.describe('Run Results Grouping', () => {
    test('toggles list/grouped, shows group headers, collapses, and persists', async ({ page, runDetailPage, api }) => {
        let run;

        await test.step('Seed a folder, two tests, a category, and a run via API', async () => {
            const stamp = Date.now();
            const folder = await api.createFolder(`Grp Folder ${stamp}`);
            const t1 = await api.createTest(`Grp Test A ${stamp}`, folder.id);
            const t2 = await api.createTest(`Grp Test B ${stamp}`, folder.id);
            const category = await api.createCategory(`Grp Category ${stamp}`);
            await api.linkTestToCategory(t1.id, category.id);
            await api.linkTestToCategory(t2.id, category.id);
            run = await api.createRun(`Grp Run ${stamp}`, { categoryId: category.id });
        });

        await test.step('Open the run detail page and verify the toolbar in list view', async () => {
            await runDetailPage.open(run.id);
            await expect(runDetailPage.toolbar).toBeVisible({ timeout: TIMEOUTS.HEAVY_GRID });
        });

        await test.step('Switch to grouped view and verify the selector and a group header', async () => {
            await runDetailPage.groupedViewToggle.click();
            await expect(runDetailPage.groupBySelect).toBeVisible();
            await expect(runDetailPage.groupHeader.first()).toBeVisible();
        });

        await test.step('Use the collapse-all and expand-all controls', async () => {
            await runDetailPage.collapseAll();
            await runDetailPage.expandAll();
        });

        await test.step('Reload and verify the grouped-view preference persists', async () => {
            await page.reload();
            await page.waitForLoadState('domcontentloaded');
            await expect(runDetailPage.groupedViewToggle).toBeVisible({ timeout: TIMEOUTS.HEAVY_GRID });
            await expect(runDetailPage.groupBySelect).toBeVisible();
        });
    });
});
