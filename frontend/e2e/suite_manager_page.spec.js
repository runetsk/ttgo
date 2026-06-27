import { test, expect } from '@playwright/test';

test.describe('Suite Manager Page', () => {
    test.setTimeout(30000);

    test('should navigate to suites page and manage suites', async ({ page }) => {
        const suiteName = `Suite Page Test ${Date.now()}`;

        await page.goto('/');

        // 1. Verify Suite Manager is NOT on Dashboard
        await expect(page.getByText('Available Suites')).not.toBeVisible();

        // 2. Navigate to Suites Page
        await page.getByRole('button', { name: 'Suites' }).click();
        await expect(page.url()).toContain('/suites');
        await expect(page.getByRole('heading', { name: 'Suite Management' }).first()).toBeVisible();

        // 3. Create a Suite via modal
        await page.getByTestId('open-create-suite-modal').click();
        await page.getByTestId('suite-name-input').fill(suiteName);
        await page.getByTestId('create-suite-button').click();

        // 4. Verify Suite Creation
        await expect(page.getByText(suiteName)).toBeVisible();

        // 5. Navigate back to Home
        await page.getByRole('button', { name: 'Tests' }).click();
        await expect(page.url()).not.toContain('/suites');
    });
});
