import { test, expect } from '../../fixtures/test.js';
import { SLEEPS } from '../../config.js';
import { countRequests } from '../../helpers/diagnostics.js';

test.describe('Performance & Regression', () => {

    test('should not trigger infinite request loop on folder deep link', async ({ page, libraryPage }) => {
        const folderName = `LoopCheck ${Date.now()}`;
        let folderId;

        await test.step('Create a root folder', async () => {
            await libraryPage.open();
            await libraryPage.createRootFolder(folderName);
        });

        await test.step('Open the folder and capture its ID', async () => {
            await libraryPage.selectFolder(folderName);
            await expect(page.url()).toContain('/library/folders/');
            folderId = page.url().split('/library/folders/')[1];
        });

        await test.step('Reload the deep link and assert no request loop occurs', async () => {
            // Start counting only the reload's traffic (the deep-link mount was the loop cause).
            const folderReqs = countRequests(page, `/api/folders/${folderId}`);
            const testsReqs = countRequests(page, new RegExp(`/api/tests\\?.*folder_id=${folderId}`));

            await page.reload();

            // grid-title holds a child rename (✏️) button, so assert containment.
            await expect(page.locator('h2.grid-title')).toContainText(folderName);

            // Loops fire rapidly — wait a window to confirm none happens.
            await page.waitForTimeout(SLEEPS.LOOP_OBSERVE);

            // Strict Mode may double-mount, so allow a few; a loop would produce dozens.
            expect(folderReqs.count).toBeLessThanOrEqual(4);
            expect(testsReqs.count).toBeLessThanOrEqual(15);
        });
    });
});
