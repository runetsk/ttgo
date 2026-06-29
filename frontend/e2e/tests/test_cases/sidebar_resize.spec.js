
import { test, expect } from '@playwright/test';

test.describe('Sidebar Resize and Zoom', () => {
    test.beforeEach(async ({ page }) => {
        await page.goto('/');
    });

    test('should resize sidebar', async ({ page }) => {
        let sidebar;
        let handle;
        let box;

        await test.step('Verify the sidebar starts at its default width', async () => {
            sidebar = page.getByTestId('sidebar');
            await expect(sidebar).toBeVisible({ timeout: 10000 });
            handle = page.locator('.resize-handle');

            // Initial width check (default 240 — see Sidebar.jsx sidebarWidth fallback)
            box = await sidebar.boundingBox();
            expect(box.width).toBeCloseTo(240, 0);
        });

        await test.step('Drag the resize handle and verify the new width', async () => {
            await handle.hover();
            await page.mouse.down();
            await page.mouse.move(400, 300); // Move to x=400
            await page.mouse.up();

            // Check new width
            box = await sidebar.boundingBox();
            expect(box.width).toBeCloseTo(400, 0);
        });

        await test.step('Reload and verify the resized width persists', async () => {
            await page.reload();
            const sidebarReloaded = page.getByTestId('sidebar');
            await expect(sidebarReloaded).toBeVisible();
            box = await sidebarReloaded.boundingBox();
            expect(box.width).toBeCloseTo(400, 0);
        });
    });

    test('should zoom sidebar', async ({ page }) => {
        // The zoom control lives in the run-folder sidebar on /runs. Its zoomable
        // wrapper is the div that directly contains the "All Runs" entry and carries
        // the inline `font-size: <zoom>rem` style.
        const zoomWrapper = page.getByTestId('all-runs-entry').locator('..');

        await test.step('Verify the folder tree starts at the default font size', async () => {
            await page.goto('/runs');
            const sidebar = page.getByTestId('run-folder-sidebar');
            await expect(sidebar).toBeVisible({ timeout: 10000 });

            // Default zoom is 1 → inline font-size: 1rem (16px computed).
            await expect(zoomWrapper).toHaveCSS('font-size', '16px');
        });

        await test.step('Zoom in and verify the font size increases', async () => {
            // Zoom In
            await page.getByTitle('Zoom In').click();

            // Check new size (1.1rem -> ~17.6px)
            // Check the style attribute because computed CSS might vary across browsers/systems
            await expect(zoomWrapper).toHaveAttribute('style', /font-size: 1.1rem/);
        });

        await test.step('Reload and verify the zoom level persists', async () => {
            await page.reload();
            const sidebarReloaded = page.getByTestId('run-folder-sidebar');
            await expect(sidebarReloaded).toBeVisible();
            await expect(zoomWrapper).toHaveAttribute('style', /font-size: 1.1rem/);
        });
    });
});
