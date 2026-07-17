import { test, expect } from '../../fixtures/test.js';

// Migrated from the removed `suites` concept to `categories`. The Suite Manager
// page is now the Category Manager page at /categories.
test.describe('Category Manager Page', () => {

    test('should navigate to categories page and manage categories', async ({ page, libraryPage, categoriesPage }) => {
        const catName = `Category Page Test ${Date.now()}`;

        await test.step('Open the home page', async () => {
            await libraryPage.open();
        });

        await test.step('Navigate to the Categories page via the Quality section', async () => {
            // Categories lives under the top-nav "Quality" section, not a direct button.
            await libraryPage.nav('Quality');
            await libraryPage.nav('Categories');
            await expect(page.url()).toContain('/categories');
            await expect(page.getByTestId('category-manager')).toBeVisible();
        });

        await test.step('Create a category via the modal', async () => {
            await categoriesPage.create(catName);
        });

        await test.step('Verify the category was created', async () => {
            await expect(page.getByText(catName)).toBeVisible();
        });

        await test.step('Navigate back to Tests', async () => {
            await libraryPage.nav('Tests');
            await expect(page.url()).not.toContain('/categories');
        });
    });
});
