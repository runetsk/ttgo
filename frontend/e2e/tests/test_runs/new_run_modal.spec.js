import { test, expect } from '../../fixtures/test.js';

test.describe('New run modal — tree picker', () => {
    test('pick a folder in the tree, create the run, and see it in the sidebar', async ({ page, runsPage, runDetailPage, api }) => {
        const stamp = Date.now();
        const folderName = `Tree Folder ${stamp}`;
        const runName = `Tree Run ${stamp}`;
        const names = [`Tree-A ${stamp}`, `Tree-B ${stamp}`];
        let folder;

        await test.step('Seed a folder with two tests', async () => {
            folder = await api.createFolder(folderName);
            for (const n of names) await api.createTest(n, folder.id);
        });

        await test.step('Open the modal; the tree shows the folder and tests', async () => {
            await runsPage.open();
            await runsPage.openNewRunModal();
            await expect(runsPage.treePicker).toBeVisible();
            await expect(runsPage.treeFolder(folder.id)).toBeVisible();
        });

        await test.step('Ticking the folder selects both of its tests', async () => {
            await runsPage.runNameInput.fill(runName);
            await runsPage.treeFolder(folder.id).check();
            await expect(runsPage.treeSelectedCount).toContainText('2');
        });

        await test.step('Create Run lands on the run detail with both tests', async () => {
            await runsPage.submitNewRun();
            await expect(page).toHaveURL(/\/runs\/run\/[a-f0-9-]+$/);
            // stats-passed renders "{passed} / {total}" — both tests are in the run.
            await expect(runDetailPage.stat('passed')).toContainText('/ 2');
        });

        await test.step('The new run appears in the left run sidebar without a refresh', async () => {
            await runsPage.expandUncategorised();
            await expect(runsPage.sidebar.getByText(runName)).toBeVisible();
        });
    });

    test('run detail Execute runs only the checked results', async ({ page, runDetailPage, runExecutePage, api }) => {
        const stamp = Date.now();
        const names = ['Alpha exec', 'Beta exec', 'Gamma exec'];
        let run;
        const tc = {};

        await test.step('Seed a run with three cases via API', async () => {
            const folder = await api.createFolder(`Exec Folder ${stamp}`);
            run = await api.createRun(`Exec Run ${stamp}`);
            for (const n of names) {
                tc[n] = await api.createTest(n, folder.id);
                await api.addRunResult(run.id, tc[n].id);
            }
        });

        await test.step('With nothing checked, Execute is disabled and countless', async () => {
            await runDetailPage.open(run.id);
            await expect(runDetailPage.stat('passed')).toContainText('/ 3');
            await expect(runDetailPage.executeRunButton).toBeDisabled();
            await expect(runDetailPage.executeRunButton).not.toContainText('(');
        });

        await test.step('Checking two rows enables Execute and shows the count (2)', async () => {
            // The row checkbox is a custom-styled label wrapping a collapsed native
            // input; clicking the label natively toggles it and fires onChange.
            await runDetailPage.selectResult(tc['Beta exec'].id).click();
            await runDetailPage.selectResult(tc['Gamma exec'].id).click();
            await expect(runDetailPage.executeRunButton).toBeEnabled();
            await expect(runDetailPage.executeRunButton).toContainText('(2)');
        });

        await test.step('Execute scopes the queue to just those two tests', async () => {
            await runDetailPage.executeRunButton.click();
            await expect(page).toHaveURL(/\/execute\?only=/);
            await expect(runExecutePage.progress).toContainText('0 / 2');
            // Queue is sorted by name, so Beta is current and Alpha is absent.
            await expect(runExecutePage.currentName).toHaveText('Beta exec');
        });
    });
});
