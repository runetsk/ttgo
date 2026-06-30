
import { test, expect } from '@playwright/test';

test.describe('Custom Fields Integration', () => {

    test('should define field and use it in test case', async ({ page }) => {
        const timestamp = Date.now();
        const fieldName = `Priority ${timestamp}`;
        const folderName = `CF Demo ${timestamp} `;
        const testName = `CF Test ${timestamp} `;

        await test.step('Define a SELECT custom field in settings', async () => {
            await page.goto('/settings');
            await page.getByPlaceholder('e.g. Priority').fill(fieldName);
            await page.locator('select').selectOption('SELECT');
            await page.getByPlaceholder('Low, Medium, High').fill('Low, High');
            await page.getByRole('button', { name: '+ Add Field' }).click();
            await expect(page.getByText(fieldName).first()).toBeVisible();
        });

        await test.step('Create a folder and a test case', async () => {
            await page.getByRole('button', { name: 'Tests' }).click();

            await page.getByTestId('create-root-folder-button').click();
            await page.getByTestId('modal-input').fill(folderName);
            await page.getByTestId('modal-confirm-button').click();
            await expect(page.getByText('New Root Folder')).not.toBeVisible();

            // Wait for folder
            await expect(page.getByTestId('folder-name').filter({ hasText: folderName })).toBeVisible();
            await page.getByTestId('folder-name').filter({ hasText: folderName }).click();

            await page.getByTestId('create-test-button').click();
            await page.getByTestId('modal-input').fill(testName);
            await page.getByTestId('modal-confirm-button').click();
        });

        await test.step('Open the test case and set the custom field value', async () => {
            // wait for the test case to appear in the grid after modal confirm
            const testGridEntry = page.locator('.test-grid-container').getByText(testName.trim()).first();
            await expect(testGridEntry).toBeVisible({ timeout: 10000 });
            await testGridEntry.click();
            await expect(page.getByTestId('test-case-name-input')).toBeVisible();

            // In the inline detail pane each custom field renders as a
            // .detail-pane-custom-key label followed by its <select> sibling.
            const fieldKey = page.locator('.detail-pane-custom-key').filter({ hasText: fieldName });
            await expect(fieldKey).toBeVisible();
            await fieldKey.locator('xpath=following-sibling::select[1]').selectOption('High');

            await page.getByRole('button', { name: 'Save Changes' }).click();
            await expect(page.getByTestId('test-case-name-input')).not.toBeVisible({ timeout: 10000 });
        });

        await test.step('Reopen the test case and verify the value persisted', async () => {
            const testGridEntry2 = page.locator('.test-grid-container').getByText(testName.trim()).first();
            await expect(testGridEntry2).toBeVisible({ timeout: 10000 });
            await testGridEntry2.click();
            await expect(page.getByTestId('test-case-name-input')).toBeVisible();
            const vKey = page.locator('.detail-pane-custom-key').filter({ hasText: fieldName });
            await expect(vKey).toBeVisible();
            await expect(vKey.locator('xpath=following-sibling::select[1]')).toHaveValue('High');
        });
    });
});
