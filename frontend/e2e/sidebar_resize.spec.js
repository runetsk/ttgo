
import { test, expect } from '@playwright/test';

test.describe('Sidebar Resize and Zoom', () => {
    test.beforeEach(async ({ page }) => {
        await page.goto('/');
    });

    test('should resize sidebar', async ({ page }) => {
        const sidebar = page.getByTestId('sidebar');
        await expect(sidebar).toBeVisible({ timeout: 10000 });
        const handle = page.locator('.resize-handle');

        // Initial width check (default 280)
        let box = await sidebar.boundingBox();
        expect(box.width).toBeCloseTo(280, 0);

        // Drag to resize
        await handle.hover();
        await page.mouse.down();
        await page.mouse.move(400, 300); // Move to x=400
        await page.mouse.up();

        // Check new width
        box = await sidebar.boundingBox();
        expect(box.width).toBeCloseTo(400, 0);

        // Reload and check persistence
        await page.reload();
        const sidebarReloaded = page.getByTestId('sidebar');
        await expect(sidebarReloaded).toBeVisible();
        box = await sidebarReloaded.boundingBox();
        expect(box.width).toBeCloseTo(400, 0);
    });

    test('should zoom sidebar', async ({ page }) => {
        const sidebar = page.getByTestId('sidebar');
        await expect(sidebar).toBeVisible({ timeout: 10000 });

        // Initial check (default 1rem = 16px usually, but checking style)
        // We set fontSize inline, so we can check that.
        // Initially it might be 1rem (from state default)
        await expect(page.locator('.folder-tree')).toHaveCSS('font-size', '16px');

        // Zoom In
        await page.getByTitle('Zoom In').click();

        // Check new size (1.1rem -> ~17.6px)
        // Check new size (1.1rem -> ~17.6px)
        // Check style attribute because computed CSS might vary across browsers/systems
        const folderTree = page.locator('.folder-tree');
        await expect(folderTree).toHaveAttribute('style', /font-size: 1.1rem/);

        // Reload and check persistence
        await page.reload();
        const sidebarReloaded = page.getByTestId('sidebar');
        await expect(sidebarReloaded).toBeVisible();
        const folderTreeReloaded = page.locator('.folder-tree');
        await expect(folderTreeReloaded).toHaveAttribute('style', /font-size: 1.1rem/);
    });
});
