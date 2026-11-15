import { test, expect } from '../../fixtures/test.js';
import { TIMEOUTS } from '../../config.js';

test.describe('Test Steps Management', () => {
    test('editing and reordering existing steps and clearing the description survive reload', async ({ page, api, testCaseDetailPage }) => {
        const folder = await api.createFolder('Existing steps ' + Date.now());
        const tc = await api.createTest('Edit existing steps', folder.id, '<p>Clear me</p>', {
            steps: [
                { action: '<p>First</p>', expected_result: '<p>Expected</p>', order_index: 0 },
                { action: '<p>Second</p>', expected_result: '', order_index: 1 },
            ],
        });
        await testCaseDetailPage.open(tc.id);
        await testCaseDetailPage.fillRichField('[data-testid="step-action-0"]', ' edited');
        await testCaseDetailPage.deactivateEditors();
        const description = page.getByTestId('test-case-description-input');
        await description.locator('.rich-text-field').click();
        await description.locator('.ProseMirror').click();
        await page.keyboard.press('ControlOrMeta+A');
        await page.keyboard.press('Backspace');
        await testCaseDetailPage.deactivateEditors();
        await expect(description.locator('.rich-text-placeholder')).toBeVisible();
        const handle = page.getByRole('button', { name: '☰', exact: true }).first();
        const before = await handle.boundingBox();
        const target = await page.getByRole('button', { name: '☰', exact: true }).nth(1).boundingBox();
        await page.mouse.move(before.x + before.width / 2, before.y + before.height / 2);
        await page.mouse.down();
        await page.mouse.move(before.x + before.width / 2, before.y + before.height / 2 + 5);
        await expect(handle).toHaveAttribute('aria-pressed', 'true');
        await page.mouse.move(target.x + target.width / 2, target.y + target.height / 2, { steps: 10 });
        await expect.poll(async () => (await handle.boundingBox()).y).toBeGreaterThan(before.y);
        await page.mouse.up();
        await expect(testCaseDetailPage.stepActionDisplay(0)).toContainText('Second');
        await testCaseDetailPage.save();
        await expect(testCaseDetailPage.nameInput).not.toBeVisible();
        await testCaseDetailPage.open(tc.id);
        await expect(testCaseDetailPage.stepActionDisplay(0)).toContainText('Second');
        await expect(testCaseDetailPage.stepActionDisplay(1)).toContainText('First edited');
        await expect(page.getByTestId('test-case-description-input').locator('.rich-text-display')).toBeEmpty();
    });

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
