import { test, expect } from '@playwright/test';

// Migrated from the removed `suites` concept to `categories` (Category Manager
// page at /categories). The suite manager UI was renamed to the category manager.
test.describe('Categories Page Bulk Actions', () => {
    test('should create multiple categories and delete them in bulk', async ({ page }) => {
        const timestamp = Date.now();
        const cat1 = `Bulk Cat A ${timestamp}`;
        const cat2 = `Bulk Cat B ${timestamp}`;

        let row1;
        let row2;
        let bulkDeleteBtn;

        await test.step('Navigate to the Categories page', async () => {
            await page.goto('/categories');
        });

        await test.step('Create Category A via the modal', async () => {
            await page.getByTestId('open-create-category-modal').click();
            await page.getByTestId('category-name-input').fill(cat1);
            await page.getByTestId('create-category-button').click();
            await expect(page.getByText(cat1)).toBeVisible();
        });

        await test.step('Create Category B via the modal', async () => {
            await page.getByTestId('open-create-category-modal').click();
            await page.getByTestId('category-name-input').fill(cat2);
            await page.getByTestId('create-category-button').click();
            await expect(page.getByText(cat2)).toBeVisible();
        });

        await test.step('Select both categories via checkboxes', async () => {
            row1 = page.locator(`[data-testid^="category-row-"]`).filter({ hasText: cat1 });
            row2 = page.locator(`[data-testid^="category-row-"]`).filter({ hasText: cat2 });

            await row1.locator('input[type="checkbox"]').check();
            await row2.locator('input[type="checkbox"]').check();
        });

        await test.step('Verify the Bulk Delete button appears', async () => {
            bulkDeleteBtn = page.getByTestId('bulk-delete-categories-button');
            await expect(bulkDeleteBtn).toBeVisible();
        });

        await test.step('Perform the bulk delete', async () => {
            page.on('dialog', dialog => dialog.accept());
            await bulkDeleteBtn.click();
        });

        await test.step('Verify both categories are gone', async () => {
            await expect(page.getByText(cat1)).not.toBeVisible();
            await expect(page.getByText(cat2)).not.toBeVisible();
        });
    });

    test('should verify column layout details', async ({ page }) => {
        await test.step('Open the Categories page and verify the column headers', async () => {
            await page.goto('/categories');

            // Verify headers
            await expect(page.getByText('Name', { exact: true })).toBeVisible();
            await expect(page.getByText('Description', { exact: true })).toBeVisible();
            await expect(page.getByText('Created', { exact: true })).toBeVisible();
        });
    });
});
