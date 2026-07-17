import { test, expect } from '../../fixtures/test.js';

// Migrated from the removed `suites` concept to `categories` (Category Manager
// page at /categories). The suite manager UI was renamed to the category manager.
test.describe('Categories Page Bulk Actions', () => {
    test('should create multiple categories and delete them in bulk', async ({ page, categoriesPage }) => {
        const timestamp = Date.now();
        const cat1 = `Bulk Cat A ${timestamp}`;
        const cat2 = `Bulk Cat B ${timestamp}`;

        await test.step('Navigate to the Categories page', async () => {
            await categoriesPage.open();
        });

        await test.step('Create Category A via the modal', async () => {
            await categoriesPage.create(cat1);
            await expect(page.getByText(cat1)).toBeVisible();
        });

        await test.step('Create Category B via the modal', async () => {
            await categoriesPage.create(cat2);
            await expect(page.getByText(cat2)).toBeVisible();
        });

        await test.step('Select both categories via checkboxes', async () => {
            await categoriesPage.selectCategory(cat1);
            await categoriesPage.selectCategory(cat2);
        });

        await test.step('Verify the Bulk Delete button appears', async () => {
            await expect(categoriesPage.bulkDeleteButton).toBeVisible();
        });

        await test.step('Perform the bulk delete', async () => {
            await categoriesPage.bulkDelete();
        });

        await test.step('Verify both categories are gone', async () => {
            await expect(page.getByText(cat1)).not.toBeVisible();
            await expect(page.getByText(cat2)).not.toBeVisible();
        });
    });

    test('should verify column layout details', async ({ page, api, categoriesPage }) => {
        await test.step('Seed a category so the table (with its headers) renders', async () => {
            // The manager shows an empty state — no table headers — when there are
            // zero categories, so ensure at least one exists first.
            await api.createCategory(`Layout Cat ${Date.now()}`);
        });

        await test.step('Open the Categories page and verify the column headers', async () => {
            await categoriesPage.open();
            await expect(page.getByText('Name', { exact: true })).toBeVisible();
            await expect(page.getByText('Description', { exact: true })).toBeVisible();
            await expect(page.getByText('Created', { exact: true })).toBeVisible();
        });
    });
});
