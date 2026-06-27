import { test, expect } from '@playwright/test';

test.describe('Custom Fields Settings', () => {
    test.setTimeout(60000);

    test('should add and delete custom field definition', async ({ page }) => {
        const fieldName = `Priority ${Date.now()}`;

        await page.goto('/settings');
        await expect(page.getByRole('heading', { name: 'Settings' })).toBeVisible();

        // Create Field
        await page.getByPlaceholder('e.g. Priority').fill(fieldName);
        await page.locator('select').selectOption('SELECT');
        await page.getByPlaceholder('Low, Medium, High').fill('Low, High');
        await page.getByRole('button', { name: '+ Add Field' }).click();

        // Verify List
        await expect(page.getByText(fieldName).first()).toBeVisible();
        await expect(page.getByText('SELECT (Low, High)').first()).toBeVisible();

        // Delete
        const row = page.locator('.glass-panel').filter({ hasText: fieldName }).first();
        await expect(row).toBeVisible();
        await row.getByRole('button', { name: 'Delete' }).click({ force: true });

        // Confirm Modal
        await expect(page.getByText('Delete Custom Field')).toBeVisible(); // Title
        await page.getByTestId('modal-confirm-button').click();

        // Wait for modal to close
        await expect(page.getByText('Delete Custom Field')).not.toBeVisible();

        // Verify row is gone
        await expect(page.getByText(fieldName, { exact: true })).not.toBeVisible({ timeout: 15000 });
    });
});
