import { test, expect } from '@playwright/test';

// Migrated from the removed `suites` concept to `categories`. The Suite Manager
// page is now the Category Manager page at /categories.
test.describe('Category Manager Page', () => {
    test.setTimeout(30000);

    test('should navigate to categories page and manage categories', async ({ page }) => {
        const catName = `Category Page Test ${Date.now()}`;

        await test.step('Open the home page', async () => {
            await page.goto('/');
        });

        await test.step('Navigate to the Categories page via the top nav', async () => {
            await page.getByRole('button', { name: 'Categories' }).click();
            await expect(page.url()).toContain('/categories');
            await expect(page.getByTestId('category-manager')).toBeVisible();
        });

        await test.step('Create a category via the modal', async () => {
            await page.getByTestId('open-create-category-modal').click();
            await page.getByTestId('category-name-input').fill(catName);
            await page.getByTestId('create-category-button').click();
        });

        await test.step('Verify the category was created', async () => {
            await expect(page.getByText(catName)).toBeVisible();
        });

        await test.step('Navigate back to Tests', async () => {
            await page.getByRole('button', { name: 'Tests' }).click();
            await expect(page.url()).not.toContain('/categories');
        });
    });
});
