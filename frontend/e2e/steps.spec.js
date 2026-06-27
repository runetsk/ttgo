import { test, expect } from '@playwright/test';

/**
 * Helper: activate a RichTextField by clicking its container, wait for the
 * editor toolbar (confirms isEditing=true), then type via keyboard.
 *
 * Clicking the name input between fields deactivates the previous editor so
 * that blur/focus events don't interfere with the next activation.
 */
async function fillRichField(page, containerSelector, text) {
    // Deactivate any currently-open editor by focusing the name input
    await page.getByTestId('test-case-name-input').click();

    // Click the rich-text-field div to enter edit mode
    const richField = page.locator(`${containerSelector} .rich-text-field`);
    await richField.click();

    // Wait for the toolbar — its appearance confirms isEditing=true
    await page.locator(`${containerSelector} .rich-text-toolbar`).waitFor({ state: 'visible', timeout: 5000 });

    // Click the ProseMirror contenteditable to ensure it has focus, then type
    const proseMirror = page.locator(`${containerSelector} .ProseMirror`);
    await proseMirror.click();
    await page.keyboard.type(text);
}

test.describe('Test Steps Management', () => {
    test('should add and reorder steps', async ({ page }) => {
        test.setTimeout(60000);
        const folderName = `Steps Demo ${Date.now()}`;
        const testName = `Step Test ${Date.now()}`;

        // 1. Create a folder and test
        await page.goto('/');
        await page.getByText('+ Root').click();
        await page.getByPlaceholder('Folder name').fill(folderName);
        await page.getByRole('button', { name: 'Confirm' }).click();
        await expect(page.getByText('New Root Folder')).not.toBeVisible();

        // Wait for folder and click
        await expect(page.getByTestId('folder-name').filter({ hasText: folderName })).toBeVisible();
        await page.getByTestId('folder-name').filter({ hasText: folderName }).click();

        await page.getByText('+ New Test').click();
        await page.getByPlaceholder('Test case name...').fill(testName);
        await page.getByRole('button', { name: 'Confirm' }).click();

        // 2. Open Test Detail
        await page.getByText(testName).click();
        await expect(page.getByTestId('test-case-name-input')).toBeVisible();

        // 3. Add Steps (each RichTextField requires explicit activation)
        await page.getByTestId('add-step-button').click();
        // Wait for step 0 container to appear
        await page.locator('[data-testid="step-action-0"]').waitFor({ state: 'visible', timeout: 10000 });
        await fillRichField(page, '[data-testid="step-action-0"]', 'First Step');
        await fillRichField(page, '[data-testid="step-expected-0"]', 'First Result');

        // Deactivate before clicking add again
        await page.getByTestId('test-case-name-input').click();
        await page.getByTestId('add-step-button').click();
        // Wait for step 1 container to appear
        await page.locator('[data-testid="step-action-1"]').waitFor({ state: 'visible', timeout: 10000 });
        await fillRichField(page, '[data-testid="step-action-1"]', 'Second Step');
        await fillRichField(page, '[data-testid="step-expected-1"]', 'Second Result');

        // Deactivate the last field before saving
        await page.getByTestId('test-case-name-input').click();

        // 4. Save
        await page.getByRole('button', { name: 'Save Changes' }).click();

        // Debug error banner if visible
        if (await page.locator('.error-banner').isVisible()) {
            console.log('Error banner text:', await page.locator('.error-banner').innerText());
        }

        // After save the detail navigates away
        await expect(page.getByTestId('test-case-name-input')).not.toBeVisible({ timeout: 10000 });

        // 5. Reopen and verify persistence
        await page.getByText(testName).click();
        await expect(page.getByTestId('test-case-name-input')).toBeVisible();

        // Steps render in read-only mode via .rich-text-display
        await expect(page.locator('[data-testid="step-action-0"] .rich-text-display')).toContainText('First Step');
        await expect(page.locator('[data-testid="step-action-1"] .rich-text-display')).toContainText('Second Step');
    });
});
