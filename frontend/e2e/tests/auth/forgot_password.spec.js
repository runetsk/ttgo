import { test, expect } from '../../fixtures/test.js';

// Logged-out surface: the login screen's password-recovery guidance.
test.use({ storageState: { cookies: [], origins: [] } });

test.describe('Login — forgot password guidance', () => {
    test('reveals recovery help with the break-glass command', async ({ page }) => {
        await page.goto('/login');
        await expect(page.getByRole('heading', { name: 'Sign In' })).toBeVisible();

        const toggle = page.getByTestId('forgot-password-toggle');
        await expect(toggle).toBeVisible();
        // Collapsed until asked for.
        await expect(page.getByTestId('forgot-password-help')).toHaveCount(0);

        await toggle.click();
        const help = page.getByTestId('forgot-password-help');
        await expect(help).toBeVisible();
        await expect(help).toContainText('Settings → Users → Reset Password');
        await expect(help).toContainText('ttgo reset-password you@example.com');

        // The example command embeds the typed email.
        await page.getByPlaceholder('admin@example.com').fill('boss@corp.com');
        await expect(help).toContainText('ttgo reset-password boss@corp.com');

        // Toggles closed again.
        await toggle.click();
        await expect(page.getByTestId('forgot-password-help')).toHaveCount(0);
    });

    test('is not offered during first-run setup', async ({ page }) => {
        // Against a seeded instance needs-setup is false, so we mock the probe to
        // exercise the setup branch of the login screen.
        await page.route('**/api/auth/needs-setup', route =>
            route.fulfill({ status: 200, contentType: 'application/json', body: '{"needs_setup":true}' }));

        await page.goto('/login');
        await expect(page.getByRole('heading', { name: 'Create admin account' })).toBeVisible();
        await expect(page.getByTestId('forgot-password-toggle')).toHaveCount(0);
    });
});
