import { test, expect } from '../../fixtures/test.js';

test.describe('Add tests to an open run — tree picker', () => {
    test('locks tests already in the run and adds a newly picked one', async ({ runDetailPage, api }) => {
        const stamp = Date.now();
        const names = [`Add-A ${stamp}`, `Add-B ${stamp}`];
        let run;
        const tc = {};

        await test.step('Seed a folder with two tests; a run holding only the first', async () => {
            const folder = await api.createFolder(`Add Folder ${stamp}`);
            run = await api.createRun(`Add Run ${stamp}`);
            for (const n of names) tc[n] = await api.createTest(n, folder.id);
            await api.addRunResult(run.id, tc[names[0]].id); // A already in the run
        });

        await test.step('Open the run — only test A is present', async () => {
            await runDetailPage.open(run.id);
            await expect(runDetailPage.stat('passed')).toContainText('/ 1');
        });

        await test.step('Open the add modal — test A is locked (checked + disabled)', async () => {
            await runDetailPage.openAddTests();
            await expect(runDetailPage.treePicker).toBeVisible();
            const lockedA = runDetailPage.treeTest(tc[names[0]].id);
            await expect(lockedA).toBeChecked();
            await expect(lockedA).toBeDisabled();
        });

        await test.step('Tick test B and add it', async () => {
            await runDetailPage.treeTest(tc[names[1]].id).check();
            await expect(runDetailPage.treeSelectedCount).toContainText('1');
            await runDetailPage.submitAddTests();
        });

        await test.step('The run now holds both tests', async () => {
            await expect(runDetailPage.treePicker).not.toBeVisible(); // modal closed
            await expect(runDetailPage.selectResult(tc[names[1]].id)).toBeVisible();
            await expect(runDetailPage.stat('passed')).toContainText('/ 2');
        });
    });
});
