import { test, expect } from '../../fixtures/test.js';

// The card talks to the real backend; no vendor call is made (Test connection is not clicked).
test.describe('Settings — TypeSafe.ai card', () => {
    test.beforeEach(async ({ api }) => {
        // Start from a clean singleton so a previous run's key does not leak in.
        await api.put('/settings/typesafe', { enabled: false, clear_api_key: true, model: 'jev-1.13.0' });
    });

    test('saves a key, masks it, preserves it on a blank save and clears it', async ({ settingsPage }) => {
        await test.step('Open the card', async () => {
            await settingsPage.open();
            await settingsPage.openTypeSafeCard();
            await expect(settingsPage.typesafeKeyStatus).toHaveText('No API key stored');
        });

        await test.step('Save a key and see it masked', async () => {
            await settingsPage.typesafeKeyInput.fill('ts-e2e-key-7788');
            await settingsPage.typesafeSaveButton.click();
            await expect(settingsPage.typesafeKeyStatus).toHaveText('Key stored (…7788)');
            await expect(settingsPage.typesafeKeyInput).toHaveValue('');
        });

        await test.step('A blank key with another change preserves the stored key', async () => {
            await settingsPage.typesafeModelInput.fill('jev-latest');
            await settingsPage.typesafeSaveButton.click();
            await expect(settingsPage.typesafeKeyStatus).toHaveText('Key stored (…7788)');
            await settingsPage.page.reload();
            await settingsPage.openTypeSafeCard();
            await expect(settingsPage.typesafeModelInput).toHaveValue('jev-latest');
            await expect(settingsPage.typesafeKeyStatus).toHaveText('Key stored (…7788)');
        });

        await test.step('Clearing removes the key', async () => {
            await settingsPage.typesafeClearKeyCheckbox.check();
            await settingsPage.typesafeSaveButton.click();
            await expect(settingsPage.typesafeKeyStatus).toHaveText('No API key stored');
        });
    });
});
