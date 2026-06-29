import { test, expect } from '@playwright/test';

// Migrated from the removed `suites` concept to `categories` (Category Manager
// page at /categories). The suite manager UI was renamed to the category manager.
test.describe('Categories Page Bulk Actions', () => {
    test('should create multiple categories and delete them in bulk', async ({ page }) => {
        const timestamp = Date.now();
        const cat1 = `Bulk Cat A ${timestamp}`;
        const cat2 = `Bulk Cat B ${timestamp}`;

        // 1. Navigate to Categories page
        await page.goto('/categories');

        // 2. Create Category A via modal
        await page.getByTestId('open-create-category-modal').click();
        await page.getByTestId('category-name-input').fill(cat1);
        await page.getByTestId('create-category-button').click();
        await expect(page.getByText(cat1)).toBeVisible();

        // 3. Create Category B via modal
        await page.getByTestId('open-create-category-modal').click();
        await page.getByTestId('category-name-input').fill(cat2);
        await page.getByTestId('create-category-button').click();
        await expect(page.getByText(cat2)).toBeVisible();

        // 4. Select both via checkboxes
        const row1 = page.locator(`[data-testid^="category-row-"]`).filter({ hasText: cat1 });
        const row2 = page.locator(`[data-testid^="category-row-"]`).filter({ hasText: cat2 });

        await row1.locator('input[type="checkbox"]').check();
        await row2.locator('input[type="checkbox"]').check();

        // 5. Verify Bulk Delete button appears
        const bulkDeleteBtn = page.getByTestId('bulk-delete-categories-button');
        await expect(bulkDeleteBtn).toBeVisible();

        // 6. Perform Bulk Delete
        page.on('dialog', dialog => dialog.accept());
        await bulkDeleteBtn.click();

        // 7. Verify both are gone
        await expect(page.getByText(cat1)).not.toBeVisible();
        await expect(page.getByText(cat2)).not.toBeVisible();
    });

    test('should verify column layout details', async ({ page }) => {
        await page.goto('/categories');

        // Verify headers
        await expect(page.getByText('Name', { exact: true })).toBeVisible();
        await expect(page.getByText('Description', { exact: true })).toBeVisible();
        await expect(page.getByText('Created', { exact: true })).toBeVisible();
    });
});
