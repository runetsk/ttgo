
import { test, expect } from '@playwright/test';

test.describe('Custom Fields Integration', () => {
    test.setTimeout(60000);

    test('should define field and use it in test case', async ({ page }) => {
        const timestamp = Date.now();
        const fieldName = `Priority ${timestamp}`;
        const folderName = `CF Demo ${timestamp} `;
        const testName = `CF Test ${timestamp} `;

        // 1. Define Field
        await page.goto('/settings');
        await page.getByPlaceholder('e.g. Priority').fill(fieldName);
        await page.locator('select').selectOption('SELECT');
        await page.getByPlaceholder('Low, Medium, High').fill('Low, High');
        await page.getByRole('button', { name: '+ Add Field' }).click();
        await expect(page.getByText(fieldName).first()).toBeVisible();

        // 2. Create Test
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

        // 3. Set Value — wait for the test case to appear in the grid after modal confirm
        const testGridEntry = page.locator('.test-grid-container').getByText(testName.trim()).first();
        await expect(testGridEntry).toBeVisible({ timeout: 10000 });
        await testGridEntry.click();
        await expect(page.getByTestId('test-case-name-input')).toBeVisible();

        // The field name is truncated to 14 chars in the meta bar label.
        // Use the .meta-field[title] attribute (which holds the full name) to locate it.
        const fieldContainer = page.locator(`.meta-field[title="${fieldName}"]`);
        await expect(fieldContainer).toBeVisible();
        await fieldContainer.getByRole('combobox').selectOption('High');

        await page.getByRole('button', { name: 'Save Changes' }).click();
        await expect(page.getByTestId('test-case-name-input')).not.toBeVisible({ timeout: 10000 });

        // 4. Verify Persistence
        const testGridEntry2 = page.locator('.test-grid-container').getByText(testName.trim()).first();
        await expect(testGridEntry2).toBeVisible({ timeout: 10000 });
        await testGridEntry2.click();
        await expect(page.getByTestId('test-case-name-input')).toBeVisible();
        const vContainer = page.locator(`.meta-field[title="${fieldName}"]`);
        await expect(vContainer).toBeVisible();
        await expect(vContainer.getByRole('combobox')).toHaveValue('High');
    });
});
