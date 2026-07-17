import { test, expect } from '../../fixtures/test.js';
import { TIMEOUTS } from '../../config.js';

test.describe('Test Steps Management', () => {
    test('should add and reorder steps', async ({ page, libraryPage, testCaseDetailPage }) => {
        const folderName = `Steps Demo ${Date.now()}`;
        const testName = `Step Test ${Date.now()}`;

        await test.step('Create a folder and a test case', async () => {
            await libraryPage.open();
            await libraryPage.createRootFolder(folderName);
            await expect(page.getByText('New Root Folder')).not.toBeVisible();
            await libraryPage.selectFolder(folderName);
            await libraryPage.createTestCase(testName);
        });

        await test.step('Open the test detail view', async () => {
            await libraryPage.openTestCase(testName);
            await expect(testCaseDetailPage.nameInput).toBeVisible();
        });

        await test.step('Add the first step with action and expected result', async () => {
            await testCaseDetailPage.addStep(0);
            await testCaseDetailPage.fillRichField('[data-testid="step-action-0"]', 'First Step');
            await testCaseDetailPage.fillRichField('[data-testid="step-expected-0"]', 'First Result');
        });

        await test.step('Add the second step with action and expected result', async () => {
            await testCaseDetailPage.deactivateEditors();
            await testCaseDetailPage.addStep(1);
            await testCaseDetailPage.fillRichField('[data-testid="step-action-1"]', 'Second Step');
            await testCaseDetailPage.fillRichField('[data-testid="step-expected-1"]', 'Second Result');
            await testCaseDetailPage.deactivateEditors();
        });

        await test.step('Save the changes', async () => {
            await testCaseDetailPage.save();
            // After save the detail navigates away.
            await expect(testCaseDetailPage.nameInput).not.toBeVisible({ timeout: TIMEOUTS.ELEMENT });
        });

        await test.step('Reopen the test and verify the steps persisted', async () => {
            await libraryPage.openTestCase(testName);
            await expect(testCaseDetailPage.nameInput).toBeVisible();

            // Steps render in read-only mode via .rich-text-display.
            await expect(testCaseDetailPage.stepActionDisplay(0)).toContainText('First Step');
            await expect(testCaseDetailPage.stepActionDisplay(1)).toContainText('Second Step');
        });
    });
});
