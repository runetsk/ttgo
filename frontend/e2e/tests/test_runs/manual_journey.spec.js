import { test, expect } from '@playwright/test';

test.describe('Manual run journey', () => {
    test('creating a run lands on its detail page', async ({ page }) => {
        const runName = 'Journey Run ' + Date.now();

        await test.step('Create an empty run via the UI modal', async () => {
            await page.goto('/runs');
            await page.getByTestId('create-test-run-button').click();
            await page.getByTestId('create-run-name-input').fill(runName);
            await page.getByTestId('create-run-submit').click();
        });

        await test.step('The app navigates into the new run', async () => {
            await expect(page).toHaveURL(/\/runs\/run\/[a-f0-9-]+$/);
            await expect(page.getByTestId('run-title')).toHaveText(runName);
        });
    });
});
