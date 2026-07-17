import { test, expect } from '../../fixtures/test.js';
import { TIMEOUTS } from '../../config.js';

test.describe('Custom Fields Settings', () => {

    test('should add and delete custom field definition', async ({ page, settingsPage }) => {
        const fieldName = `Priority ${Date.now()}`;

        await test.step('Open the settings page', async () => {
            await settingsPage.open();
            await expect(page.getByRole('heading', { name: 'Settings' })).toBeVisible();
        });

        await test.step('Create a SELECT custom field', async () => {
            await settingsPage.addCustomField({ name: fieldName, type: 'SELECT', options: 'Low, High' });
        });

        await test.step('Verify the new field appears in the list', async () => {
            await expect(page.getByText(fieldName).first()).toBeVisible();
            await expect(page.getByText('SELECT (Low, High)').first()).toBeVisible();
        });

        await test.step('Delete the custom field', async () => {
            const row = settingsPage.customFieldRow(fieldName);
            await expect(row).toBeVisible();
            await row.getByRole('button', { name: 'Delete' }).click({ force: true });
        });

        await test.step('Confirm the delete in the modal', async () => {
            await expect(page.getByText('Delete Custom Field')).toBeVisible();
            await settingsPage.confirmModal();
            await expect(page.getByText('Delete Custom Field')).not.toBeVisible();
        });

        await test.step('Verify the field row is gone', async () => {
            await expect(page.getByText(fieldName, { exact: true })).not.toBeVisible({ timeout: TIMEOUTS.APP_RENDER });
        });
    });
});
