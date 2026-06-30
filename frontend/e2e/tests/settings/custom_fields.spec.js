import { test, expect } from '@playwright/test';

test.describe('Custom Fields Settings', () => {

    test('should add and delete custom field definition', async ({ page }) => {
        const fieldName = `Priority ${Date.now()}`;

        await test.step('Open the settings page', async () => {
            await page.goto('/settings');
            await expect(page.getByRole('heading', { name: 'Settings' })).toBeVisible();
        });

        await test.step('Create a SELECT custom field', async () => {
            await page.getByPlaceholder('e.g. Priority').fill(fieldName);
            await page.locator('select').selectOption('SELECT');
            await page.getByPlaceholder('Low, Medium, High').fill('Low, High');
            await page.getByRole('button', { name: '+ Add Field' }).click();
        });

        await test.step('Verify the new field appears in the list', async () => {
            await expect(page.getByText(fieldName).first()).toBeVisible();
            await expect(page.getByText('SELECT (Low, High)').first()).toBeVisible();
        });

        await test.step('Delete the custom field', async () => {
            const row = page.locator('.glass-panel').filter({ hasText: fieldName }).first();
            await expect(row).toBeVisible();
            await row.getByRole('button', { name: 'Delete' }).click({ force: true });
        });

        await test.step('Confirm the delete in the modal', async () => {
            await expect(page.getByText('Delete Custom Field')).toBeVisible(); // Title
            await page.getByTestId('modal-confirm-button').click();
            // Wait for modal to close
            await expect(page.getByText('Delete Custom Field')).not.toBeVisible();
        });

        await test.step('Verify the field row is gone', async () => {
            await expect(page.getByText(fieldName, { exact: true })).not.toBeVisible({ timeout: 15000 });
        });
    });
});
