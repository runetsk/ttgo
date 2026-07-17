import { test, expect } from '../../fixtures/test.js';

// A PNG signature is all the server's content-type sniff (F-020) needs to accept the upload.
const PNG = Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x00]);

test.describe('Screenshot upload', () => {
    test('attach a screenshot to a run result from the detail view', async ({ runDetailPage, api }) => {
        const stamp = Date.now();
        const testName = `Shot case ${stamp}`;
        let run;

        await test.step('Seed a run with one failing result', async () => {
            const folder = await api.createFolder('Shot Folder ' + stamp);
            const tc = await api.createTest(testName, folder.id);
            run = await api.createRun('Shot Run ' + stamp);
            await api.addRunResult(run.id, tc.id, { status: 'FAIL' });
        });

        await test.step('Expand the result row and attach a screenshot', async () => {
            await runDetailPage.open(run.id);
            await expect(runDetailPage.resultRow(testName)).toBeVisible();
            await runDetailPage.expandResultRow(testName);
            await expect(runDetailPage.resultDetail).toBeVisible();
            await expect(runDetailPage.attachScreenshots).toBeVisible();
            await runDetailPage.attachScreenshotsInput.setInputFiles({
                name: 'evidence.png', mimeType: 'image/png', buffer: PNG,
            });
        });

        await test.step('The uploaded screenshot appears in the gallery (via WS delta)', async () => {
            await expect(runDetailPage.artifactScreenshot).toBeVisible();
        });

        await test.step('The screenshot persisted to the API', async () => {
            const fresh = await api.getRun(run.id);
            const result = fresh.run_results.find(r => r.test_name_snapshot === testName);
            const shots = JSON.parse(result.screenshots || '[]');
            expect(shots.length).toBe(1);
            expect(shots[0]).toContain(`/api/uploads/screenshots/${result.id}/`);
        });
    });
});
