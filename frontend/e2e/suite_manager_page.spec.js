import { test, expect } from '@playwright/test';

// Migrated from the removed `suites` concept to `categories`. The Suite Manager
// page is now the Category Manager page at /categories.
test.describe('Category Manager Page', () => {
    test.setTimeout(30000);

    test('should navigate to categories page and manage categories', async ({ page }) => {
        const catName = `Category Page Test ${Date.now()}`;

        await page.goto('/');

        // 1. Navigate to Categories Page via the top nav
        await page.getByRole('button', { name: 'Categories' }).click();
        await expect(page.url()).toContain('/categories');
        await expect(page.getByTestId('category-manager')).toBeVisible();

        // 2. Create a Category via modal
        await page.getByTestId('open-create-category-modal').click();
        await page.getByTestId('category-name-input').fill(catName);
        await page.getByTestId('create-category-button').click();

        // 3. Verify category creation
        await expect(page.getByText(catName)).toBeVisible();

        // 4. Navigate back to Tests
        await page.getByRole('button', { name: 'Tests' }).click();
        await expect(page.url()).not.toContain('/categories');
    });
});
