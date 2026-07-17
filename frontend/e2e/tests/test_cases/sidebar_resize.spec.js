import { test, expect } from '../../fixtures/test.js';
import { TIMEOUTS } from '../../config.js';

test.describe('Sidebar Resize and Zoom', () => {
    test.beforeEach(async ({ libraryPage }) => {
        await libraryPage.open();
    });

    test('should resize sidebar', async ({ page, libraryPage }) => {
        let box;

        await test.step('Verify the sidebar starts at its default width', async () => {
            await expect(libraryPage.sidebar).toBeVisible({ timeout: TIMEOUTS.ELEMENT });
            // Initial width check (default 240 — see Sidebar.jsx sidebarWidth fallback)
            box = await libraryPage.sidebar.boundingBox();
            expect(box.width).toBeCloseTo(240, 0);
        });

        await test.step('Drag the resize handle and verify the new width', async () => {
            await libraryPage.resizeHandle.hover();
            await page.mouse.down();
            await page.mouse.move(400, 300); // Move to x=400
            await page.mouse.up();

            box = await libraryPage.sidebar.boundingBox();
            expect(box.width).toBeCloseTo(400, 0);
        });

        await test.step('Reload and verify the resized width persists', async () => {
            await page.reload();
            await expect(libraryPage.sidebar).toBeVisible();
            box = await libraryPage.sidebar.boundingBox();
            expect(box.width).toBeCloseTo(400, 0);
        });
    });

    test('should zoom sidebar', async ({ page, runsPage }) => {
        // The zoom control lives in the run-folder sidebar on /runs. Its zoomable
        // wrapper is the div that directly contains the "All Runs" entry and carries
        // the inline `font-size: <zoom>rem` style.
        const zoomWrapper = runsPage.allRunsEntry.locator('..');

        await test.step('Verify the folder tree starts at the default font size', async () => {
            await runsPage.open();
            await expect(runsPage.sidebar).toBeVisible({ timeout: TIMEOUTS.ELEMENT });
            // Default zoom is 1 → inline font-size: 1rem (16px computed).
            await expect(zoomWrapper).toHaveCSS('font-size', '16px');
        });

        await test.step('Zoom in and verify the font size increases', async () => {
            await page.getByTitle('Zoom In').click();
            // Assert the style attribute (1.1rem) — computed px can vary across systems.
            await expect(zoomWrapper).toHaveAttribute('style', /font-size: 1.1rem/);
        });

        await test.step('Reload and verify the zoom level persists', async () => {
            await page.reload();
            await expect(runsPage.sidebar).toBeVisible();
            await expect(zoomWrapper).toHaveAttribute('style', /font-size: 1.1rem/);
        });
    });
});
