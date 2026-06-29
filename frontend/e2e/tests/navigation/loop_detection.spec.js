import { test, expect } from '@playwright/test';

test.describe('Performance & Regression', () => {
    test.setTimeout(30000);

    test('should not trigger infinite request loop on folder deep link', async ({ page }) => {
        const folderName = `LoopCheck ${Date.now()}`;
        let folderRequestCount = 0;
        let testsRequestCount = 0;
        let folderId;

        await test.step('Create a root folder', async () => {
            // 1. Create Folder
            await page.goto('/');
            await page.getByTestId('create-root-folder-button').click();
            await page.getByTestId('modal-input').fill(folderName);
            await page.getByTestId('modal-confirm-button').click();

            await expect(page.getByTestId('folder-name').filter({ hasText: folderName })).toBeVisible();
        });

        await test.step('Open the folder and capture its ID', async () => {
            // 2. Click to Navigate and Capture ID
            await page.getByTestId('folder-name').filter({ hasText: folderName }).click();
            await expect(page.url()).toContain('/library/folders/');
            folderId = page.url().split('/library/folders/')[1];
        });

        await test.step('Reload the deep link and assert no request loop occurs', async () => {
            // 3. Monitor Requests
            // We will reload the page to test the "Deep Link" scenario which was the cause of the loop (FolderViewWrapper mount).

            page.on('request', request => {
                if (request.url().includes(`/api/folders/${folderId}`)) {
                    folderRequestCount++;
                    console.log(`Folder Req: ${request.url()}`);
                }
                if (request.url().includes('/api/tests') && request.url().includes(`folder_id=${folderId}`)) {
                    testsRequestCount++;
                }
            });

            console.log('Reloading page to trigger deep link logic...');
            await page.reload();

            // Wait for the grid title (fetch complete). The <h2.grid-title> also
            // contains a child rename (✏️) button, so its full text is
            // "<folderName>✏️" — assert containment rather than exact text.
            await expect(page.locator('h2.grid-title')).toContainText(folderName);

            // Wait a bit to ensure no subsequent loop happens (loops usually fire rapidly)
            await page.waitForTimeout(2000);

            console.log(`Requests detected - Folder: ${folderRequestCount}, Tests: ${testsRequestCount}`);

            // Assertions
            // In Strict Mode (Dev), components might mount twice, so 2 requests are acceptable.
            // A loop would produce dozens.
            // We accept up to 4 to be super safe against retries/strict mode, but typically it should be 1-2.
            expect(folderRequestCount).toBeLessThanOrEqual(4);
            expect(testsRequestCount).toBeLessThanOrEqual(15);
        });
    });
});
