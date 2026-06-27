import { test, expect } from '@playwright/test';

test.describe('Suites Page Bulk Actions', () => {
    test('should create multiple suites and delete them in bulk', async ({ page }) => {
        const timestamp = Date.now();
        const suite1 = `Bulk Suite A ${timestamp}`;
        const suite2 = `Bulk Suite B ${timestamp}`;

        // 1. Navigate to Suites page
        await page.goto('/suites');

        // 2. Create Suite A via modal
        await page.getByTestId('open-create-suite-modal').click();
        await page.getByTestId('suite-name-input').fill(suite1);
        await page.getByTestId('create-suite-button').click();
        await expect(page.getByText(suite1)).toBeVisible();

        // 3. Create Suite B via modal
        await page.getByTestId('open-create-suite-modal').click();
        await page.getByTestId('suite-name-input').fill(suite2);
        await page.getByTestId('create-suite-button').click();
        await expect(page.getByText(suite2)).toBeVisible();

        // 4. Select both via checkboxes
        const row1 = page.locator(`[data-testid^="suite-row-"]`).filter({ hasText: suite1 });
        const row2 = page.locator(`[data-testid^="suite-row-"]`).filter({ hasText: suite2 });

        await row1.locator('input[type="checkbox"]').check();
        await row2.locator('input[type="checkbox"]').check();

        // 5. Verify Bulk Delete button appears
        const bulkDeleteBtn = page.getByTestId('bulk-delete-suites-button');
        await expect(bulkDeleteBtn).toBeVisible();
        await expect(bulkDeleteBtn).toContainText('Delete 2 selected');

        // 6. Perform Bulk Delete
        page.on('dialog', dialog => dialog.accept());
        await bulkDeleteBtn.click();

        // 7. Verify both are gone
        await expect(page.getByText(suite1)).not.toBeVisible();
        await expect(page.getByText(suite2)).not.toBeVisible();
    });

    test('should verify column layout details', async ({ page }) => {
        await page.goto('/suites');

        // Verify headers
        await expect(page.getByText('Name', { exact: true })).toBeVisible();
        await expect(page.getByText('Description', { exact: true })).toBeVisible();
        await expect(page.getByText('Created', { exact: true })).toBeVisible();
    });
});
