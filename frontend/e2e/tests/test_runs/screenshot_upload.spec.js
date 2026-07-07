import { test, expect } from '@playwright/test';
import { API_URL } from '../../config.js';
import { createFolderAPI, createTestAPI, createRunAPI, addRunResultAPI } from '../../helpers/api.js';

// A PNG signature is all the server's content-type sniff (F-020) needs to accept the upload.
const PNG = Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x00]);

test.describe('Screenshot upload', () => {
    test('attach a screenshot to a run result from the detail view', async ({ page, request }) => {
        const stamp = Date.now();
        const testName = `Shot case ${stamp}`;
        let run;

        await test.step('Seed a run with one failing result', async () => {
            const folder = await createFolderAPI(request, 'Shot Folder ' + stamp);
            const tc = await createTestAPI(request, testName, folder.id);
            run = await createRunAPI(request, 'Shot Run ' + stamp);
            await addRunResultAPI(request, run.id, tc.id, { status: 'FAIL' });
        });

        await test.step('Expand the result row and attach a screenshot', async () => {
            await page.goto(`/runs/run/${run.id}`);
            await page.waitForLoadState('domcontentloaded');
            const row = page.getByRole('row', { name: testName });
            await expect(row).toBeVisible();
            // Click the chevron area of the test-case cell (left of the link) to expand
            // the row — the status/defect cells stop propagation.
            await row.locator('td').nth(1).click({ position: { x: 6, y: 8 } });
            await expect(page.getByTestId('run-result-detail')).toBeVisible();
            await expect(page.getByTestId('attach-screenshots')).toBeVisible();
            await page.getByTestId('attach-screenshots-input').setInputFiles({
                name: 'evidence.png', mimeType: 'image/png', buffer: PNG,
            });
        });

        await test.step('The uploaded screenshot appears in the gallery (via WS delta)', async () => {
            await expect(page.getByTestId('artifact-screenshot')).toBeVisible();
        });

        await test.step('The screenshot persisted to the API', async () => {
            const fresh = await request.get(`${API_URL}/runs/${run.id}`).then(r => r.json());
            const result = fresh.run_results.find(r => r.test_name_snapshot === testName);
            const shots = JSON.parse(result.screenshots || '[]');
            expect(shots.length).toBe(1);
            expect(shots[0]).toContain(`/api/uploads/screenshots/${result.id}/`);
        });
    });
});
