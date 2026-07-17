import { test, expect } from '../../fixtures/test.js';
import { TIMEOUTS } from '../../config.js';

test.describe('Custom Fields Integration', () => {

    test('should define field and use it in test case', async ({ page, libraryPage, settingsPage, testCaseDetailPage }) => {
        const timestamp = Date.now();
        const fieldName = `Priority ${timestamp}`;
        const folderName = `CF Demo ${timestamp} `;
        const testName = `CF Test ${timestamp} `;

        await test.step('Define a SELECT custom field in settings', async () => {
            await settingsPage.open();
            await page.getByTestId('custom-field-name-input').fill(fieldName);
            await page.locator('select').selectOption('SELECT');
            await page.getByTestId('custom-field-options-input').fill('Low, High');
            await page.getByRole('button', { name: '+ Add Field' }).click();
            await expect(page.getByText(fieldName).first()).toBeVisible();
        });

        await test.step('Create a folder and a test case', async () => {
            await libraryPage.nav('Tests');
            await libraryPage.createRootFolder(folderName);
            await expect(page.getByText('New Root Folder')).not.toBeVisible();
            await libraryPage.selectFolder(folderName);
            await libraryPage.createTestCase(testName);
        });

        await test.step('Open the test case and set the custom field value', async () => {
            await libraryPage.openTestCase(testName);
            await expect(testCaseDetailPage.nameInput).toBeVisible();

            await expect(testCaseDetailPage.customFieldKey(fieldName)).toBeVisible();
            await testCaseDetailPage.customFieldSelect(fieldName).selectOption('High');

            await testCaseDetailPage.save();
            await expect(testCaseDetailPage.nameInput).not.toBeVisible({ timeout: TIMEOUTS.ELEMENT });
        });

        await test.step('Reopen the test case and verify the value persisted', async () => {
            await libraryPage.openTestCase(testName);
            await expect(testCaseDetailPage.nameInput).toBeVisible();
            await expect(testCaseDetailPage.customFieldKey(fieldName)).toBeVisible();
            await expect(testCaseDetailPage.customFieldSelect(fieldName)).toHaveValue('High');
        });
    });
});
